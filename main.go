package main

import (
	"crypto/tls"
	"fmt"
	"github.com/BurntSushi/toml"
	"github.com/NYTimes/gziphandler"
	"github.com/VojtechVitek/ratelimit"
	"github.com/VojtechVitek/ratelimit/memory"
	"github.com/coreos/go-systemd/daemon"
	"github.com/mpdroog/hfast/proxy"
	"github.com/unrolled/secure"
	"golang.org/x/crypto/acme/autocert"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"
)

type Overrides struct {
	Proxy string // Just forward to given addr
}

var pushAssets map[string][]string

func init() {
	pushAssets = make(map[string][]string)
}

func push(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Vary", "Accept-Encoding")
		if assets, ok := pushAssets[r.Host]; r.URL.Path == "/" && ok {
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
		// Strip off sensitive info
		w.Header().Del("X-Powered-By")
		w.Header().Set("Server", "HFast")
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

func exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return true, err
}

func getPush(path string) ([]string, error) {
	ok, err := exists(path)
	if err != nil || !ok {
		return nil, err
	}

	fileInfo, err := ioutil.ReadDir(path)
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

func getOverrides(path string) (Overrides, error) {
	c := Overrides{}

	if _, e := os.Stat(path); os.IsNotExist(e) {
		return c, nil
	}

	r, e := os.Open(path)
	if e != nil {
		return c, e
	}
	defer r.Close()
	_, e = toml.DecodeReader(r, &c)
	return c, e
}

func main() {
	domains, e := getDomains()
	if e != nil {
		panic(e)
	}

	muxs := make(map[string]*http.ServeMux)
	for _, domain := range domains {

		overrides, e := getOverrides(fmt.Sprintf("/var/www/%s/override.toml", domain))
		if e != nil {
			panic(e)
		}

		if len(overrides.Proxy) > 0 {
			fn, e := proxy.Proxy(overrides.Proxy)
			if e != nil {
				panic(e)
			}
			mux := &http.ServeMux{}
			mux.Handle("/", fn)
			muxs[domain] = mux
			continue
		}

		fs := gziphandler.GzipHandler(push(http.FileServer(http.Dir(fmt.Sprintf("/var/www/%s/pub", domain)))))
		limit := ratelimit.Request(ratelimit.IP).Rate(30, time.Minute).LimitBy(memory.New()) // 30req/min
		action := gziphandler.GzipHandler(limit(NewHandler(fmt.Sprintf("/var/www/%s/action/index.php", domain), "tcp", "127.0.0.1:8000")))

		mux := &http.ServeMux{}
		mux.Handle("/action/", action)
		mux.Handle("/", fs)
		muxs[domain] = mux

		a, e := getPush(fmt.Sprintf("/var/www/%s/pub/push", domain))
		if e != nil {
			panic(e)
		}
		pushAssets[domain] = a
	}

	m := &autocert.Manager{
		Cache:      autocert.DirCache("/tmp/certs"),
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist(domains...),
	}

	secureMiddleware := secure.New(secure.Options{
		AllowedHosts:          domains,
		HostsProxyHeaders:     []string{"X-Forwarded-Host"},
		SSLRedirect:           false, // autocert takes care of this
		SSLHost:               "",
		SSLProxyHeaders:       map[string]string{"X-Forwarded-Proto": "https"},
		STSSeconds:            315360000,
		STSIncludeSubdomains:  false,
		STSPreload:            true,
		FrameDeny:             true,
		ContentTypeNosniff:    true,
		BrowserXssFilter:      true,
		ContentSecurityPolicy: "script-src $NONCE",
	})
	app := secureMiddleware.Handler(vhost(muxs))
	s := &http.Server{
		Addr:         ":443",
		TLSConfig:    &tls.Config{GetCertificate: m.GetCertificate},
		Handler:      app,
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
