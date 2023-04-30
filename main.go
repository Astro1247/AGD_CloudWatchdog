package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

const (
	cloudflareAPIToken = "CLOUDFLARE_TOKEN"
	cloudflareZoneID   = "CLOUDFLARE_ZONEID"
	cloudflareEmail    = "CLOUDFLARE_EMAIL"
	grafanaBearerToken = "GRAFANA_BEARER_TOKEN"
	telegramBotToken   = "TELEGRAM_BOT_TOKEN"
	telegramChatID     = 0
)

var (
	currentSecurityLevel string
	isUnderAttack        bool
	timer                *time.Timer
)

func sendTelegramMessage(message string) {
	payload := fmt.Sprintf(`{"chat_id":%v,"text":"[AGD CloudWatchdog] %v","parse_mode":"HTML"}`, telegramChatID, message)
	resp, err := http.Post(fmt.Sprintf("https://api.telegram.org/bot%v/sendMessage", telegramBotToken), "application/json", strings.NewReader(payload))
	if err != nil {
		fmt.Println("Error sending Telegram message:", err)
		return
	}
	defer resp.Body.Close()
}

func getCurrentSecurityLevel() {
	req, err := http.NewRequest("GET", fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%v/settings/security_level", cloudflareZoneID), nil)
	if err != nil {
		fmt.Println("Error creating request:", err)
		return
	}
	req.Header.Set("X-Auth-Email", cloudflareEmail)
	req.Header.Set("Authorization", "Bearer "+cloudflareAPIToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		sendTelegramMessage("Error getting Cloudflare security level")
		fmt.Println("Error getting Cloudflare security level:", err)
		return
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading response body:", err)
		return
	}
	var data map[string]interface{}
	err = json.Unmarshal(body, &data)
	if err != nil {
		fmt.Println("Error unmarshalling JSON:", err)
		return
	}
	currentSecurityLevel = data["result"].(map[string]interface{})["value"].(string)
	fmt.Println("Cloudflare current security level:", currentSecurityLevel)
}

func handleAlert(data map[string]interface{}) {
	fmt.Println("Received alert notification")
	alerts := data["alerts"].([]interface{})
	getCurrentSecurityLevel()
	neededAlertFound := false
	attackStatus := ""
	for _, alert := range alerts {
		alertMap := alert.(map[string]interface{})
		status := alertMap["status"].(string)
		labels, ok := alertMap["labels"].(map[string]interface{})
		if !ok {
			fmt.Println("Alert labels not found. Skipping alert...")
			continue
		}
		alert_name, ok := labels["alertname"].(string)
		if !ok {
			fmt.Println("Alert name not found. Skipping alert...")
			continue
		}
		// Check if alertname is one of the alerts we want to handle
		if alert_name != "5xx error rate" && alert_name != "2xx response rate" && alert_name != "Requests total LOW [Server]" && alert_name != "Requests total HIGH [Server]" && alert_name != "Requests total LOW [Upstream]" && alert_name != "Requests total HIGH [Upstream]" {
			fmt.Println("Alert name not found in list of alerts to handle. Skipping alert...")
			continue
		} else {
			neededAlertFound = true
		}
		if status == "firing" {
			attackStatus = "ongoing"
			break
		} else if status == "resolved" {
			attackStatus = "resolved"
		}
	}
	if neededAlertFound {
		switch attackStatus {
		case "ongoing":
			if currentSecurityLevel == "under_attack" && isUnderAttack {
				fmt.Println("Already under attack. Skipping alert...")
				return
			}
			sendTelegramMessage("Attack detected, switching Cloudflare security level to under attack mode!")
			modifyCloudflareSecurityLevel("under_attack")
			fmt.Println("Under attack mode activated!")
			sendTelegramMessage("Under attack mode activated!")
			isUnderAttack = true
		case "resolved":
			if currentSecurityLevel == "high" && !isUnderAttack {
				fmt.Println("Already in high security mode. Skipping alert...")
				return
			}
			if timer != nil {
				timer.Stop()
				timer = nil
			}
			fmt.Println("Attack ended! Switching back to high security if there is no new attack in 10 minutes...")
			sendTelegramMessage("Attack ended! Switching back to high security if there is no new attack in 10 minutes...")
			isUnderAttack = false
			timer = time.AfterFunc(10*time.Minute, func() {
				if isUnderAttack {
					fmt.Println("Attack is still active! Preventing Cloudflare security level from being switched back to high")
					sendTelegramMessage("Attack is still active! Preventing Cloudflare security level from being switched back to high")
				} else {
					sendTelegramMessage("No new attacks detected in the last 10 minutes. Switching Cloudflare security level back to high.")
					modifyCloudflareSecurityLevel("high")
					fmt.Println("Security level switched back to high!")
				}
			})
		}
	}
}

func modifyCloudflareSecurityLevel(level string) {
	payload := fmt.Sprintf(`{"value":"%v"}`, level)
	req, err := http.NewRequest("PATCH", fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%v/settings/security_level", cloudflareZoneID), strings.NewReader(payload))
	if err != nil {
		fmt.Println("Error creating request:", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Auth-Email", cloudflareEmail)
	req.Header.Set("Authorization", "Bearer "+cloudflareAPIToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		sendTelegramMessage("Error updating Cloudflare security level")
		fmt.Println("Error updating Cloudflare security level:", err)
		return
	}
	defer resp.Body.Close()
	_, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading response body:", err)
		return
	}
	fmt.Println("Cloudflare security level updated to:", level)
}

func main() {
	getCurrentSecurityLevel()
	http.HandleFunc("/grafana/alerts", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			bearerToken := r.Header.Get("Authorization")
			if bearerToken != fmt.Sprintf("Bearer %s", grafanaBearerToken) {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			body, err := ioutil.ReadAll(r.Body)
			if err != nil {
				fmt.Println("Error reading request body:", err)
				return
			}
			var data map[string]interface{}
			err = json.Unmarshal(body, &data)
			if err != nil {
				fmt.Println("Error unmarshalling JSON:", err)
				return
			}
			handleAlert(data)
		}
	})
	err := http.ListenAndServe(":9590", nil)
	if err != nil {
		return
	}
	fmt.Println("CloudWatchdog is running!")
}
