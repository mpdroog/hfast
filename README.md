HFast
-------------
HTTPS-Server with convention over config, favoring security over convenience.

Why?
- Few to no configuration
 So bye bye to those hours wasting on setting
 up PHP-FCGI, ratelimiting etc...
- HTTP/2 by default with HTTP/2 Push
- TLS by default, no config! (LetsEncrypt)
- Ratelimit by default on PHP
- Proper caching/security headers everywhere

How?
- You need to place files in the pre-defined project structure
- Think about URL-versioning CSS/img/JS-files if you want to replace them
- Content Security Policy (CSP) by default, no CSS/JS in the body of HTML
- Proper deadlines, so 5sec to finish a script, if your script it slower fix it!

Systemd?
Supported, see contrib dir for an example config to use.
```
vi /etc/systemd/system/hfast.service
chmod 644 /etc/systemd/system/hfast.service
systemctl daemon-reload
systemctl enable hfast
systemctl start hfast
```

Future plan(s)
- Write small co-worker to offer distributed (DNS)
 hosting where the site is kept online when nodes fall off.

Thanks to:
* https://github.com/coreos/go-systemd/tree/master/examples/activation/httpserver

