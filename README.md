HFast
-------------
HTTPS-Server serving PHP (FPM) with convention over config.

> Use of this source code is governed by a BSD-style license that can be found in the LICENSE file.

Why HFast?
HFast eliminates the tedious setup of PHP-FPM, Nginx, rate limiting, and TLS certificates. Just point it at your webroot and go.

**Zero configuration defaults:**
- Automatic TLS via LetsEncrypt
- HTTP/2 and HTTP/3 (QUIC) with IPv4 and IPv6
- Security headers and caching out of the box
- Rate limiting on PHP endpoints (30 req/min per IP)

**Built-in features:**
- Password-protected `/admin/` area
- Pre-compressed asset serving (Brotli/Gzip)
- JSON access logs for easy parsing
- Message queues (`/queue/`) for reliable background processing
- Graceful shutdown (6 second timeout)

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
├── action/         # PHP endpoints at /action/
└── override.toml   # Site-specific configuration
```

**pub/** - Static files (HTML, CSS, JS, images) served directly at the root URL. Place your website's public assets here. Pre-compressed `.br`/`.gz` variants are served automatically (see Caching section).

**admin/** - Protected admin area accessible at `/admin/`. Requires basic auth credentials configured via `Admin` in `override.toml`. Must contain an `index.php` as the entry point.

**action/** - PHP backend endpoints accessible at `/action/`. All requests route through `index.php`. Subject to rate limiting (30 req/min per IP by default) and strict timeouts:
- Read timeout: 5 seconds
- Write timeout: 10 seconds

**override.toml** - Optional per-site configuration file. See configuration reference below.

Certificates are stored in `/var/lib/hfast/certs` (auto-created with 0700 permissions).

**Note:** Content Security Policy (CSP) is enforced by default, requiring CSS and JS to be in external files rather than inline in HTML. Use `SiteType = "weak"` to disable if needed.

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

Caching
-------------
HFast implements RFC 7232/7233 compliant caching with sensible defaults.

**Cache-Control Headers**

| File Type | Header | Duration |
|-----------|--------|----------|
| `.html` and `/` (root) | `no-cache,no-store,must-revalidate` | Always revalidate |
| `.css`, `.js`, `.png`, `.gif`, `.jpg` | `public,max-age=2678400` | 31 days |

HTML files always revalidate to ensure visitors get the latest content, while static assets are cached long-term for performance.

**Conditional Requests (304 Not Modified)**

HFast automatically handles conditional request headers:
- `If-None-Match` / `If-Match` - ETag comparison (strong and weak ETags)
- `If-Modified-Since` / `If-Unmodified-Since` - Last-Modified comparison
- `If-Range` - Partial content requests

When content hasn't changed, HFast returns `304 Not Modified` instead of the full response, saving bandwidth.

**URL Versioning (Cache Busting)**

Static assets support version markers in the URL pattern `asset.vXXXXXX.ext`:
```
style.v123456.css  →  server reads style.css
app.v789.js        →  server reads app.js
```
Change the version number in your HTML to bust browser caches without renaming the physical file.

**Pre-compressed Content**

Place pre-compressed versions alongside your files:
```
style.css      # Original
style.css.br   # Brotli compressed
style.css.gz   # Gzip compressed
```
HFast automatically serves the compressed version based on the client's `Accept-Encoding` header, adding the appropriate `Content-Encoding` header. Supported for `.html`, `.js`, and `.css` files.

**Range Requests**

Full RFC 7233 support for partial content:
- `Accept-Ranges: bytes` header on all static files
- Single and multi-part byte-range requests
- Useful for resumable downloads and video seeking

Firewall
-------------
Open the following ports:
| Port | Protocol | Purpose |
|------|----------|---------|
| 80 | TCP | HTTP (ACME challenges, redirects) |
| 443 | TCP | HTTPS / HTTP/2 |
| 443 | UDP | HTTP/3 (QUIC) |

For optimal HTTP/3 performance, increase UDP buffer sizes:
```
sysctl -w net.core.rmem_max=7500000
sysctl -w net.core.wmem_max=7500000
```

Systemd?
Used by default, see contrib dir for an example config to use.
```
vi /etc/systemd/system/hfast.service
vi /etc/systemd/system/hfast.socket
systemctl daemon-reload
systemctl enable --now hfast.socket
```

override.toml
Place this file in your site root (e.g., `/var/www/example.com/override.toml`) to customize behavior per site.
FYI. This file is only read on `systemctl restart hfast`

| Setting | Type | Description |
|---------|------|-------------|
| `Proxy` | string | Reverse proxy all requests to given URL (e.g., `http://127.0.0.1:3000`). When set, PHP/static handling is bypassed. |
| `ExcludedDomains` | array | Domains to add to Content-Security-Policy header, allowing external CSS/JS (e.g., `["cdn.example.com", "fonts.googleapis.com"]`). |
| `Lang` | array | Supported languages for auto-redirect. Visitors are redirected to `pub/[lang]/` based on Accept-Language header (e.g., `["en", "nl"]`). |
| `Admin` | table | Username/password pairs for `/admin/` basic auth (e.g., `Admin = { "user" = "pass" }`). |
| `DevMode` | bool | Protect entire site with `Authlist` IP whitelist or `Admin` credentials. Useful for staging sites. |
| `Authlist` | table | IP access control. `IP = true` to whitelist, `IP = false` to blacklist. Only applies to `/admin/` or when `DevMode = true`. |
| `SiteType` | string | Site behavior mode: `""` (default, all security rules), `"weak"` (disable CSP), `"indexphp"` (route all requests through index.php). |
| `Pprof` | bool | Enable Go pprof debugging at `/debug/pprof/`. Requires Admin authentication. |
| `Ratelimit` | bool | Enable/disable PHP ratelimiting (default: `true`, 30 req/min per IP). Set to `false` to disable. |
| `SecretKey` | string | HMAC-SHA256 secret for `/queue/` endpoint signing. Queue feature is disabled when not set. |

Example:
```toml
# Proxy = "http://127.0.0.1:3000"
ExcludedDomains = ["cdn.jsdelivr.net", "fonts.googleapis.com"]
Lang = []
Admin = { "admin" = "secretpass" }
DevMode = true
Authlist = { "192.168.1.1" = true, "10.0.0.50" = false }
SiteType = ""
Pprof = false
Ratelimit = false
SecretKey = ""
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
See [contrib/logparser](contrib/logparser) for a tool to parse these logs.

