package handlers

import (
	"github.com/mpdroog/hfast/config"
	"github.com/mpdroog/hfast/logger"
	"net/http"
)

func Vhost() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		host, iswww := normalizeHost(r.Host)

		if iswww {
			// Redirect http(s)://www.domain to https://domain
			target := "https://" + stripPort(host) + r.URL.RequestURI()
			http.Redirect(w, r, target, http.StatusMovedPermanently)
			return
		}

		m, ok := config.Muxs[r.Host]
		if !ok {
			logger.Printf("Unmatched host: %s", r.Host)
			w.Write([]byte("ERR: No such site."))
			return
		}

		cfg, ok := config.Overrides[r.Host]
		if !ok {
			panic("Host set in muxs but not on overrides?")
		}

		// Extend with some vars for use in hfast/queue
		r.Header.Set("X-Domain", host)
		r.Header.Set("X-Secretkey", cfg.SecretKey)

		m.ServeHTTP(w, r)
		// Strip off sensitive info
		w.Header().Del("X-Powered-By")
		w.Header().Set("Server", "HFast")
	}
}
