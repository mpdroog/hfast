package handlers

import (
	"github.com/mpdroog/hfast/config"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNormalizeHost(t *testing.T) {
	tests := []struct {
		input    string
		host     string
		iswww    bool
	}{
		{"example.com", "example.com", false},
		{"www.example.com", "example.com", true},
		{"WWW.EXAMPLE.COM", "example.com", true},
		{"Example.Com", "example.com", false},
		{"www.sub.example.com", "sub.example.com", true},
		{"sub.example.com", "sub.example.com", false},
		// Port stripping tests
		{"example.com:443", "example.com", false},
		{"example.com:8080", "example.com", false},
		{"www.example.com:443", "example.com", true},
		{"WWW.EXAMPLE.COM:443", "example.com", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			host, iswww := normalizeHost(tt.input)
			if host != tt.host {
				t.Errorf("normalizeHost(%q) host = %q, want %q", tt.input, host, tt.host)
			}
			if iswww != tt.iswww {
				t.Errorf("normalizeHost(%q) iswww = %v, want %v", tt.input, iswww, tt.iswww)
			}
		})
	}
}

func TestStripPort(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"example.com", "example.com"},
		{"example.com:80", "example.com:443"},
		{"example.com:8080", "example.com:443"},
		{"192.168.1.1:80", "192.168.1.1:443"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := stripPort(tt.input)
			if result != tt.expected {
				t.Errorf("stripPort(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestRedirectHandler_HTTPtoHTTPS(t *testing.T) {
	// Setup: register a domain in config.Muxs
	config.Muxs["example.com"] = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})
	defer delete(config.Muxs, "example.com")

	handler := &RedirectHandler{}

	req := httptest.NewRequest("GET", "http://example.com/page?query=1", nil)
	req.Host = "example.com"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Errorf("Expected 302 Found, got %d", rec.Code)
	}

	location := rec.Header().Get("Location")
	// stripPort only adds :443 when input has a port
	if location != "https://example.com/page?query=1" {
		t.Errorf("Redirect location: expected https://example.com/page?query=1, got %s", location)
	}
}

func TestRedirectHandler_WWWRedirect(t *testing.T) {
	// Setup: register domain
	config.Muxs["example.com"] = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})
	defer delete(config.Muxs, "example.com")

	handler := &RedirectHandler{}

	req := httptest.NewRequest("GET", "http://www.example.com/page", nil)
	req.Host = "www.example.com"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// www.* should get 301 (permanent) redirect
	if rec.Code != http.StatusMovedPermanently {
		t.Errorf("www redirect: expected 301, got %d", rec.Code)
	}

	location := rec.Header().Get("Location")
	if location != "https://example.com/page" {
		t.Errorf("Redirect location: expected https://example.com/page, got %s", location)
	}
}

func TestRedirectHandler_NonGETMethod(t *testing.T) {
	handler := &RedirectHandler{}

	methods := []string{"POST", "PUT", "DELETE", "PATCH"}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "http://example.com/", nil)
			req.Host = "example.com"
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("%s: expected 400, got %d", method, rec.Code)
			}
		})
	}
}

func TestRedirectHandler_HEAD(t *testing.T) {
	config.Muxs["example.com"] = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})
	defer delete(config.Muxs, "example.com")

	handler := &RedirectHandler{}

	req := httptest.NewRequest("HEAD", "http://example.com/", nil)
	req.Host = "example.com"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// HEAD should be allowed like GET
	if rec.Code != http.StatusFound {
		t.Errorf("HEAD: expected 302, got %d", rec.Code)
	}
}

func TestRedirectHandler_UnknownDomain(t *testing.T) {
	// No domains configured, no default
	handler := &RedirectHandler{}

	req := httptest.NewRequest("GET", "http://unknown.com/", nil)
	req.Host = "unknown.com"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Unknown domain: expected 400, got %d", rec.Code)
	}
}

func TestRedirectHandler_DefaultFallback(t *testing.T) {
	// Setup default handler
	defaultCalled := false
	config.Muxs["default"] = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defaultCalled = true
		w.WriteHeader(200)
		w.Write([]byte("Default"))
	})
	defer delete(config.Muxs, "default")

	handler := &RedirectHandler{}

	req := httptest.NewRequest("GET", "http://unknown.com/", nil)
	req.Host = "unknown.com"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !defaultCalled {
		t.Error("Default handler should be called for unknown domain")
	}
	if rec.Code != 200 {
		t.Errorf("Default fallback: expected 200, got %d", rec.Code)
	}
}
