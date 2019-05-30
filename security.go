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

	featurePolicyHeader = "Feature-Policy"
	featurePolicyValue  = "vibrate 'none'; geolocation 'none'; camera 'none'; document-domain 'none'; microphone 'none'"
)

func sec(domains []string, useCSP bool) map[string]string {
	responseHeader := make(map[string]string)

	responseHeader[stsHeader] = fmt.Sprintf("max-age=%d%s", 315360000, stsPreloadString)
	responseHeader[frameOptionsHeader] = frameOptionsValue
	responseHeader[contentTypeHeader] = contentTypeValue
	responseHeader[xssProtectionHeader] = xssProtectionValue
	responseHeader[featurePolicyHeader] = featurePolicyValue
	if useCSP {
		responseHeader[cspHeader] = fmt.Sprintf("default-src 'self' %s", strings.Join(domains, " "))
	}
	return responseHeader
}

func SecureWrapper(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		override, ok := overrides[r.Host]
		if !ok {
			override = overrides["default"]
			// panic(fmt.Sprintf("DevErr: Host(%s) not configured", host))
		}

		useCSP := true
		if override.SiteType == "amp" || override.SiteType == "weak" {
			useCSP = false
		}

		for k, v := range sec(override.ExcludedDomains, useCSP) {
			w.Header().Add(k, v)
		}
		h.ServeHTTP(w, r)
	})
}
