package handlers

import (
	"net/http"
	"net/url"
	"strings"
)

// CSRF validates Origin/Referer headers for state-changing requests (POST, PUT, DELETE, PATCH).
// This provides protection against Cross-Site Request Forgery attacks.
// Safe methods (GET, HEAD, OPTIONS) are always allowed.
// State-changing methods require a valid Origin or Referer header matching the request Host.
func CSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Allow safe methods
		if r.Method == "GET" || r.Method == "HEAD" || r.Method == "OPTIONS" {
			next.ServeHTTP(w, r)
			return
		}

		// Validate Origin/Referer for state-changing methods
		if !validateCSRFOrigin(r) {
			w.WriteHeader(403)
			w.Write([]byte("CSRF validation failed\n"))
			return
		}

		next.ServeHTTP(w, r)
	})
}

// validateCSRFOrigin checks if the request Origin or Referer matches the Host.
// Returns true if valid, false if the request should be rejected.
func validateCSRFOrigin(r *http.Request) bool {
	targetHost := csrfStripPort(r.Host)

	// Check Origin header first (preferred, more reliable)
	origin := r.Header.Get("Origin")
	if origin != "" {
		return originMatchesHost(origin, targetHost)
	}

	// Fall back to Referer header
	referer := r.Header.Get("Referer")
	if referer != "" {
		return refererMatchesHost(referer, targetHost)
	}

	// Strict mode: reject if neither header is present
	return false
}

// originMatchesHost checks if the Origin header matches the target host.
// Origin format: "https://example.com" or "http://example.com:8080"
func originMatchesHost(origin, targetHost string) bool {
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	originHost := csrfStripPort(u.Host)
	return strings.EqualFold(originHost, targetHost)
}

// refererMatchesHost checks if the Referer header's host matches the target host.
// Referer format: "https://example.com/path/page"
func refererMatchesHost(referer, targetHost string) bool {
	u, err := url.Parse(referer)
	if err != nil {
		return false
	}
	refererHost := csrfStripPort(u.Host)
	return strings.EqualFold(refererHost, targetHost)
}

// csrfStripPort removes the port from a host string if present.
// "example.com:443" -> "example.com"
// "example.com" -> "example.com"
func csrfStripPort(host string) string {
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		// Check if this is an IPv6 address
		if strings.Contains(host, "]") {
			// IPv6: [::1]:8080 -> extract after ]
			if bracketIdx := strings.LastIndex(host, "]"); bracketIdx != -1 && idx > bracketIdx {
				return host[:idx]
			}
			return host
		}
		return host[:idx]
	}
	return host
}
