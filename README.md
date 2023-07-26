HFast
-------------
HTTPS-Server serving PHP (FPM) with convention over config.
License: MIT

Why?
- Few to no configuration
 So bye bye to those hours wasting on setting
 up PHP-FCGI, ratelimiting etc...
- HTTP/2 and IPv4+IPv6 by default
- TLS by default, zero config! (LetsEncrypt)
- Ratelimit by default on PHP-backend
- Proper caching/security headers everywhere
- base64 auth protected /admin to put sensitive stuff behind
- Native support for pre-optimized content. Let Brotli/Zopfli pre-compress assets
 to `.br`/`.gz` and HFast will serve them
- Accesslog as JSON for easy parsing
- Dynamic queues (`/queue/<chan>`) to easily queue data to the site without
 worrying about losing data on bugs in your code (faster code building!)

How?
- You need to place files in the pre-defined project structure
- Think about URL-versioning CSS/img/JS-files if you want to replace them (support file.vXXX.css|js by default where the vXXX is stripped off)
- Content Security Policy (CSP) by default, no CSS/JS in the body of HTML
- Proper deadlines, so 5sec to finish a script, if your script it slower fix it!

Systemd?
Supported, see contrib dir for an example config to use.
```
vi /etc/systemd/system/hfast.service
vi /etc/systemd/system/hfast.socket
chmod 644 /etc/systemd/system/hfast.service
systemctl daemon-reload
systemctl enable hfast
systemctl enable hfast.socket
systemctl start hfast.socket
systemctl start hfast
```

Example overrides.toml
```
type Overrides struct {
	Proxy           string // Just forward to given http(s)://addr:port
	ExcludedDomains []string // CSP-domains added to header (allowing external CSS/JS)
	Lang            []string // Auto redirect to supported pub/[lang]
	Admin           map[string]string // Admin user+pass for backend
	DevMode         bool // Protect whole site with Authlist(IP) or Admin user+pass
	Authlist        map[string]bool // Whitelist with IP=>true, Blacklist with IP=>false (works only with DevMode or /admin)
	SiteType        string // "" = default (all rules on), "weak" = Site without CSP, "indexphp" = Site with index.php as central file
}
```

/var/log/hfast.access.log
```
type Msg struct {
	Method string
	Host string
	URL string
	Status int
	Remote string
	Ratelimit string
	Duration int64
	UA string
	Proto string
	Len uint64
	Date string
	Time string
	Referer string
}
```
FYI. Example that reads the logs findable in `contrib/logparser`.

Future plan(s)
- Write small co-worker to offer distributed (DNS)
 hosting where the site is kept online when nodes fall off.

Thanks to:
* https://github.com/coreos/go-systemd/tree/master/examples/activation/httpserver

