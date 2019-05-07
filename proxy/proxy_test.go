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

func TestProxy(t *testing.T) {
	bw := &bufferWriter{header: make(http.Header), buffer: &bytes.Buffer{}}
	b := strings.NewReader(`Hello world`)

	// Temp server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if url := req.URL.String(); url != "/LP_TA/index.cfm?CTP=AF%5FTA%2CTSYqLzdTL1MtUFglIFEoJzcsTFwuM1ohNDEqR0E%2BW0YlSCgyNEdMSD4nWz46IFkiKE4gR0dGUTU4USs1SQpNSCktQ1IqUjI4LlxTTDBQNF9LOzJIWkAqLjs6IUc%2BLEpDOlg2QyhOI0lQVVBeSlY1XFBNTzdQV0EtOldMCjJdTEkmWFxJMUc9Nyc6WFNeW1xASlJPUyIK&FN=test" {
			t.Errorf("URL malformed during forwarding, received=" + url)
		}
		if head := req.Header.Get("Test"); head != "Is Forwarded" {
			t.Errorf("Header invalid/missing Test-field, received=" + head)
		}
		defer req.Body.Close()
		b := new(bytes.Buffer)
		if _, e := io.Copy(b, req.Body); e != nil {
			t.Errorf("io.Copy failed: " + e.Error())
		}
		if b.String() != "Hello world" {
			t.Errorf("b.String() mismatch " + b.String())
		}

		if _, e := w.Write([]byte("Reply")); e != nil {
			t.Errorf("w.Write failed: " + e.Error())
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
