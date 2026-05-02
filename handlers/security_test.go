package handlers

import (
	"github.com/mpdroog/hfast/config"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSec_DefaultHeaders(t *testing.T) {
	headers := sec(nil, "")

	tests := []struct {
		header   string
		contains string
	}{
		{"Strict-Transport-Security", "max-age=315360000"},
		{"Strict-Transport-Security", "preload"},
		{"X-Frame-Options", "DENY"},
		{"X-Content-Type-Options", "nosniff"},
		{"X-XSS-Protection", "1; mode=block"},
		{"Permissions-Policy", "geolocation=()"},
		{"Referrer-Policy", "strict-origin-when-cross-origin"},
		{"Alt-Svc", "h3="},
		{"Content-Security-Policy", "default-src 'self'"},
	}

	for _, tt := range tests {
		t.Run(tt.header, func(t *testing.T) {
			val, ok := headers[tt.header]
			if !ok {
				t.Errorf("Missing header: %s", tt.header)
				return
			}
			if !strings.Contains(val, tt.contains) {
				t.Errorf("%s: expected to contain %q, got %q", tt.header, tt.contains, val)
			}
		})
	}
}

func TestSec_ExcludedDomains(t *testing.T) {
	domains := []string{"cdn.example.com", "fonts.googleapis.com"}
	headers := sec(domains, "")

	csp := headers["Content-Security-Policy"]
	for _, domain := range domains {
		if !strings.Contains(csp, domain) {
			t.Errorf("CSP should contain %s, got %s", domain, csp)
		}
	}
}

func TestSec_WeakMode_NoCSP(t *testing.T) {
	headers := sec(nil, "weak")

	if _, ok := headers["Content-Security-Policy"]; ok {
		t.Error("weak mode should NOT have CSP header")
	}

	// Other headers should still be present
	if _, ok := headers["Strict-Transport-Security"]; !ok {
		t.Error("HSTS should still be present in weak mode")
	}
}

func TestSecureWrapper_SetsHeaders(t *testing.T) {
	// Setup
	config.Overrides["example.com"] = config.Override{
		ExcludedDomains: []string{"cdn.test.com"},
		SiteType:        "",
	}
	defer delete(config.Overrides, "example.com")

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})

	handler := SecureWrapper(next)

	req := httptest.NewRequest("GET", "https://example.com/", nil)
	req.Host = "example.com"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Check security headers are set
	if rec.Header().Get("Strict-Transport-Security") == "" {
		t.Error("HSTS header missing")
	}
	if rec.Header().Get("X-Frame-Options") != "DENY" {
		t.Error("X-Frame-Options missing or incorrect")
	}
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("X-Content-Type-Options missing")
	}

	// Check CSP includes excluded domain
	csp := rec.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "cdn.test.com") {
		t.Errorf("CSP should contain cdn.test.com: %s", csp)
	}
}

func TestSecureWrapper_WeakSiteType(t *testing.T) {
	config.Overrides["weak.example.com"] = config.Override{
		SiteType: "weak",
	}
	defer delete(config.Overrides, "weak.example.com")

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})

	handler := SecureWrapper(next)

	req := httptest.NewRequest("GET", "https://weak.example.com/", nil)
	req.Host = "weak.example.com"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// CSP should NOT be present in weak mode
	if rec.Header().Get("Content-Security-Policy") != "" {
		t.Error("weak mode should not have CSP")
	}

	// Other security headers should still be present
	if rec.Header().Get("Strict-Transport-Security") == "" {
		t.Error("HSTS should still be present")
	}
}

func TestSecureWrapper_DefaultFallback(t *testing.T) {
	// Setup default override
	config.Overrides["default"] = config.Override{
		SiteType: "",
	}
	defer delete(config.Overrides, "default")

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})

	handler := SecureWrapper(next)

	// Request for unknown host should use default
	req := httptest.NewRequest("GET", "https://unknown.com/", nil)
	req.Host = "unknown.com"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Should still have security headers from default config
	if rec.Header().Get("Strict-Transport-Security") == "" {
		t.Error("Should use default override for unknown host")
	}
}

func TestSecureWrapper_PassesRequestToNext(t *testing.T) {
	config.Overrides["example.com"] = config.Override{}
	defer delete(config.Overrides, "example.com")

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(201)
		w.Write([]byte("Created"))
	})

	handler := SecureWrapper(next)

	req := httptest.NewRequest("POST", "https://example.com/resource", nil)
	req.Host = "example.com"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !nextCalled {
		t.Error("Next handler should be called")
	}
	if rec.Code != 201 {
		t.Errorf("Status code should be preserved: got %d", rec.Code)
	}
}

func TestSecureWrapper_AltSvcHeader(t *testing.T) {
	config.Overrides["example.com"] = config.Override{}
	defer delete(config.Overrides, "example.com")

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})

	handler := SecureWrapper(next)

	req := httptest.NewRequest("GET", "https://example.com/", nil)
	req.Host = "example.com"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	altSvc := rec.Header().Get("Alt-Svc")
	if !strings.Contains(altSvc, "h3=") {
		t.Errorf("Alt-Svc should advertise HTTP/3: got %s", altSvc)
	}
}

func TestSecureWrapper_PermissionsPolicy(t *testing.T) {
	config.Overrides["example.com"] = config.Override{}
	defer delete(config.Overrides, "example.com")

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})

	handler := SecureWrapper(next)

	req := httptest.NewRequest("GET", "https://example.com/", nil)
	req.Host = "example.com"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	pp := rec.Header().Get("Permissions-Policy")
	features := []string{"geolocation=()", "camera=()", "microphone=()", "payment=()"}
	for _, f := range features {
		if !strings.Contains(pp, f) {
			t.Errorf("Permissions-Policy should contain %s: got %s", f, pp)
		}
	}
}
