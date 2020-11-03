package config

import (
	"net/http"
	"golang.org/x/text/language"
)

type Override struct {
	Proxy           string // Just forward to given addr
	ExcludedDomains []string
	Lang            []string
	Admin           map[string]string // Admin user+pass
	DevMode         bool              // Only allow admin user+pass
	Authlist        map[string]bool
	SiteType        string
	Ratelimit       bool
}

const MAX_WORKERS = 50000 // max 50k go-routines per listener
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
