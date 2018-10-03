// Package proxy implements the oxy-library for HTTP-forwarding
package proxy

import (
	"context"
	"github.com/vulcand/oxy/utils"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
)

func Proxy(to string) (http.HandlerFunc, error) {
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

		req.Header.Set("X-HFast", "0.1.0")
		//log.Printf("Host(%s) -> %s\n", req.Host, to+req.URL.String())

		dest := to + req.URL.String()
		proxReq, e := http.NewRequest(req.Method, dest, req.Body)
		if e != nil {
			log.Printf("newRequest(%s) %s\n", dest, e.Error())
			PrettyError(w)
			return
		}
		utils.CopyHeaders(proxReq.Header, req.Header)
		proxReq.Host = req.Host

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		defer cancel()
		proxReq = proxReq.WithContext(ctx)
		defer proxReq.Body.Close()

		res, e := netClient.Do(proxReq)
		if e != nil {
			// ignore timeouts when client went away
			clientErr := strings.Contains(e.Error(), "Client.Timeout exceeded while awaiting headers")
			if !clientErr {
				log.Printf("netClient.Get(%s) %s\n", dest, e.Error())
			}
			PrettyError(w)
			return
		}
		defer res.Body.Close()

		utils.CopyHeaders(w.Header(), res.Header)
		w.WriteHeader(res.StatusCode)
		_, e = io.Copy(w, res.Body)
		if e != nil {
			log.Printf("Failed writing buf to client. e=%s\n", e.Error())
		}
	}), nil
}
