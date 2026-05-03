package handlers

import (
	"github.com/mpdroog/hfast/config"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestVhost_RoutesToCorrectMux(t *testing.T) {
	// Setup
	muxCalled := false
	config.Muxs["example.com"] = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		muxCalled = true
		w.WriteHeader(200)
		w.Write([]byte("example.com"))
	})
	config.Overrides["example.com"] = config.Override{
		SecretKey: "test-secret",
	}
	defer func() {
		delete(config.Muxs, "example.com")
		delete(config.Overrides, "example.com")
	}()

	handler := Vhost()

	req := httptest.NewRequest("GET", "https://example.com/page", nil)
	req.Host = "example.com"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !muxCalled {
		t.Error("Mux for example.com should be called")
	}
	if rec.Code != 200 {
		t.Errorf("Expected 200, got %d", rec.Code)
	}
}

func TestVhost_WWWRedirect(t *testing.T) {
	// Setup domain (without www)
	config.Muxs["example.com"] = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})
	config.Overrides["example.com"] = config.Override{}
	defer func() {
		delete(config.Muxs, "example.com")
		delete(config.Overrides, "example.com")
	}()

	handler := Vhost()

	req := httptest.NewRequest("GET", "https://www.example.com/page?q=1", nil)
	req.Host = "www.example.com"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMovedPermanently {
		t.Errorf("www redirect: expected 301, got %d", rec.Code)
	}

	location := rec.Header().Get("Location")
	expected := "https://example.com/page?q=1"
	if location != expected {
		t.Errorf("Location: expected %s, got %s", expected, location)
	}
}

func TestVhost_UnknownHost(t *testing.T) {
	handler := Vhost()

	req := httptest.NewRequest("GET", "https://unknown.com/", nil)
	req.Host = "unknown.com"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Body.String() != "ERR: No such site." {
		t.Errorf("Unknown host: expected error message, got %q", rec.Body.String())
	}
}

func TestVhost_SetsXDomainHeader(t *testing.T) {
	var capturedDomain string
	config.Muxs["example.com"] = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedDomain = r.Header.Get("X-Domain")
		w.WriteHeader(200)
	})
	config.Overrides["example.com"] = config.Override{
		SecretKey: "my-secret",
	}
	defer func() {
		delete(config.Muxs, "example.com")
		delete(config.Overrides, "example.com")
	}()

	handler := Vhost()

	req := httptest.NewRequest("GET", "https://example.com/", nil)
	req.Host = "example.com"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if capturedDomain != "example.com" {
		t.Errorf("X-Domain: expected example.com, got %s", capturedDomain)
	}
}

func TestVhost_SetsXSecretkeyHeader(t *testing.T) {
	var capturedSecret string
	config.Muxs["example.com"] = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedSecret = r.Header.Get("X-Secretkey")
		w.WriteHeader(200)
	})
	config.Overrides["example.com"] = config.Override{
		SecretKey: "super-secret-key",
	}
	defer func() {
		delete(config.Muxs, "example.com")
		delete(config.Overrides, "example.com")
	}()

	handler := Vhost()

	req := httptest.NewRequest("GET", "https://example.com/", nil)
	req.Host = "example.com"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if capturedSecret != "super-secret-key" {
		t.Errorf("X-Secretkey: expected super-secret-key, got %s", capturedSecret)
	}
}

func TestVhost_StripsXPoweredBy(t *testing.T) {
	config.Muxs["example.com"] = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Powered-By", "PHP/8.0")
		w.WriteHeader(200)
	})
	config.Overrides["example.com"] = config.Override{}
	defer func() {
		delete(config.Muxs, "example.com")
		delete(config.Overrides, "example.com")
	}()

	handler := Vhost()

	req := httptest.NewRequest("GET", "https://example.com/", nil)
	req.Host = "example.com"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Header().Get("X-Powered-By") != "" {
		t.Error("X-Powered-By should be stripped")
	}
}

func TestVhost_SetsServerHeader(t *testing.T) {
	config.Muxs["example.com"] = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})
	config.Overrides["example.com"] = config.Override{}
	defer func() {
		delete(config.Muxs, "example.com")
		delete(config.Overrides, "example.com")
	}()

	handler := Vhost()

	req := httptest.NewRequest("GET", "https://example.com/", nil)
	req.Host = "example.com"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Header().Get("Server") != "HFast" {
		t.Errorf("Server header: expected HFast, got %s", rec.Header().Get("Server"))
	}
}

func TestVhost_HostWithPort(t *testing.T) {
	// Regression test: Host header may include port (e.g., "example.com:443")
	// The handler should strip the port and match config keyed by "example.com"
	muxCalled := false
	config.Muxs["example.com"] = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		muxCalled = true
		w.WriteHeader(200)
	})
	config.Overrides["example.com"] = config.Override{
		SecretKey: "test-secret",
	}
	defer func() {
		delete(config.Muxs, "example.com")
		delete(config.Overrides, "example.com")
	}()

	handler := Vhost()

	// Request with port should match config without port
	req := httptest.NewRequest("GET", "https://example.com:443/page", nil)
	req.Host = "example.com:443"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !muxCalled {
		t.Error("Host example.com:443 should match config entry example.com")
	}
	if rec.Code != 200 {
		t.Errorf("Expected 200, got %d", rec.Code)
	}
}

func TestVhost_HostWithPortNormalized(t *testing.T) {
	// Regression test: ensure normalized host (lowercase, no port) is used for lookup
	muxCalled := false
	config.Muxs["example.com"] = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		muxCalled = true
		w.WriteHeader(200)
	})
	config.Overrides["example.com"] = config.Override{}
	defer func() {
		delete(config.Muxs, "example.com")
		delete(config.Overrides, "example.com")
	}()

	handler := Vhost()

	// Mixed case host with port should match lowercase config without port
	req := httptest.NewRequest("GET", "https://EXAMPLE.COM:443/", nil)
	req.Host = "EXAMPLE.COM:443"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !muxCalled {
		t.Error("Host EXAMPLE.COM:443 should match config entry example.com")
	}
}

func TestVhost_MultipleDomains(t *testing.T) {
	// Setup multiple domains
	config.Muxs["site1.com"] = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("site1"))
	})
	config.Muxs["site2.com"] = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("site2"))
	})
	config.Overrides["site1.com"] = config.Override{}
	config.Overrides["site2.com"] = config.Override{}
	defer func() {
		delete(config.Muxs, "site1.com")
		delete(config.Muxs, "site2.com")
		delete(config.Overrides, "site1.com")
		delete(config.Overrides, "site2.com")
	}()

	handler := Vhost()

	// Test site1
	req1 := httptest.NewRequest("GET", "https://site1.com/", nil)
	req1.Host = "site1.com"
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if rec1.Body.String() != "site1" {
		t.Errorf("site1.com: expected 'site1', got %q", rec1.Body.String())
	}

	// Test site2
	req2 := httptest.NewRequest("GET", "https://site2.com/", nil)
	req2.Host = "site2.com"
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Body.String() != "site2" {
		t.Errorf("site2.com: expected 'site2', got %q", rec2.Body.String())
	}
}
