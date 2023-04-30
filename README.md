# AGD CloudWatchdog

## About

Program is designed to do very simple task - automaticaly switch CloudFlare security level to "Under attack" when your website is under attack and switch it back to "High", when attack is gone and timeout is over after the attack.

## Details

The program works as a service that launches an independent binary. Also you can create a service on your server to support automatic launch/restart of the program for any occasion, including restarting the host server.
In this case to view program logs - you can run the following command on host under any user:

`journalctl -f -u SERVICE_NAME`

Where "SERVICE_NAME" is the name you typed in service file on your host.

The program listens for incoming connections on port 9590 along the /grafana/alerts path using the POST method
When a POST request is received at the URL /grafana/alerts on port 9590, the program checks for the presence of an authorization token, if not, 403 response code will be generated.
The program was configured to react to any NGINX-related alerts in grafana, after triggering at least one of them - it sets the CloudFlare protection mode to the "Under Attack!".
Alert must have one of the following alert names to react on it:
 - 5xx error rate
 - 2xx response rate
 - Requests total LOW [Server]
 - Requests total HIGH [Server]
 - Requests total LOW [Upstream]
 - Requests total HIGH [Upstream]

After receiving a notification where none of these 6 alerts is "firing" - a 10-minute timer is started to prevent attacks from restarting. As long as the timer is active, the security mode at CloudFlare remains at the "Under Attack!" level. If after the timer expires there were no repeated alerts, the CloudFlare security mode will be changed to "High". If a new attack repeats within this 10-minute window, the protection level will not be lowered, and when the timer is triggered, the protection level will be prevented with a corresponding notification in Telegram.

All changes and events of the program are also accompanied by alerts in Telegrams.

The program is designed as a complete and independent binary. Doesn't have any dependencies, requirements, etc. It has the lowest possible load on the host, as it can be compiled as a binary file. It immediately has all the settings necessary for work inside itself, therefore for its operation it is enough just to start it. It can be easily moved to any other host, if necessary, with a simple action - transferring the main program file by simply copying it, then launching it on a new host and changing the link in the grafana itself, where to knock with an alert.

## WARNING

Due to the fact that the program has all the settings within itself, despite the fact that it is assembled into a single integral binary, it is likely that in skillful hands they will be able to get the Bearer request verification token from Grafana, the CloudFlare token that manages protection levels site, as well as the telegram bot token that alerts occur. In this regard, the binary should not fall into the wrong hands.
