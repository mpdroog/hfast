// Package proxy implements the oxy-library for HTTP-forwarding
package proxy

import (
	"context"
	"fmt"
	"github.com/mpdroog/hfast/logger"
	"github.com/vulcand/oxy/utils"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

// blockedProxyHeaders contains headers that should not be forwarded to prevent
// request smuggling, host header injection, and IP spoofing attacks
var blockedProxyHeaders = map[string]struct{}{
	// Hop-by-hop headers (RFC 2616)
	"Connection":        {},
	"Keep-Alive":        {},
	"Transfer-Encoding": {},
	"Te":                {},
	"Trailer":           {},
	"Upgrade":           {},
	// Host is set based on target
	"Host": {},
	// Proxy headers - set by HFast, don't allow client spoofing
	"X-Forwarded-For":   {},
	"X-Forwarded-Host":  {},
	"X-Forwarded-Proto": {},
	"X-Forwarded-Port":  {},
	"X-Real-Ip":         {},
	"X-Real-Port":       {},
	"Forwarded":         {}, // RFC 7239
	"X-Hfast":           {}, // Our own header
}

// copySafeHeaders copies all headers except blocked ones
func copySafeHeaders(dst, src http.Header) {
	for name, values := range src {
		if _, blocked := blockedProxyHeaders[name]; !blocked {
			for _, v := range values {
				dst.Add(name, v)
			}
		}
	}
}

func Proxy(to string) (http.HandlerFunc, error) {
	if !strings.HasPrefix(to, "http://") && !strings.HasPrefix(to, "https://") {
		return nil, fmt.Errorf("to(%s) does not begin with http:// nor https://", to)
	}

	var netTransport = &http.Transport{
		Dial: (&net.Dialer{
			Timeout: 5 * time.Second,
		}).Dial,
		TLSHandshakeTimeout: 5 * time.Second,
	}
	var netClient = &http.Client{
		Timeout:   time.Second * 10,
		Transport: netTransport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		defer req.Body.Close()
		ip, _, e := net.SplitHostPort(req.RemoteAddr)
		if e != nil {
			logger.Printf("net.SplitHostPort(%s) %s\n", req.RemoteAddr, e.Error())
			PrettyError(w)
			return
		}

		dest := to + req.URL.String()

		proxReq, e := http.NewRequest(req.Method, dest, req.Body)
		if e != nil {
			logger.Printf("newRequest(%s) %s\n", dest, e.Error())
			PrettyError(w)
			return
		}

		// Copy headers except hop-by-hop headers that could cause issues
		copySafeHeaders(proxReq.Header, req.Header)

		// Set proxy headers
		proxReq.Header.Set("X-HFast", "0.1.0")
		proxReq.Header.Set("X-Forwarded-For", ip)
		proxReq.Header.Set("X-Forwarded-Proto", "https")

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		defer cancel()
		proxReq = proxReq.WithContext(ctx)
		defer proxReq.Body.Close()

		res, e := netClient.Do(proxReq)
		if e != nil {
			// ignore timeouts when client went away
			clientErr := strings.Contains(e.Error(), "Client.Timeout exceeded while awaiting headers")
			if !clientErr {
				logger.Printf("netClient.Get(%s) %s\n", dest, e.Error())
			}
			PrettyError(w)
			return
		}
		defer res.Body.Close()

		utils.CopyHeaders(w.Header(), res.Header)
		w.WriteHeader(res.StatusCode)
		_, e = io.Copy(w, res.Body)
		if e != nil {
			logger.Printf("Failed writing buf to client. e=%s\n", e.Error())
		}
	}), nil
}
