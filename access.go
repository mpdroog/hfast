// Accesslog
package main

import (
	"encoding/json"
	"github.com/mpdroog/hfast/logger"
	"io"
	"net/http"
	"time"
)

var (
	enc *json.Encoder
)

func SetLog(w io.Writer) {
	enc = json.NewEncoder(w)
}

type Msg struct {
	Method    string
	Host      string
	URL       string
	Status    int
	Remote    string
	Ratelimit string
	Duration  int64
	UA        string
	Proto     string
	Len       uint64
	Date      string
	Time      string
	Referer   string
}

type statusWriter struct {
	http.ResponseWriter
	Status int
	Length uint64
}

func (w *statusWriter) Header() http.Header {
	return w.ResponseWriter.Header()
}

func (w *statusWriter) WriteHeader(status int) {
	w.Status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.Status == 0 {
		w.Status = 200
	}
	n, err := w.ResponseWriter.Write(b)
	w.Length += uint64(n)
	return n, err
}

func AccessLog(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		begin := time.Now()
		sw := &statusWriter{ResponseWriter: w}
		h.ServeHTTP(sw, r)

		// TODO: Re-use objects?
		diff := time.Since(begin)
		msg := Msg{}
		msg.Method = r.Method
		msg.Host = r.Host
		msg.URL = r.URL.String()
		msg.Status = sw.Status
		msg.Remote = r.RemoteAddr
		msg.Ratelimit = w.Header().Get("X-Ratelimit-Remaining")
		msg.Duration = int64(diff.Seconds())
		msg.UA = r.Header.Get("User-Agent")
		msg.Proto = r.Proto
		msg.Len = sw.Length
		msg.Date = begin.Format("2006-01-02")
		msg.Time = begin.Format("15:04:05")
		msg.Referer = r.Referer()

		if e := enc.Encode(msg); e != nil {
			logger.Printf("accesslog: " + e.Error())
		}
		if int(diff.Seconds()) > 5 {
			logger.Printf("perf_slow: " + msg.Method + " " + msg.Host + " " + msg.URL)
		}
		if sw.Length > 1024*1024*100 {
			logger.Printf("perf_big: " + msg.Method + " " + msg.Host + " " + msg.URL)
		}
	})
}
