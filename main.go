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
	"github.com/quic-go/quic-go/http3"
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
	var http3Conn net.PacketConn
	{
		if !skipsysd {
			// Get all file descriptors from systemd (without clearing env)
			files := activation.Files(false)
			fmt.Printf("Systemd FDs=%d\n", len(files))

			for _, f := range files {
				// Try as TCP listener first
				if l, err := net.FileListener(f); err == nil {
					addr := l.Addr().String()
					if strings.HasSuffix(addr, ":80") {
						listeners["HTTP"] = limit(l)
						fmt.Printf("  HTTP=%s\n", addr)
					} else if strings.HasSuffix(addr, ":443") {
						listeners["HTTPS"] = limit(l)
						fmt.Printf("  HTTPS=%s\n", addr)
					} else {
						fmt.Printf("  Unknown TCP: %s\n", addr)
						l.Close()
					}
				} else if pc, err := net.FilePacketConn(f); err == nil {
					// It's a UDP socket
					http3Conn = pc
					fmt.Printf("  HTTP3=%s\n", pc.LocalAddr())
				}
				f.Close()
			}
		}
	}
	if len(listeners) == 0 {
		// No listeners from systemd, do it ourselves
		{
			l, e := listener(":443")
			if e != nil {
				panic(e)
			}
			listeners["HTTPS"] = l
		}
		{
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

	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
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
			defer func() {
				if e := queue.Close(); e != nil {
					fmt.Printf("queue.Close e=%s\n", e.Error())
				}
			}()
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
				// Activate pprof on admin backend with authentication
				mux.Handle("/debug/pprof/", handlers.BasicAuth(http.HandlerFunc(pprof.Index), "Backend", override.Admin, override.Authlist))
				mux.Handle("/debug/pprof/cmdline", handlers.BasicAuth(http.HandlerFunc(pprof.Cmdline), "Backend", override.Admin, override.Authlist))
				mux.Handle("/debug/pprof/profile", handlers.BasicAuth(http.HandlerFunc(pprof.Profile), "Backend", override.Admin, override.Authlist))
				mux.Handle("/debug/pprof/symbol", handlers.BasicAuth(http.HandlerFunc(pprof.Symbol), "Backend", override.Admin, override.Authlist))
				mux.Handle("/debug/pprof/trace", handlers.BasicAuth(http.HandlerFunc(pprof.Trace), "Backend", override.Admin, override.Authlist))
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

		action = handlers.AccessLog(action)
		base := handlers.AccessLog(fs)
		if override.DevMode {
			action = handlers.BasicAuth(action, "Backend", override.Admin, override.Authlist)
			base = handlers.BasicAuth(base, "Backend", override.Admin, override.Authlist)
		}

		mux.Handle(path, action)
		mux.Handle("/", base)

		config.Overrides[domain] = override
		config.Muxs[domain] = handlers.SecureWrapper(mux)
	}
	domains = append(domains, wwwDomains...)

	// Ensure secure certificate directory exists with restrictive permissions
	certDir := "/var/lib/hfast/certs"
	if err := os.MkdirAll(certDir, 0700); err != nil {
		panic(fmt.Sprintf("Failed to create cert directory %s: %s", certDir, err))
	}

	m := &autocert.Manager{
		Cache:      autocert.DirCache(certDir),
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist(domains...),
	}
	wg := new(sync.WaitGroup)
	var (
		httpServer  *http.Server
		httpsServer *http.Server
		http3Server *http3.Server
	)

	if useQueues {
		wg.Add(1)
		fn, e := queue.Serve(queue.WORKER_LISTEN)
		if e != nil {
			panic(fmt.Sprintf("queue.Serve e=%s\n", e.Error()))
		}

		go func() {
			if e := fn(); e != nil {
				fmt.Printf("queue.Serve e=%s\n", e.Error())
			}
			wg.Done()
		}()
	}

	// :80
	if _, ok := listeners["HTTP"]; ok {
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
	}

	// :443
	if _, ok := listeners["HTTPS"]; ok {
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
	}

	// HTTP/3 (QUIC) on UDP :443
	{
		wg.Add(1)
		go func() {
			s := &http3.Server{
				Addr:      ":443",
				TLSConfig: m.TLSConfig(),
				Handler:   RecoverWrap(handlers.Vhost()),
			}
			http3Server = s
			var e error
			if http3Conn != nil {
				// Use systemd-provided UDP socket
				e = s.Serve(http3Conn)
			} else {
				// Create our own UDP listener
				e = s.ListenAndServe()
			}
			if e != nil && e != http.ErrServerClosed {
				fmt.Printf("http3 server e=%s\n", e.Error())
			}
			wg.Done()
		}()
	}

	// Below routines that quit with our custom channel
	quit := make(chan os.Signal, 1)

	// Graceful shutdown
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-quit
		fmt.Println("server.shutdown")
		ctx, cancel := context.WithTimeout(context.Background(),
			6*time.Second)
		defer cancel()

		if httpServer != nil {
			httpServer.SetKeepAlivesEnabled(false)
			if e := httpServer.Shutdown(ctx); e != nil {
				panic(e)
			}
		}
		if httpsServer != nil {
			httpsServer.SetKeepAlivesEnabled(false)
			if e := httpsServer.Shutdown(ctx); e != nil {
				panic(e)
			}
		}
		if http3Server != nil {
			if e := http3Server.Close(); e != nil {
				fmt.Printf("http3Server.Close e=%s\n", e.Error())
			}
		}

		if useQueues {
			if e := queue.Listen.Close(); e != nil {
				fmt.Printf("queue.Listen.Close e=%s\n", e.Error())
			}
		}
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
