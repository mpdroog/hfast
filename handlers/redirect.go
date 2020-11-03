package handlers

import (
	"net"
	"net/http"
	"strings"
	"github.com/mpdroog/hfast/config"
	"github.com/mpdroog/hfast/logger"
)

func stripPort(hostport string) string {
	host, _, err := net.SplitHostPort(hostport)
	if err != nil {
		return hostport
	}
	return net.JoinHostPort(host, "443")
}
func normalizeHost(raw string) (host string, iswww bool) {
	host = strings.ToLower(raw)
	iswww = strings.HasPrefix(host, "www.")

	if iswww {
		host = host[len("www."):]
	}
	return
}

type RedirectHandler struct {
}

func (rh *RedirectHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" && r.Method != "HEAD" {
		http.Error(w, "Use HTTPS", http.StatusBadRequest)
		return
	}

	host, iswww := normalizeHost(r.Host)
	if _, ok := config.Muxs[host]; !ok {
		if !strings.HasPrefix(host, "127.0.0.1") {
			logger.Printf("Unmatched host: %s", host)
		}

		if mux, ok := config.Muxs["default"]; ok {
			mux.ServeHTTP(w, r)
			return
		}

		http.Error(w, "ERR: No such site.", http.StatusBadRequest)
		return
	}

	target := "https://" + stripPort(host) + r.URL.RequestURI()
	status := http.StatusFound
	if iswww {
		status = http.StatusMovedPermanently
	}
	http.Redirect(w, r, target, status)
}