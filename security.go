// https://github.com/unrolled/secure/blob/v1/secure.go
package main

import (
	"fmt"
	"net/http"
	"strings"
)

const (
	stsHeader = "Strict-Transport-Security"
	//stsSubdomainString   = "; includeSubdomains"
	stsPreloadString    = "; preload"
	frameOptionsHeader  = "X-Frame-Options"
	frameOptionsValue   = "DENY"
	contentTypeHeader   = "X-Content-Type-Options"
	contentTypeValue    = "nosniff"
	xssProtectionHeader = "X-XSS-Protection"
	xssProtectionValue  = "1; mode=block"
	cspHeader           = "Content-Security-Policy"
)

func sec(domains []string) map[string]string {
	responseHeader := make(map[string]string)

	responseHeader[stsHeader] = fmt.Sprintf("max-age=%d%s", 315360000, stsPreloadString)
	responseHeader[frameOptionsHeader] = frameOptionsValue
	responseHeader[contentTypeHeader] = contentTypeValue
	responseHeader[xssProtectionHeader] = xssProtectionValue
	responseHeader[cspHeader] = fmt.Sprintf("default-src 'self' %s", strings.Join(domains, " "))

	return responseHeader
}

func SecureWrapper(h http.Handler, domains []string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for k, v := range sec(domains) {
			w.Header().Add(k, v)
		}
		h.ServeHTTP(w, r)
	})
}
