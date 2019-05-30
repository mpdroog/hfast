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
	cspAMP              = "default-src * data: blob:; script-src blob: https://cdn.ampproject.org/v0.js https://cdn.ampproject.org/v0/ https://cdn.ampproject.org/viewer/ https://cdn.ampproject.org/rtv/; object-src 'none'; style-src 'unsafe-inline' https://cdn.ampproject.org/rtv/ https://cdn.materialdesignicons.com https://cloud.typography.com https://fast.fonts.net https://fonts.googleapis.com https://maxcdn.bootstrapcdn.com https://p.typekit.net https://use.fontawesome.com https://use.typekit.net"

	featurePolicyHeader = "Feature-Policy"
	featurePolicyValue  = "vibrate 'none'; geolocation 'none'; camera 'none'; document-domain 'none'; microphone 'none'"
)

func sec(domains []string, appMode string) map[string]string {
	responseHeader := make(map[string]string)

	responseHeader[stsHeader] = fmt.Sprintf("max-age=%d%s", 315360000, stsPreloadString)
	responseHeader[frameOptionsHeader] = frameOptionsValue
	responseHeader[contentTypeHeader] = contentTypeValue
	responseHeader[xssProtectionHeader] = xssProtectionValue
	responseHeader[featurePolicyHeader] = featurePolicyValue
	if appMode == "" {
		responseHeader[cspHeader] = fmt.Sprintf("default-src 'self' %s", strings.Join(domains, " "))
	} else if appMode == "amp" {
		responseHeader[cspHeader] = cspAMP
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

		for k, v := range sec(override.ExcludedDomains, override.SiteType) {
			w.Header().Add(k, v)
		}
		h.ServeHTTP(w, r)
	})
}
