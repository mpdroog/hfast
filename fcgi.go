package main

import (
	"github.com/yookoala/gofast"
	"net/http"
)

func hfastMap(inner gofast.SessionHandler) gofast.SessionHandler {
	return func(client gofast.Client, req *gofast.Request) (*gofast.ResponsePipe, error) {
		r := req.Raw
		req.Params["SERVER_NAME"] = r.Host
		return inner(client, req)
	}
}

func newFileEndpoint(endpointFile string) gofast.Middleware {
	return gofast.Chain(
		gofast.BasicParamsMap,
		gofast.MapHeader,
		gofast.MapEndpoint(endpointFile),
		hfastMap,
	)
}

// NewHandler returns a fastcgi web server implementation as an http.Handler
// Please note that this handler doesn't handle the fastcgi application process.
// You'd need to start it with other means.
//
// docroot: the document root of the PHP site.
// network: network protocol (tcp / tcp4 / tcp6)
//          or if it is a unix socket, "unix"
// address: IP address and port, or the socket physical address of the fastcgi
//          application.
func NewHandler(docroot, network, address string) http.Handler {
	connFactory := gofast.SimpleConnFactory("tcp", address)

	// route all requests to a single php file
	return gofast.NewHandler(
		newFileEndpoint(docroot)(gofast.BasicSession),
		gofast.SimpleClientFactory(connFactory, 0),
	)
}
