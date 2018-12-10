// Accesslog
package main

import (
	"fmt"
	"io"
	"net/http"
	"time"
)

var mw io.Writer

func SetLog(w io.Writer) {
	mw = w
}

func AccessLog(h http.Handler) http.Handler {
	// TODO:
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		begin := time.Now()
		h.ServeHTTP(w, r)

		diff := time.Since(begin)
		log := fmt.Sprintf("time=%s host=%s method=%s url=%s ip=%s dur=%dns\n", time.Now().Unix(), r.Host, r.Method, r.URL, r.RemoteAddr, diff.Nanoseconds())
		mw.Write([]byte(log))
	})
}
