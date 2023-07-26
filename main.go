package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/NYTimes/gziphandler"
	"github.com/coreos/go-systemd/activation"
	"github.com/coreos/go-systemd/daemon"
	"github.com/mpdroog/hfast/config"
	"github.com/mpdroog/hfast/handlers"
	"github.com/mpdroog/hfast/logger"
	"github.com/mpdroog/hfast/proxy"
	"github.com/mpdroog/hfast/queue"
	"github.com/mpdroog/ratelimit"
	"github.com/mpdroog/ratelimit/memory"
	"golang.org/x/crypto/acme/autocert"
	"golang.org/x/text/language"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

func main() {
	skipsysd := false
	logPath := ""

	//flag.BoolVar(&Test, "t", false, "Test-config and close")
	flag.BoolVar(&config.Verbose, "v", false, "Verbose-mode (log more)")
	flag.StringVar(&config.Webdir, "w", "/var/www", "Webroot")
	flag.BoolVar(&skipsysd, "s", false, "Disable systemd socket activation")
	flag.StringVar(&logPath, "l", "/var/log/hfast.access.log", "Logpath")
	flag.Parse()

	// Socket/self activation
	listeners := make(map[string]net.Listener)
	{
		haveHTTP := false
		haveHTTPS := false

		if !skipsysd {
			activated, e := activation.Listeners()
			if e != nil {
				panic(e)
			}
			for _, addr := range activated {
				if strings.HasSuffix(addr.Addr().String(), ":80") {
					haveHTTP = true
					listeners["HTTP"] = addr
				} else if strings.HasSuffix(addr.Addr().String(), ":443") {
					haveHTTPS = true
					listeners["HTTPS"] = addr
				} else {
					panic("Unsupported listener-addr=" + addr.Addr().String())
				}
			}
		}

		if !haveHTTP {
			l, e := listener(":443")
			if e != nil {
				panic(e)
			}
			listeners["HTTPS"] = l
		}
		if !haveHTTPS {
			l, e := listener(":80")
			if e != nil {
				panic(e)
			}
			listeners["HTTP"] = l
		}
	}

	domains, e := getDomains()
	if e != nil {
		panic(e)
	}

	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	handlers.SetLog(f)

	// Lookup tbl for translating www-domain to domain
	// (using lookup to ensure the domain is valid)
	wwwDomains := []string{}
	// Lookup valid site types
	siteTypes := map[string]bool{
		"":         true,
		"weak":     true,
		"indexphp": true,
	}
	useQueues := false

	for _, domain := range domains {
		fname := fmt.Sprintf(config.Webdir+"/%s/override.toml", domain)
		override, e := getOverride(fname)
		if e != nil {
			fmt.Printf("WARN_SKIP: Failed parsing(%s) e=%s\n", fname, e.Error())
			continue
		}
		if !siteTypes[override.SiteType] {
			panic(fmt.Errorf("overrides.SiteType invalid, given=%s", override.SiteType))
		}

		if domain == "default" {
			// Fallback domain
			fs := FileServer(Dir(fmt.Sprintf(config.Webdir+"/%s/pub", domain)))
			mux := &http.ServeMux{}
			mux.Handle("/", fs)
			config.Muxs[domain] = handlers.SecureWrapper(handlers.AccessLog(mux))
			config.Overrides[domain] = override
			continue
		}
		if !strings.HasPrefix(domain, "www.") {
			wwwDomains = append(wwwDomains, "www."+domain)
		}

		// Reverse Proxy-mode (passing data to next node)
		if len(override.Proxy) > 0 {
			fn, e := proxy.Proxy(override.Proxy)
			if e != nil {
				panic(e)
			}
			mux := &http.ServeMux{}
			// Devmode-enforces auth (IP or user+pass) protected domain
			if override.DevMode {
				mux.Handle("/", handlers.AccessLog(handlers.BasicAuth(fn, "Backend", override.Admin, override.Authlist)))
			} else {
				mux.Handle("/", handlers.AccessLog(fn))
			}
			config.Overrides[domain] = override
			config.Muxs[domain] = handlers.SecureWrapper(mux)
			continue
		}

		// Auto-redirect homepage request to /supported_lang/index
		if len(override.Lang) > 0 {
			var tags []language.Tag
			for _, lang := range override.Lang {
				tags = append(tags, language.MustParse(lang))
			}
			config.Langs[domain] = language.NewMatcher(tags)
		}

		// Serve pub-dir and add ratelimiter
		fs := FileServer(Dir(fmt.Sprintf(config.Webdir+"/%s/pub", domain)))
		limit := ratelimit.Request(ratelimit.IP).Rate(30, time.Minute).LimitBy(memory.NewLimited(1000)) // 30req/min

		mux := &http.ServeMux{}
		if len(override.SecretKey) > 0 {
			if e := queue.Init(); e != nil {
				panic(e)
			}
			useQueues = true
			mux.Handle("/queue/", handlers.AccessLog(queue.Handle()))
		}

		// Add /admin-path for mgmt
		if len(override.Admin) > 0 {
			admin := gziphandler.GzipHandler(NewHandler(fmt.Sprintf(config.Webdir+"/%s/admin/index.php", domain), "tcp", config.PHP_FPM))
			mux.Handle("/admin/", handlers.BasicAuth(handlers.AccessLog(admin), "Backend", override.Admin, override.Authlist))
		}

		if override.Pprof {
			if len(override.Admin) > 0 || len(override.Authlist) > 0 {
				// Activate pprof on admin backend
				mux.HandleFunc("/debug/pprof/", pprof.Index)
				mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
				mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
				mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
				mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
			} else {
				panic("Cannot enable pprof when admin-mode not enabled")
			}
		}

		// Base-path to make PHP active
		path := "/action/"
		if override.SiteType == "indexphp" {
			path = "/index.php"
		}

		php := NewHandler(fmt.Sprintf(config.Webdir+"/%s/action/index.php", domain), "tcp", config.PHP_FPM)
		action := gziphandler.GzipHandler(limit(php))
		if !override.Ratelimit {
			action = gziphandler.GzipHandler(php)
		}

		if override.DevMode {
			mux.Handle(path, handlers.BasicAuth(handlers.AccessLog(action), "Backend", override.Admin, override.Authlist))
			mux.Handle("/", handlers.BasicAuth(handlers.AccessLog(fs), "Backend", override.Admin, override.Authlist))
		} else {
			mux.Handle(path, handlers.AccessLog(action))
			mux.Handle("/", handlers.AccessLog(fs))
		}
		config.Overrides[domain] = override
		config.Muxs[domain] = handlers.SecureWrapper(mux)
	}
	domains = append(domains, wwwDomains...)

	// Add www-prefix
	m := &autocert.Manager{
		Cache:      autocert.DirCache("/tmp/certs"),
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist(domains...),
	}
	wg := new(sync.WaitGroup)
	var (
		httpServer  *http.Server
		httpsServer *http.Server
	)

	if useQueues {
		wg.Add(1)
		fn, e := queue.Serve(queue.WORKER_LISTEN)
		if e != nil {
			panic(fmt.Sprintf("queue.Serve e=%s\n", e.Error()))
		}

		go func() {
			if e := fn(); e != nil {
				fmt.Printf("queue.Serve e=" + e.Error())
			}
			wg.Done()
		}()
	}

	// :80
	wg.Add(1)
	go func() {
		s := &http.Server{
			Handler:      RecoverWrap(m.HTTPHandler(&handlers.RedirectHandler{})),
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 10 * time.Second,
			IdleTimeout:  15 * time.Second,
			ErrorLog:     logger.Logger("@main.http-server: "),
		}
		httpServer = s
		ln := listeners["HTTP"]
		defer ln.Close()
		if e := s.Serve(ln); e != nil && e != http.ErrServerClosed {
			logger.Fatal(e)
		}
		wg.Done()
	}()

	// :443
	wg.Add(1)
	go func() {
		s := &http.Server{
			TLSConfig:    m.TLSConfig(),
			Handler:      RecoverWrap(handlers.Vhost()),
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 10 * time.Second,
			IdleTimeout:  15 * time.Second,
			ErrorLog:     logger.Logger("@main.https-server: "),
		}
		httpsServer = s
		ln := listeners["HTTPS"]
		defer ln.Close()
		if e := s.ServeTLS(ln, "", ""); e != nil && e != http.ErrServerClosed {
			panic(e)
		}
		wg.Done()
	}()

	// Below routines that quit with our custom channel
	quit := make(chan os.Signal, 1)

	// watchdog
	go func() {
		if skipsysd {
			return
		}
		interval, e := daemon.SdWatchdogEnabled(false)
		if e != nil || interval == 0 {
			panic(e)
		}
		ticker := time.NewTicker(interval / 3)

		tr := &http.Transport{
			MaxIdleConns:    5,
			IdleConnTimeout: 10 * time.Second,
		}
		client := &http.Client{Transport: tr}
		addr := listeners["HTTP"].Addr().String()
		port := addr[strings.LastIndex(addr, ":"):]
		if config.Verbose {
			fmt.Printf("ticker interval=%d addr=%s\n", interval/3, "http://127.0.0.1"+port)
		}

		for {
			select {
			case <-quit:
				break
			case <-ticker.C:
				req, e := http.NewRequest("GET", "http://127.0.0.1"+port, nil)
				if e != nil {
					fmt.Printf("KeepAlive.err: %s\n", e.Error())
				}
				res, e := client.Do(req)
				if e != nil {
					fmt.Printf("KeepAlive.err: %s\n", e.Error())
				} else {
					res.Body.Close()
					if config.Verbose {
						fmt.Printf("watchdog.notify\n")
					}
					daemon.SdNotify(false, "WATCHDOG=1")
				}
			}
		}
	}()

	// Graceful shutdown
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-quit
		fmt.Println("server.shutdown")
		ctx, cancel := context.WithTimeout(context.Background(),
			6*time.Second)
		defer cancel()

		httpServer.SetKeepAlivesEnabled(false)
		httpsServer.SetKeepAlivesEnabled(false)
		if e := httpServer.Shutdown(ctx); e != nil {
			panic(e)
		}
		if e := httpsServer.Shutdown(ctx); e != nil {
			panic(e)
		}

		queue.Listen.Close()
	}()

	sent, e := daemon.SdNotify(false, "READY=1")
	if e != nil {
		logger.Fatal(e)
	}
	if !sent {
		logger.Printf("SystemD notify NOT sent\n")
	}

	wg.Wait()
}
