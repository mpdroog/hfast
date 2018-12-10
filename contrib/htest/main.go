// HTest is a mini HTTP-server for unittesting
// the action-code.
package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func fs(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotFound)
	if _, e := w.Write([]byte("Regular page handle, you forgot to call /action!")); e != nil {
		fmt.Printf("w.Write e=" + e.Error())
	}
}

func mux(m *http.ServeMux) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m.ServeHTTP(w, r)
	}
}

func main() {
	httpListen := ""
	flag.StringVar(&httpListen, "l", ":8081", "HTTP iface:port (to override port 80 binding)")
	flag.Parse()

	dir, e := os.Getwd()
	if e != nil {
		fmt.Printf("getwd e=%s\n", e.Error())
	}

	mux := &http.ServeMux{}
	mux.Handle("/action/", NewHandler(dir+"/action/index.php", "tcp", "127.0.0.1:9000"))
	mux.HandleFunc("/", fs)

	s := &http.Server{
		Addr:         httpListen,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	sigOS := make(chan os.Signal, 1)
	signal.Notify(sigOS, os.Interrupt)
	signal.Notify(sigOS, syscall.SIGTERM)

	go func() {
		panic(s.ListenAndServe())
	}()
	<-sigOS
}
