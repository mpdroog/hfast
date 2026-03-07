HFast
-------------
HTTPS-Server serving PHP (FPM) with convention over config.

> Use of this source code is governed by a BSD-style license that can be found in the LICENSE file.

Why?
- Few to no configuration
  So bye bye to those hours wasting on setting
  up PHP-FCGI, ratelimiting etc...
- HTTP/2 and IPv4+IPv6 by default
- TLS by default, zero config! (LetsEncrypt)
- Ratelimit by default on PHP-backend (30 req/min per IP)
- Proper caching/security headers everywhere
- base64 auth protected /admin to put sensitive stuff behind
- Native support for pre-optimized content. Let Brotli/Zopfli pre-compress assets
  to `.br`/`.gz` and HFast will serve them
- Accesslog as JSON for easy parsing
- Dynamic queues (`/queue/<chan>`) to easily queue data to the site without
  worrying about losing data on bugs in your code (faster code building!)
- Graceful shutdown on SIGINT/SIGTERM (6 second timeout)

How?
- You need to place files in the pre-defined project structure
- Think about URL-versioning CSS/img/JS-files if you want to replace them (support file.vXXX.css|js by default where the vXXX is stripped off)
- Content Security Policy (CSP) by default, no CSS/JS in the body of HTML
- Proper deadlines, so 5sec to finish a script, if your script it slower fix it!

Requirements
- PHP-FPM listening on `127.0.0.1:8000`
- systemd (or [sdnotify-wrapper](https://github.com/mpdroog/sdnotify-wrapper) for non-systemd systems)

CLI Flags
```
-v        Verbose mode (log more)
-w        Webroot directory (default: /var/www)
-s        Disable systemd socket activation
-l        Log path (default: /var/log/hfast.access.log)
```

Project Structure
```
/var/www/example.com/
├── pub/            # Public static files served at /
├── admin/          # Admin backend at /admin/ (protected by basic auth)
├── action/         # PHP endpoints at /action/ (index.php)
└── override.toml   # Site-specific configuration
```

Certificates are stored in `/var/lib/hfast/certs` (auto-created with 0700 permissions).

Security Headers
```
Strict-Transport-Security: max-age=315360000; preload
X-Frame-Options: DENY
X-Content-Type-Options: nosniff
X-XSS-Protection: 1; mode=block
Content-Security-Policy: default-src 'self' [ExcludedDomains]
Permissions-Policy: geolocation=(), camera=(), microphone=(), payment=(), usb=(), magnetometer=(), gyroscope=(), accelerometer=()
Referrer-Policy: strict-origin-when-cross-origin
```

Systemd?
Supported, see contrib dir for an example config to use.
```
vi /etc/systemd/system/hfast.service
vi /etc/systemd/system/hfast.socket
systemctl daemon-reload
systemctl enable --now hfast.socket
```

Example override.toml
```
type Override struct {
	Proxy           string            // Reverse proxy to given http(s)://addr:port
	ExcludedDomains []string          // CSP-domains added to header (allowing external CSS/JS)
	Lang            []string          // Auto redirect to supported pub/[lang]
	Admin           map[string]string // Admin user+pass for backend
	DevMode         bool              // Protect whole site with Authlist(IP) or Admin user+pass
	Authlist        map[string]bool   // Whitelist with IP=>true, Blacklist with IP=>false (works only with DevMode or /admin)
	SiteType        string            // "" = default (all rules on), "weak" = Site without CSP, "indexphp" = Site with index.php as central file
	Pprof           bool              // Enable Go pprof debugging at /debug/pprof/ (requires Admin)
	Ratelimit       bool              // Override default ratelimiter on PHP code (default: true)
	SecretKey       string            // Secret key for HMAC-SHA256 queue signing (required to enable /queue/)
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

