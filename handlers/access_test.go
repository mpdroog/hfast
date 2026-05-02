package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStatusWriter_CapturesStatus(t *testing.T) {
	rec := httptest.NewRecorder()
	sw := &statusWriter{ResponseWriter: rec}

	sw.WriteHeader(201)

	if sw.Status != 201 {
		t.Errorf("Status: expected 201, got %d", sw.Status)
	}
}

func TestStatusWriter_DefaultStatus200(t *testing.T) {
	rec := httptest.NewRecorder()
	sw := &statusWriter{ResponseWriter: rec}

	// Write without explicit WriteHeader
	sw.Write([]byte("Hello"))

	if sw.Status != 200 {
		t.Errorf("Default status: expected 200, got %d", sw.Status)
	}
}

func TestStatusWriter_CapturesLength(t *testing.T) {
	rec := httptest.NewRecorder()
	sw := &statusWriter{ResponseWriter: rec}

	sw.Write([]byte("Hello"))
	sw.Write([]byte(" World"))

	if sw.Length != 11 {
		t.Errorf("Length: expected 11, got %d", sw.Length)
	}
}

func TestStatusWriter_Header(t *testing.T) {
	rec := httptest.NewRecorder()
	sw := &statusWriter{ResponseWriter: rec}

	sw.Header().Set("X-Custom", "value")

	if rec.Header().Get("X-Custom") != "value" {
		t.Error("Header should be set on underlying ResponseWriter")
	}
}

func TestAccessLog_LogsRequest(t *testing.T) {
	// Setup log output
	var buf bytes.Buffer
	SetLog(&buf)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("OK"))
	})

	handler := AccessLog(next)

	req := httptest.NewRequest("GET", "http://example.com/page?q=1", nil)
	req.Host = "example.com"
	req.RemoteAddr = "192.168.1.1:12345"
	req.Header.Set("User-Agent", "TestAgent/1.0")
	req.Header.Set("Referer", "https://google.com")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Parse the JSON log entry
	var msg Msg
	if err := json.Unmarshal(buf.Bytes(), &msg); err != nil {
		t.Fatalf("Failed to parse log JSON: %v", err)
	}

	if msg.Method != "GET" {
		t.Errorf("Method: expected GET, got %s", msg.Method)
	}
	if msg.Host != "example.com" {
		t.Errorf("Host: expected example.com, got %s", msg.Host)
	}
	// httptest.NewRequest creates full URL, actual requests have path only
	if msg.URL != "http://example.com/page?q=1" && msg.URL != "/page?q=1" {
		t.Errorf("URL: expected path or full URL, got %s", msg.URL)
	}
	if msg.Status != 200 {
		t.Errorf("Status: expected 200, got %d", msg.Status)
	}
	if msg.Remote != "192.168.1.1:12345" {
		t.Errorf("Remote: expected 192.168.1.1:12345, got %s", msg.Remote)
	}
	if msg.UA != "TestAgent/1.0" {
		t.Errorf("UA: expected TestAgent/1.0, got %s", msg.UA)
	}
	if msg.Len != 2 { // "OK" = 2 bytes
		t.Errorf("Len: expected 2, got %d", msg.Len)
	}
	if msg.Referer != "https://google.com" {
		t.Errorf("Referer: expected https://google.com, got %s", msg.Referer)
	}
}

func TestAccessLog_CapturesStatusCodes(t *testing.T) {
	var buf bytes.Buffer
	SetLog(&buf)

	tests := []int{200, 201, 301, 400, 404, 500}

	for _, code := range tests {
		buf.Reset()

		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(code)
		})

		handler := AccessLog(next)
		req := httptest.NewRequest("GET", "http://example.com/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		var msg Msg
		if err := json.Unmarshal(buf.Bytes(), &msg); err != nil {
			t.Fatalf("Failed to parse log: %v", err)
		}

		if msg.Status != code {
			t.Errorf("Status %d: got %d", code, msg.Status)
		}
	}
}

func TestAccessLog_CapturesProto(t *testing.T) {
	var buf bytes.Buffer
	SetLog(&buf)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})

	handler := AccessLog(next)

	req := httptest.NewRequest("GET", "http://example.com/", nil)
	req.Proto = "HTTP/2.0"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	var msg Msg
	json.Unmarshal(buf.Bytes(), &msg)

	if msg.Proto != "HTTP/2.0" {
		t.Errorf("Proto: expected HTTP/2.0, got %s", msg.Proto)
	}
}

func TestAccessLog_CapturesRatelimitHeader(t *testing.T) {
	var buf bytes.Buffer
	SetLog(&buf)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Ratelimit-Remaining", "25")
		w.WriteHeader(200)
	})

	handler := AccessLog(next)

	req := httptest.NewRequest("GET", "http://example.com/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	var msg Msg
	json.Unmarshal(buf.Bytes(), &msg)

	if msg.Ratelimit != "25" {
		t.Errorf("Ratelimit: expected 25, got %s", msg.Ratelimit)
	}
}

func TestAccessLog_DateTimeFormat(t *testing.T) {
	var buf bytes.Buffer
	SetLog(&buf)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})

	handler := AccessLog(next)

	req := httptest.NewRequest("GET", "http://example.com/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	var msg Msg
	json.Unmarshal(buf.Bytes(), &msg)

	// Date should be in YYYY-MM-DD format
	if len(msg.Date) != 10 || msg.Date[4] != '-' || msg.Date[7] != '-' {
		t.Errorf("Date format invalid: %s", msg.Date)
	}

	// Time should be in HH:MM:SS format
	if len(msg.Time) != 8 || msg.Time[2] != ':' || msg.Time[5] != ':' {
		t.Errorf("Time format invalid: %s", msg.Time)
	}
}

func TestAccessLog_NextHandlerCalled(t *testing.T) {
	var buf bytes.Buffer
	SetLog(&buf)

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(200)
	})

	handler := AccessLog(next)

	req := httptest.NewRequest("GET", "http://example.com/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !nextCalled {
		t.Error("Next handler should be called")
	}
}

func TestMsgPool_GetAndPut(t *testing.T) {
	msg1 := msgPool.Get()
	if msg1 == nil {
		t.Fatal("Get() returned nil")
	}

	msg1.Method = "GET"
	msg1.Host = "test.com"

	msgPool.Put(msg1)

	// Get another - may or may not be the same instance
	msg2 := msgPool.Get()
	if msg2 == nil {
		t.Fatal("Get() returned nil after Put()")
	}
}
