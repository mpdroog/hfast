package main

import (
	"crypto/tls"
	"fmt"
	"github.com/VojtechVitek/ratelimit"
	"github.com/coreos/go-systemd/daemon"
	"golang.org/x/crypto/acme/autocert"
	"log"
	"net/http"
	"time"
	"github.com/NYTimes/gziphandler"
	"io/ioutil"
)

var assets = []string{
	"/css/bootstrap.min.css", "/css/design.css", "/img/bg.jpg",
}

func push(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Vary", "Accept-Encoding")
		w.Header().Set("x-frame-options", "SAMEORIGIN")
		w.Header().Set("x-xss-protection", "1; mode=block")
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

func vhost(muxs map[string]*http.ServeMux) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m, ok := muxs[r.Host]
		if !ok {
			log.Printf("Unmatched host: %s", r.Host)
			w.Write([]byte("ERR: No such site."))
			return
		}
		m.ServeHTTP(w, r)
	}
}

func getDomains() ([]string, error) {
	fileInfo, err := ioutil.ReadDir("/var/www/")
	if err != nil {
		return nil, err
	}

	out := []string{}
	for _, file := range fileInfo {
		if file.IsDir() {
			out = append(out, file.Name())
		}
	}
	return out, nil
}

func main() {
	domains, e := getDomains()
	if e != nil {
		panic(e)
	}

	muxs := make(map[string]*http.ServeMux)
	limit := ratelimit.Throttle(1)
	for _, domain := range domains {
		fs := gziphandler.GzipHandler(push(http.FileServer(http.Dir(fmt.Sprintf("/var/www/%s/pub", domain)))))
		action := gziphandler.GzipHandler(limit(NewHandler(fmt.Sprintf("/var/www/%s/action/index.php", domain), "tcp", "127.0.0.1:8000")))

		mux := &http.ServeMux{}
	        mux.Handle("/action/", action)
		mux.Handle("/", fs)
		muxs[domain] = mux
	}

	m := &autocert.Manager{
                Cache:      autocert.DirCache("/tmp/certs"),
                Prompt:     autocert.AcceptTOS,
                HostPolicy: autocert.HostWhitelist(domains...),
        }
	s := &http.Server{
                Addr:         ":443",
                TLSConfig:    &tls.Config{GetCertificate: m.GetCertificate},
                Handler:      vhost(muxs),
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
