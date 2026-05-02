package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// dummyHandler is used as the next handler in the middleware chain
func dummyHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("OK"))
	})
}

// TestCSRF_SafeMethods tests that GET, HEAD, OPTIONS are always allowed
func TestCSRF_SafeMethods(t *testing.T) {
	handler := CSRF(dummyHandler())

	methods := []string{"GET", "HEAD", "OPTIONS"}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "http://example.com/path", nil)
			// No Origin or Referer header
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != 200 {
				t.Errorf("%s without Origin/Referer: expected 200, got %d", method, rec.Code)
			}
		})
	}
}

// TestCSRF_POST_ValidOrigin tests POST with matching Origin header
func TestCSRF_POST_ValidOrigin(t *testing.T) {
	handler := CSRF(dummyHandler())

	tests := []struct {
		name   string
		host   string
		origin string
	}{
		{"exact match", "example.com", "https://example.com"},
		{"with port in origin", "example.com", "https://example.com:443"},
		{"with port in host", "example.com:443", "https://example.com"},
		{"both with port", "example.com:443", "https://example.com:443"},
		{"http scheme", "example.com", "http://example.com"},
		{"case insensitive", "Example.COM", "https://example.com"},
		{"subdomain exact", "api.example.com", "https://api.example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "http://"+tt.host+"/action", nil)
			req.Host = tt.host
			req.Header.Set("Origin", tt.origin)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != 200 {
				t.Errorf("POST with valid Origin %q to host %q: expected 200, got %d", tt.origin, tt.host, rec.Code)
			}
		})
	}
}

// TestCSRF_POST_ValidReferer tests POST with matching Referer header (no Origin)
func TestCSRF_POST_ValidReferer(t *testing.T) {
	handler := CSRF(dummyHandler())

	tests := []struct {
		name    string
		host    string
		referer string
	}{
		{"exact match", "example.com", "https://example.com/page"},
		{"with path and query", "example.com", "https://example.com/path?query=1"},
		{"with port", "example.com", "https://example.com:443/page"},
		{"case insensitive", "Example.COM", "https://example.com/page"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "http://"+tt.host+"/action", nil)
			req.Host = tt.host
			req.Header.Set("Referer", tt.referer)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != 200 {
				t.Errorf("POST with valid Referer %q to host %q: expected 200, got %d", tt.referer, tt.host, rec.Code)
			}
		})
	}
}

// TestCSRF_POST_InvalidOrigin tests POST with non-matching Origin header
func TestCSRF_POST_InvalidOrigin(t *testing.T) {
	handler := CSRF(dummyHandler())

	tests := []struct {
		name   string
		host   string
		origin string
	}{
		{"different domain", "example.com", "https://evil.com"},
		{"subdomain mismatch", "example.com", "https://sub.example.com"},
		{"parent domain", "sub.example.com", "https://example.com"},
		{"similar domain", "example.com", "https://example.com.evil.com"},
		{"null origin", "example.com", "null"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "http://"+tt.host+"/action", nil)
			req.Host = tt.host
			req.Header.Set("Origin", tt.origin)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != 403 {
				t.Errorf("POST with invalid Origin %q to host %q: expected 403, got %d", tt.origin, tt.host, rec.Code)
			}
		})
	}
}

// TestCSRF_POST_InvalidReferer tests POST with non-matching Referer header
func TestCSRF_POST_InvalidReferer(t *testing.T) {
	handler := CSRF(dummyHandler())

	tests := []struct {
		name    string
		host    string
		referer string
	}{
		{"different domain", "example.com", "https://evil.com/page"},
		{"subdomain mismatch", "example.com", "https://sub.example.com/page"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "http://"+tt.host+"/action", nil)
			req.Host = tt.host
			req.Header.Set("Referer", tt.referer)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != 403 {
				t.Errorf("POST with invalid Referer %q to host %q: expected 403, got %d", tt.referer, tt.host, rec.Code)
			}
		})
	}
}

// TestCSRF_POST_NoHeaders tests POST without Origin or Referer (strict mode rejects)
func TestCSRF_POST_NoHeaders(t *testing.T) {
	handler := CSRF(dummyHandler())

	req := httptest.NewRequest("POST", "http://example.com/action", nil)
	req.Host = "example.com"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != 403 {
		t.Errorf("POST without Origin/Referer: expected 403, got %d", rec.Code)
	}
}

// TestCSRF_OtherMethods tests PUT, DELETE, PATCH are also protected
func TestCSRF_OtherMethods(t *testing.T) {
	handler := CSRF(dummyHandler())

	methods := []string{"PUT", "DELETE", "PATCH"}
	for _, method := range methods {
		t.Run(method+"_no_headers", func(t *testing.T) {
			req := httptest.NewRequest(method, "http://example.com/resource", nil)
			req.Host = "example.com"
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != 403 {
				t.Errorf("%s without Origin/Referer: expected 403, got %d", method, rec.Code)
			}
		})

		t.Run(method+"_valid_origin", func(t *testing.T) {
			req := httptest.NewRequest(method, "http://example.com/resource", nil)
			req.Host = "example.com"
			req.Header.Set("Origin", "https://example.com")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != 200 {
				t.Errorf("%s with valid Origin: expected 200, got %d", method, rec.Code)
			}
		})

		t.Run(method+"_invalid_origin", func(t *testing.T) {
			req := httptest.NewRequest(method, "http://example.com/resource", nil)
			req.Host = "example.com"
			req.Header.Set("Origin", "https://evil.com")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != 403 {
				t.Errorf("%s with invalid Origin: expected 403, got %d", method, rec.Code)
			}
		})
	}
}

// TestCSRF_OriginTakesPrecedence tests that Origin is checked before Referer
func TestCSRF_OriginTakesPrecedence(t *testing.T) {
	handler := CSRF(dummyHandler())

	// Valid Origin, invalid Referer -> should pass (Origin takes precedence)
	t.Run("valid_origin_invalid_referer", func(t *testing.T) {
		req := httptest.NewRequest("POST", "http://example.com/action", nil)
		req.Host = "example.com"
		req.Header.Set("Origin", "https://example.com")
		req.Header.Set("Referer", "https://evil.com/page")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != 200 {
			t.Errorf("Valid Origin with invalid Referer: expected 200, got %d", rec.Code)
		}
	})

	// Invalid Origin, valid Referer -> should fail (Origin takes precedence)
	t.Run("invalid_origin_valid_referer", func(t *testing.T) {
		req := httptest.NewRequest("POST", "http://example.com/action", nil)
		req.Host = "example.com"
		req.Header.Set("Origin", "https://evil.com")
		req.Header.Set("Referer", "https://example.com/page")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != 403 {
			t.Errorf("Invalid Origin with valid Referer: expected 403, got %d", rec.Code)
		}
	})
}

// TestCSRF_MalformedHeaders tests handling of malformed Origin/Referer
func TestCSRF_MalformedHeaders(t *testing.T) {
	handler := CSRF(dummyHandler())

	tests := []struct {
		name   string
		origin string
	}{
		{"invalid url", "not-a-valid-url"},
		{"empty scheme", "://example.com"},
		{"spaces", "https://example .com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "http://example.com/action", nil)
			req.Host = "example.com"
			req.Header.Set("Origin", tt.origin)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != 403 {
				t.Errorf("POST with malformed Origin %q: expected 403, got %d", tt.origin, rec.Code)
			}
		})
	}
}

// TestCSRF_IPv6 tests IPv6 address handling
func TestCSRF_IPv6(t *testing.T) {
	handler := CSRF(dummyHandler())

	tests := []struct {
		name     string
		host     string
		origin   string
		expected int
	}{
		{"ipv6 match", "[::1]", "http://[::1]", 200},
		{"ipv6 with port match", "[::1]:8080", "http://[::1]:8080", 200},
		{"ipv6 with port vs without", "[::1]:8080", "http://[::1]", 200},
		{"ipv6 mismatch", "[::1]", "http://[::2]", 403},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "http://"+tt.host+"/action", nil)
			req.Host = tt.host
			req.Header.Set("Origin", tt.origin)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != tt.expected {
				t.Errorf("IPv6 test %q: expected %d, got %d", tt.name, tt.expected, rec.Code)
			}
		})
	}
}

// TestCSRFStripPort tests the port stripping helper function
func TestCSRFStripPort(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"example.com", "example.com"},
		{"example.com:443", "example.com"},
		{"example.com:8080", "example.com"},
		{"[::1]", "[::1]"},
		{"[::1]:8080", "[::1]"},
		{"[2001:db8::1]", "[2001:db8::1]"},
		{"[2001:db8::1]:443", "[2001:db8::1]"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := csrfStripPort(tt.input)
			if result != tt.expected {
				t.Errorf("csrfStripPort(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestCSRF_ResponseBody tests that the error response body is correct
func TestCSRF_ResponseBody(t *testing.T) {
	handler := CSRF(dummyHandler())

	req := httptest.NewRequest("POST", "http://example.com/action", nil)
	req.Host = "example.com"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != 403 {
		t.Errorf("Expected 403, got %d", rec.Code)
	}
	if rec.Body.String() != "CSRF validation failed\n" {
		t.Errorf("Unexpected body: %q", rec.Body.String())
	}
}

// TestCSRF_NextHandlerCalled tests that the next handler is called on success
func TestCSRF_NextHandlerCalled(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(200)
	})

	handler := CSRF(next)

	req := httptest.NewRequest("POST", "http://example.com/action", nil)
	req.Host = "example.com"
	req.Header.Set("Origin", "https://example.com")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("Next handler was not called on valid CSRF request")
	}
}

// TestCSRF_NextHandlerNotCalledOnFailure tests that next handler is NOT called on CSRF failure
func TestCSRF_NextHandlerNotCalledOnFailure(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(200)
	})

	handler := CSRF(next)

	req := httptest.NewRequest("POST", "http://example.com/action", nil)
	req.Host = "example.com"
	req.Header.Set("Origin", "https://evil.com")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if called {
		t.Error("Next handler should NOT be called on CSRF failure")
	}
}
