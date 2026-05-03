package proxy

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type bufferWriter struct {
	header http.Header
	code   int
	buffer *bytes.Buffer
}

func (b *bufferWriter) Close() error {
	return nil
}

func (b *bufferWriter) Header() http.Header {
	return b.header
}

func (b *bufferWriter) Write(buf []byte) (int, error) {
	return b.buffer.Write(buf)
}

// WriteHeader sets rw.Code.
func (b *bufferWriter) WriteHeader(code int) {
	b.code = code
}

func TestStripPort(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"example.com", "example.com"},
		{"example.com:443", "example.com"},
		{"example.com:8080", "example.com"},
		{"192.168.1.1:80", "192.168.1.1"},
		{"localhost:3000", "localhost"},
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

func TestProxy_XForwardedHostStripsPort(t *testing.T) {
	var capturedHost string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		capturedHost = req.Header.Get("X-Forwarded-Host")
		w.WriteHeader(200)
	}))
	defer ts.Close()

	fn, err := Proxy("http://" + ts.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}

	// Request with port in Host header
	req := httptest.NewRequest("GET", "/test", nil)
	req.Host = "example.com:443"
	rec := httptest.NewRecorder()
	fn(rec, req)

	if capturedHost != "example.com" {
		t.Errorf("X-Forwarded-Host should strip port: got %q, want %q", capturedHost, "example.com")
	}
}

func TestProxy(t *testing.T) {
	bw := &bufferWriter{header: make(http.Header), buffer: &bytes.Buffer{}}
	b := strings.NewReader(`Hello world`)

	// Temp server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if url := req.URL.String(); url != "/LP_TA/index.cfm?CTP=AF%5FTA%2CTSYqLzdTL1MtUFglIFEoJzcsTFwuM1ohNDEqR0E%2BW0YlSCgyNEdMSD4nWz46IFkiKE4gR0dGUTU4USs1SQpNSCktQ1IqUjI4LlxTTDBQNF9LOzJIWkAqLjs6IUc%2BLEpDOlg2QyhOI0lQVVBeSlY1XFBNTzdQV0EtOldMCjJdTEkmWFxJMUc9Nyc6WFNeW1xASlJPUyIK&FN=test" {
			t.Errorf("URL malformed during forwarding, received=%s", url)
		}
		if head := req.Header.Get("Test"); head != "Is Forwarded" {
			t.Errorf("Header invalid/missing Test-field, received=%s", head)
		}
		defer req.Body.Close()
		b := new(bytes.Buffer)
		if _, e := io.Copy(b, req.Body); e != nil {
			t.Errorf("io.Copy failed: %s", e.Error())
		}
		if b.String() != "Hello world" {
			t.Errorf("b.String() mismatch %s", b.String())
		}

		if _, e := w.Write([]byte("Reply")); e != nil {
			t.Errorf("w.Write failed: %s", e.Error())
		}
	}))
	defer ts.Close()

	fn, e := Proxy("http://" + ts.Listener.Addr().String())
	if e != nil {
		t.Fatal(e)
	}

	// Let the forwarding begin
	r := httptest.NewRequest("GET", "/LP_TA/index.cfm?CTP=AF%5FTA%2CTSYqLzdTL1MtUFglIFEoJzcsTFwuM1ohNDEqR0E%2BW0YlSCgyNEdMSD4nWz46IFkiKE4gR0dGUTU4USs1SQpNSCktQ1IqUjI4LlxTTDBQNF9LOzJIWkAqLjs6IUc%2BLEpDOlg2QyhOI0lQVVBeSlY1XFBNTzdQV0EtOldMCjJdTEkmWFxJMUc9Nyc6WFNeW1xASlJPUyIK&FN=test", b)
	r.Header.Set("Test", "Is Forwarded")
	fn(bw, r)
	if bw.buffer.String() != "Reply" {
		t.Errorf("buffer not 'Reply' as expected")
	}
}
