DeltaJournal
=====================
E-mail systemd journal entries from the last emailed point.

Why?
- Get notified of any issues with your app every X-minutes
- Never miss an issue ever again

build-docker.sh?
We use Docker to spawn a Linux-environment, get the Systemd C-headers and
compile a binary with CGO.

Least privilege
```
useradd -r dj
mkdir -p /home/dj
usermod -a -G systemd-journal dj

vi /etc/systemd/system/deltajournal.service
chmod 644 /etc/systemd/system/deltajournal.service
systemctl daemon-reload
systemctl enable deltajournal
systemctl start deltajournal
```
