// h2prox(y) is a small HTTP2 server that
// does traffic proxying.
package main

import (
	"flag"
	"fmt"
	"github.com/BurntSushi/toml"
	"github.com/coreos/go-systemd/daemon"
	"github.com/mpdroog/hfast/proxy"
	"net/http"
	"os"
	"sync"
	"time"
)

type Proxy struct {
	Host string
	Dest string
}
type Config struct {
	Proxy []Proxy
}

func vhost(muxs map[string]*http.ServeMux) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m, ok := muxs[r.Host]
		if !ok {
			m, ok = muxs["*"]
			if !ok {
				panic("ConfigErr. No default host with * in config.")
			}
		}
		m.ServeHTTP(w, r)
		// Strip off sensitive info
		w.Header().Del("X-Powered-By")
		w.Header().Set("Server", "HFast")
	}
}

func main() {
	configPath := ""
	//flag.BoolVar(&Verbose, "v", false, "Verbose-mode (log more)")
	flag.StringVar(&configPath, "c", "./config.toml", "Path to config.toml")
	flag.Parse()

	r, e := os.Open(configPath)
	if e != nil {
		panic(e)
	}
	defer r.Close()

	C := Config{}
	if _, e := toml.DecodeReader(r, &C); e != nil {
		panic(e)
	}

	muxs := make(map[string]*http.ServeMux)
	for _, cfg := range C.Proxy {
		fn, e := proxy.Proxy(cfg.Dest)
		if e != nil {
			panic(e)
		}

		mux := &http.ServeMux{}
		mux.Handle("/", fn)
		muxs[cfg.Host] = mux
	}

	vm := &http.ServeMux{}
	vm.Handle("/", vhost(muxs))

	s := &http.Server{
		Addr:         ":80",
		Handler:      vm,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	wg := new(sync.WaitGroup)
	wg.Add(1)
	go func() {
		if e := s.ListenAndServe(); e != nil && e != http.ErrServerClosed {
			panic(e)
		}
		wg.Done()
	}()

	sent, e := daemon.SdNotify(false, "READY=1")
	if e != nil {
		panic(e)
	}
	if !sent {
		fmt.Printf("SystemD notify NOT sent\n")
	}
	wg.Wait()
}
