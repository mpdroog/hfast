package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORS_Headers(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})

	handler := CORS(next)

	req := httptest.NewRequest("GET", "http://example.com/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	tests := []struct {
		header   string
		expected string
	}{
		{"Access-Control-Allow-Origin", "*"},
		{"Access-Control-Allow-Credentials", "true"},
		{"Access-Control-Allow-Methods", "GET,HEAD,OPTIONS,POST,PUT"},
		{"Access-Control-Allow-Headers", "Access-Control-Allow-Headers, Origin,Accept, X-Requested-With, Content-Type, Access-Control-Request-Method, Access-Control-Request-Headers,AMP-Redirect-To"},
	}

	for _, tt := range tests {
		t.Run(tt.header, func(t *testing.T) {
			got := rec.Header().Get(tt.header)
			if got != tt.expected {
				t.Errorf("%s: expected %q, got %q", tt.header, tt.expected, got)
			}
		})
	}
}

func TestCORS_OPTIONS_Preflight(t *testing.T) {
	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(200)
	})

	handler := CORS(next)

	req := httptest.NewRequest("OPTIONS", "http://example.com/api/data", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// OPTIONS should return early with CORS response
	if rec.Body.String() != "CORS :)" {
		t.Errorf("OPTIONS body: expected 'CORS :)', got %q", rec.Body.String())
	}

	// Next handler should NOT be called for OPTIONS
	if nextCalled {
		t.Error("Next handler should not be called for OPTIONS request")
	}

	// CORS headers should still be set
	if rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("CORS headers missing on OPTIONS response")
	}
}

func TestCORS_PassThrough(t *testing.T) {
	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD"}

	for _, method := range methods {
		if method == "OPTIONS" {
			continue // OPTIONS is handled specially
		}

		t.Run(method, func(t *testing.T) {
			nextCalled := false
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true
				w.WriteHeader(200)
				w.Write([]byte("Next handler"))
			})

			handler := CORS(next)

			req := httptest.NewRequest(method, "http://example.com/", nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if !nextCalled {
				t.Errorf("%s: next handler should be called", method)
			}

			// CORS headers should be set for all methods
			if rec.Header().Get("Access-Control-Allow-Origin") != "*" {
				t.Errorf("%s: CORS headers missing", method)
			}
		})
	}
}

func TestCORS_PreservesNextHandlerResponse(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Custom-Header", "custom-value")
		w.WriteHeader(201)
		w.Write([]byte("Created"))
	})

	handler := CORS(next)

	req := httptest.NewRequest("POST", "http://example.com/resource", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != 201 {
		t.Errorf("Expected status 201, got %d", rec.Code)
	}
	if rec.Body.String() != "Created" {
		t.Errorf("Body should be preserved: got %q", rec.Body.String())
	}
	if rec.Header().Get("X-Custom-Header") != "custom-value" {
		t.Error("Custom header should be preserved")
	}
}
