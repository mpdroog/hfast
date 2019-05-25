package main

import (
	"flag"
	"fmt"
	"github.com/BurntSushi/toml"
	"github.com/NYTimes/gziphandler"
	"github.com/VojtechVitek/ratelimit"
	"github.com/VojtechVitek/ratelimit/memory"
	"github.com/coreos/go-systemd/daemon"
	"github.com/mpdroog/hfast/logger"
	"github.com/mpdroog/hfast/proxy"
	"golang.org/x/crypto/acme/autocert"
	"golang.org/x/net/netutil"
	"golang.org/x/text/language"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

type Override struct {
	Proxy           string // Just forward to given addr
	ExcludedDomains []string
	Lang            []string
	Admin           map[string]string // Admin user+pass
	DevMode         bool              // Only allow admin user+pass
	Authlist        map[string]bool
	SiteType        string
}

const MAX_WORKERS = 50000 // max 50k go-routines per listener

var (
	pushAssets map[string][]string
	muxs       map[string]http.Handler
	langs      map[string]language.Matcher
	overrides  map[string]Override
)

func init() {
	pushAssets = make(map[string][]string)
	muxs = make(map[string]http.Handler)
	langs = make(map[string]language.Matcher)
	overrides = make(map[string]Override)
}

func push(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Host = strings.ToLower(r.Host)

		w.Header().Set("Vary", "Accept-Encoding")
		if assets, ok := pushAssets[r.Host]; r.URL.Path == "/" && ok {
			if pusher, ok := w.(http.Pusher); ok {
				for _, asset := range assets {
					if err := pusher.Push(asset, nil); err != nil {
						logger.Printf("Failed push: %v", err)
						break
					}
				}
			}
		}

		match, ok := langs[r.Host]
		if r.URL.Path == "/" && ok {
			// Multi-lang
			// TODO: err handle?
			t, _, _ := language.ParseAcceptLanguage(r.Header.Get("Accept-Language"))

			tag, _, _ := match.Match(t...)
			lang := tag.String()
			if strings.Contains(lang, "-") {
				lang = lang[0:strings.Index(lang, "-")]
			}

			target := "https://" + r.Host + "/" + lang + "/"
			http.Redirect(w, r, target, http.StatusFound)
			return
		}
		h.ServeHTTP(w, r)
	})
}

func stripPort(hostport string) string {
	host, _, err := net.SplitHostPort(hostport)
	if err != nil {
		return hostport
	}
	return net.JoinHostPort(host, "443")
}

type redirectHandler struct {
}

func (rh *redirectHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" && r.Method != "HEAD" {
		http.Error(w, "Use HTTPS", http.StatusBadRequest)
		return
	}

	host := strings.ToLower(r.Host)
	iswww := strings.HasPrefix(host, "www.")
	if iswww {
		host = host[len("www."):]
	}

	if _, ok := muxs[host]; !ok {
		logger.Printf("Unmatched host: %s", host)

		if mux, ok := muxs["default"]; ok {
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

func vhost() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Host = strings.ToLower(r.Host)
		if strings.HasPrefix(r.Host, "www.") {
			host := r.Host[len("www."):]
			target := "https://" + stripPort(host) + r.URL.RequestURI()
			http.Redirect(w, r, target, http.StatusMovedPermanently)
			return
		}
		m, ok := muxs[r.Host]
		if !ok || r.Host == "default" {
			logger.Printf("Unmatched host: %s", r.Host)
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
			if strings.ToLower(file.Name()) != file.Name() {
				return nil, fmt.Errorf("/var/www/" + file.Name() + " not lowercase!")
			}
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

func getOverride(path string) (Override, error) {
	c := Override{}

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

func listener(addr string) (net.Listener, error) {
	ln, e := net.Listen("tcp", addr)
	if e != nil {
		return nil, e
	}
	aln := TCPKeepAliveListener{ln.(*net.TCPListener)}
	return netutil.LimitListener(aln, MAX_WORKERS), nil
}

func main() {
	httpListen := ""
	flag.StringVar(&httpListen, "l", "", "HTTP iface:port (to override port 80 binding)")
	flag.Parse()
	domains, e := getDomains()
	if e != nil {
		panic(e)
	}

	f, err := os.OpenFile("/var/log/hfast.access.log", os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	SetLog(f)
	wwwDomains := []string{}
	siteTypes := map[string]bool{
		"": true,
		"amp": true,
		"weak": true,
	}

	for _, domain := range domains {
		override, e := getOverride(fmt.Sprintf("/var/www/%s/override.toml", domain))
		if e != nil {
			panic(e)
		}

		if domain == "default" {
			// Fallback domain
			fs := gziphandler.GzipHandler(FileServer(Dir(fmt.Sprintf("/var/www/%s/pub", domain))))
			mux := &http.ServeMux{}
			mux.Handle("/", fs)
			muxs[domain] = NotfoundWrapper(SecureWrapper(mux))
			overrides[domain] = override
			continue
		}
		if !siteTypes[override.SiteType] {
			panic(fmt.Errorf("overrides.SiteType invalid, given=%s", override.SiteType))
		}
		if !strings.HasPrefix(domain, "www.") {
			wwwDomains = append(wwwDomains, "www."+domain)
		}

		if len(override.Proxy) > 0 {
			fn, e := proxy.Proxy(override.Proxy)
			if e != nil {
				panic(e)
			}
			mux := &http.ServeMux{}
			if override.DevMode {
				mux.Handle("/", AccessLog(BasicAuth(fn, "Backend", override.Admin, override.Authlist)))
			} else {
				mux.Handle("/", AccessLog(fn))
			}
			overrides[domain] = override
			muxs[domain] = SecureWrapper(mux)
			continue
		}

		if len(override.Lang) > 0 {
			var tags []language.Tag
			for _, lang := range override.Lang {
				tags = append(tags, language.MustParse(lang))
			}
			langs[domain] = language.NewMatcher(tags)
		}

		fs := gziphandler.GzipHandler(push(FileServer(Dir(fmt.Sprintf("/var/www/%s/pub", domain)))))
		limit := ratelimit.Request(ratelimit.IP).Rate(30, time.Minute).LimitBy(memory.New()) // 30req/min

		mux := &http.ServeMux{}
		if len(override.Admin) > 0 {
			admin := gziphandler.GzipHandler(limit(BasicAuth(NewHandler(fmt.Sprintf("/var/www/%s/admin/index.php", domain), "tcp", "127.0.0.1:8000"), "Backend", override.Admin, override.Authlist)))
			mux.Handle("/admin/", AccessLog(admin))
		}
		if override.DevMode {
			action := gziphandler.GzipHandler(limit(BasicAuth(NewHandler(fmt.Sprintf("/var/www/%s/action/index.php", domain), "tcp", "127.0.0.1:8000"), "Backend", override.Admin, override.Authlist)))
			mux.Handle("/action/", AccessLog(action))
			mux.Handle("/", BasicAuth(AccessLog(fs), "Backend", override.Admin, override.Authlist))
		} else {
			action := gziphandler.GzipHandler(limit(NewHandler(fmt.Sprintf("/var/www/%s/action/index.php", domain), "tcp", "127.0.0.1:8000")))
			mux.Handle("/action/", AccessLog(action))
			mux.Handle("/", AccessLog(fs))
		}
		overrides[domain] = override
		muxs[domain] = SecureWrapper(mux)

		a, e := getPush(fmt.Sprintf("/var/www/%s/pub/push", domain))
		if e != nil {
			panic(e)
		}
		pushAssets[domain] = a
	}
	domains = append(domains, wwwDomains...)

	// Add www-prefix
	m := &autocert.Manager{
		Cache:      autocert.DirCache("/tmp/certs"),
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist(domains...),
	}

	s := &http.Server{
		Addr:         ":443",
		TLSConfig:    m.TLSConfig(),
		Handler:      RecoverWrap(vhost()),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  15 * time.Second,
		ErrorLog:     logger.Logger("@main.https-server: "),
	}

	go func() {
		s := &http.Server{
			Addr:         httpListen,
			Handler:      RecoverWrap(m.HTTPHandler(&redirectHandler{})),
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 10 * time.Second,
			IdleTimeout:  15 * time.Second,
			ErrorLog:     logger.Logger("@main.http-server: "),
		}
		ln, e := listener(s.Addr)
		if e != nil {
			logger.Fatal(e)
		}
		defer ln.Close()
		if e := s.Serve(ln); e != nil && e != http.ErrServerClosed {
			logger.Fatal(e)
		}
	}()

	sent, e := daemon.SdNotify(false, "READY=1")
	if e != nil {
		logger.Fatal(e)
	}
	if !sent {
		logger.Printf("SystemD notify NOT sent\n")
	}

	ln, e := listener(s.Addr)
	if e != nil {
		logger.Fatal(e)
	}
	defer ln.Close()
	if e := s.ServeTLS(ln, "", ""); e != nil && e != http.ErrServerClosed {
		logger.Fatal(e)
	}
}
