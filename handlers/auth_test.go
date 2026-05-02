package handlers

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBasicAuth_WhitelistedIP(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("OK"))
	})

	authlist := map[string]bool{
		"192.168.1.100": true, // whitelisted
	}
	handler := BasicAuth(next, "Test", nil, authlist)

	req := httptest.NewRequest("GET", "http://example.com/", nil)
	req.RemoteAddr = "192.168.1.100:12345"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Errorf("Whitelisted IP: expected 200, got %d", rec.Code)
	}
}

func TestBasicAuth_BlacklistedIP(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})

	authlist := map[string]bool{
		"10.0.0.50": false, // blacklisted
	}
	handler := BasicAuth(next, "Test", nil, authlist)

	req := httptest.NewRequest("GET", "http://example.com/", nil)
	req.RemoteAddr = "10.0.0.50:12345"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != 403 {
		t.Errorf("Blacklisted IP: expected 403, got %d", rec.Code)
	}
	if rec.Body.String() != "Blacklisted IP.\n" {
		t.Errorf("Unexpected body: %q", rec.Body.String())
	}
}

func TestBasicAuth_NoCredentials_NoUserpass(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})

	// No userpass configured, no authlist match
	handler := BasicAuth(next, "TestRealm", map[string]string{}, nil)

	req := httptest.NewRequest("GET", "http://example.com/", nil)
	req.RemoteAddr = "1.2.3.4:12345"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != 401 {
		t.Errorf("No credentials: expected 401, got %d", rec.Code)
	}
	if rec.Header().Get("WWW-Authenticate") != `Basic realm="TestRealm"` {
		t.Errorf("Missing WWW-Authenticate header")
	}
}

func TestBasicAuth_ValidCredentials(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("Authenticated"))
	})

	userpass := map[string]string{
		"admin": "secretpass",
	}
	handler := BasicAuth(next, "Admin", userpass, nil)

	req := httptest.NewRequest("GET", "http://example.com/admin/", nil)
	req.RemoteAddr = "1.2.3.4:12345"
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("admin:secretpass")))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Errorf("Valid credentials: expected 200, got %d", rec.Code)
	}
}

func TestBasicAuth_InvalidPassword(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})

	userpass := map[string]string{
		"admin": "secretpass",
	}
	handler := BasicAuth(next, "Admin", userpass, nil)

	req := httptest.NewRequest("GET", "http://example.com/admin/", nil)
	req.RemoteAddr = "1.2.3.4:12345"
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("admin:wrongpass")))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != 401 {
		t.Errorf("Invalid password: expected 401, got %d", rec.Code)
	}
}

func TestBasicAuth_InvalidUser(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})

	userpass := map[string]string{
		"admin": "secretpass",
	}
	handler := BasicAuth(next, "Admin", userpass, nil)

	req := httptest.NewRequest("GET", "http://example.com/admin/", nil)
	req.RemoteAddr = "1.2.3.4:12345"
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("hacker:secretpass")))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != 401 {
		t.Errorf("Invalid user: expected 401, got %d", rec.Code)
	}
}

func TestBasicAuth_NoAuthHeader(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})

	userpass := map[string]string{
		"admin": "secretpass",
	}
	handler := BasicAuth(next, "Admin", userpass, nil)

	req := httptest.NewRequest("GET", "http://example.com/admin/", nil)
	req.RemoteAddr = "1.2.3.4:12345"
	// No Authorization header
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != 401 {
		t.Errorf("No auth header: expected 401, got %d", rec.Code)
	}
}

func TestBasicAuth_MultipleUsers(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})

	userpass := map[string]string{
		"admin": "adminpass",
		"user1": "user1pass",
		"user2": "user2pass",
	}
	handler := BasicAuth(next, "Multi", userpass, nil)

	tests := []struct {
		user string
		pass string
		code int
	}{
		{"admin", "adminpass", 200},
		{"user1", "user1pass", 200},
		{"user2", "user2pass", 200},
		{"user1", "user2pass", 401}, // wrong pass
		{"unknown", "adminpass", 401},
	}

	for _, tt := range tests {
		t.Run(tt.user, func(t *testing.T) {
			req := httptest.NewRequest("GET", "http://example.com/", nil)
			req.RemoteAddr = "1.2.3.4:12345"
			req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(tt.user+":"+tt.pass)))
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != tt.code {
				t.Errorf("%s:%s expected %d, got %d", tt.user, tt.pass, tt.code, rec.Code)
			}
		})
	}
}

func TestBasicAuth_IPPrecedence(t *testing.T) {
	// IP whitelist should take precedence over credentials
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("OK"))
	})

	userpass := map[string]string{
		"admin": "pass",
	}
	authlist := map[string]bool{
		"192.168.1.1": true,
	}
	handler := BasicAuth(next, "Test", userpass, authlist)

	// Whitelisted IP without credentials should still pass
	req := httptest.NewRequest("GET", "http://example.com/", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Errorf("Whitelisted IP should bypass auth: expected 200, got %d", rec.Code)
	}
}
