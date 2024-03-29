package handlers

import (
	"crypto/subtle"
	"github.com/mpdroog/hfast/logger"
	"net"
	"net/http"
)

func BasicAuth(h http.Handler, realm string, userpass map[string]string, authlist map[string]bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, e := net.SplitHostPort(r.RemoteAddr)
		if e != nil {
			logger.Printf(e.Error())
			w.WriteHeader(500)
			w.Write([]byte("Failed parsing IP.\n"))
			return
		}
		whitelist, ok := authlist[host]
		if ok {
			if whitelist {
				// Whitelisted
				h.ServeHTTP(w, r)
			} else {
				// Blacklisted
				w.WriteHeader(403)
				w.Write([]byte("Blacklisted IP.\n"))
			}
			return
		}

		if len(userpass) == 0 {
			w.Header().Set("WWW-Authenticate", `Basic realm="`+realm+`"`)
			w.WriteHeader(401)
			w.Write([]byte("Unauthorised.\n"))
			return
		}

		user, pass, ok := r.BasicAuth()
		if !ok {
			w.Header().Set("WWW-Authenticate", `Basic realm="`+realm+`"`)
			w.WriteHeader(401)
			w.Write([]byte("Unauthorised.\n"))
			return
		}

		cfgPass, ok := userpass[user]
		if !ok || subtle.ConstantTimeCompare([]byte(pass), []byte(cfgPass)) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="`+realm+`"`)
			w.WriteHeader(401)
			w.Write([]byte("Unauthorised.\n"))
			return
		}

		h.ServeHTTP(w, r)
	})
}
