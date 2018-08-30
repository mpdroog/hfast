package main

import (
	"crypto/tls"
	"github.com/VojtechVitek/ratelimit"
	"github.com/coreos/go-systemd/daemon"
	"golang.org/x/crypto/acme/autocert"
	"log"
	"net/http"
	"time"
	"github.com/NYTimes/gziphandler"
)

var assets = []string{
	"/css/bootstrap.min.css", "/css/design.css", "/img/bg.jpg",
}

func push(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Vary", "Accept-Encoding")
		if r.URL.Path == "/" {
			if pusher, ok := w.(http.Pusher); ok {
				for _, asset := range assets {
					if err := pusher.Push(asset, nil); err != nil {
						log.Printf("Failed push: %v", err)
						break
					}
				}
			}
		}
		h.ServeHTTP(w, r)
	})
}

func main() {
	fs := http.FileServer(http.Dir("/var/www/usenet.today/pub"))

	limit := ratelimit.Throttle(1)
	mux := &http.ServeMux{}
	mux.Handle("/action/", gziphandler.GzipHandler(limit(NewHandler("/var/www/usenet.today/action/index.php", "tcp", "127.0.0.1:8000"))))
	mux.Handle("/", gziphandler.GzipHandler(push(fs)))

	m := &autocert.Manager{
		Cache:      autocert.DirCache("/var/www/usenet.today/certs"),
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist("usenet.today"),
	}
	s := &http.Server{
		Addr:         ":443",
		TLSConfig:    &tls.Config{GetCertificate: m.GetCertificate},
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go http.ListenAndServe("", m.HTTPHandler(nil))

	sent, e := daemon.SdNotify(false, "READY=1")
	if e != nil {
		log.Fatal(e)
	}
	if !sent {
		log.Printf("SystemD notify NOT sent\n")
	}

	log.Panic(s.ListenAndServeTLS("", ""))
}
