package main

import (
	"crypto/sha256"
	"flag"
	"fmt"
	"github.com/NYTimes/gziphandler"
	"github.com/VojtechVitek/ratelimit"
	"github.com/VojtechVitek/ratelimit/memory"
	"github.com/coreos/go-systemd/daemon"
	"log"
	"net"
	"net/http"
	"os"
	"time"
	"io"
)

const maxUploadSize = 1024*1024*1.5 // 1.5MB (500KB bigger than the browser)
var Verbose bool

func status(w http.ResponseWriter, r *http.Request) {
}

func chunk(w http.ResponseWriter, r *http.Request) {
	fname := r.URL.Query().Get("f")
	if len(fname) == 0 {
		http.Error(w, "missing GET[fname]", 400)
		return
	}
	part := r.URL.Query().Get("i")
	if len(part) == 0 {
		http.Error(w, "missing GET[i]", 400)
		return
	}
	total := r.URL.Query().Get("total")
	if len(part) == 0 {
		http.Error(w, "missing GET[total]", 400)
		return
	}
	// TODO: save total somewhere for checksumming?

	// TODO: Something stronger?
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(stripPort(r.RemoteAddr))))
	fdir := "./files/" + hash
	exist, e := exists(fdir)
	if e != nil {
		log.Println("Failed stat: " + fdir)
		http.Error(w, "Failed getting status", 500)
		return
	}
	if !exist {
		if e := os.MkdirAll(fdir, os.ModePerm); e != nil {
			log.Println("Failed creating dir: " + fdir)
			http.Error(w, "Failed creating dir", 500)
			return
		}
	}

	fname = fdir + "/" + fname // + "." + part
	exist, e = exists(fname)
	if e != nil {
		log.Println("Failed stat: " + fdir)
		http.Error(w, "Failed getting status", 500)
		return
	}
	if exist {
		if e := os.Remove(fname); e != nil {
			log.Println("Failed stat: " + fdir)
			http.Error(w, "Failed removing old chunk", 500)
			return
		}
	}

	// Upload
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)

	f, err := os.OpenFile(fname, os.O_WRONLY|os.O_CREATE, 0666)
   if err != nil {
       log.Println("openfile err=" + err.Error())
		http.Error(w, "Failed opening file for writing", 500)
		return
   }
   defer f.Close()
   defer r.Body.Close()
   if _, e := io.Copy(f, r.Body); e != nil {
   				log.Println("io.Copy err: " + e.Error())
			http.Error(w, "Failed writing chunk to fs", 500)
			return
   }

   log.Println("Uploaded " + fname)
   w.Write([]byte("OK."))
}

func stripPort(hostport string) string {
	host, _, err := net.SplitHostPort(hostport)
	if err != nil {
		return hostport
	}
	return net.JoinHostPort(host, "443")
}
func exists(path string) (ok bool, err error) {
	ok = true

	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			// file does not exist
			err = nil
			ok = false
		}
	}
	return
}

func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*") // TODO: Secure!
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Methods", "GET,HEAD,OPTIONS,POST,PUT")
		w.Header().Set("Access-Control-Allow-Headers", "Access-Control-Allow-Headers, Origin,Accept, X-Requested-With, Content-Type, Access-Control-Request-Method, Access-Control-Request-Headers")
		if r.Method == "OPTIONS" {
			w.Write([]byte("CORS :)"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func main() {
	httpListen := ""
	flag.StringVar(&httpListen, "l", ":8080", "HTTP iface:port (to override port 8080 binding)")
	flag.BoolVar(&Verbose, "v", false, "Verbose-mode (log more)")
	flag.Parse()

	limit := ratelimit.Request(ratelimit.IP).Rate(60, time.Minute).LimitBy(memory.New()) // 60req/min
	fs := gziphandler.GzipHandler(http.FileServer(http.Dir("./pub")))

	mux := &http.ServeMux{}
	mux.HandleFunc("/action/uploads", status)
	mux.HandleFunc("/action/uploads/chunk", chunk)
	mux.Handle("/", fs)

	s := &http.Server{
		Addr:         httpListen,
		Handler:      CORS(limit(mux)),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	sent, e := daemon.SdNotify(false, "READY=1")
	if e != nil {
		log.Fatal(e)
	}
	if !sent {
		log.Printf("SystemD notify NOT sent\n")
	}

	log.Panic(s.ListenAndServe())
}
