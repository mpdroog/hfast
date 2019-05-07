// Accesslog
package main

import (
	"fmt"
	"github.com/mpdroog/hfast/logger"
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
		msg := fmt.Sprintf("%s %s%s remote=%s ratelimit.remain=%s dur=%dns\n", r.Method, r.Host, r.URL, r.RemoteAddr, w.Header().Get("X-Ratelimit-Remaining"), diff.Nanoseconds())

		if int(diff.Seconds()) > 5 {
			logger.Printf("perf-warning(taking longer than 5sec): " + msg)
		}

		mw.Write([]byte(msg))
	})
}
