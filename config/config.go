package config

import (
	"golang.org/x/text/language"
	"net/http"
)

type Override struct {
	Proxy           string // Reverse proxy to given http-address
	ExcludedDomains []string
	Lang            []string // Homepage auto-redirected languages
	Admin           map[string]string // Admin user+pass
	DevMode         bool              // Only allow admin user+pass
	Authlist        map[string]bool // IP Whitelist if devmode-on
	SiteType        string // Site framework
	Ratelimit       bool // Override (default on) ratelimiter on PHP-code

	SecretKey       string // Secret key used for hashing queue's (needed to have queueing enabled)
}

const MAX_WORKERS = 50000        // max 50k go-routines per listener
const PHP_FPM = "127.0.0.1:8000" // default FPM path

var (
	PushAssets map[string][]string
	Muxs       map[string]http.Handler
	Langs      map[string]language.Matcher
	Overrides  map[string]Override

	Verbose bool
	Webdir  string
)

func init() {
	PushAssets = make(map[string][]string)
	Muxs = make(map[string]http.Handler)
	Langs = make(map[string]language.Matcher)
	Overrides = make(map[string]Override)
}
