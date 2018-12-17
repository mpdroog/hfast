package proxy

import (
	"github.com/mpdroog/hfast/logger"
	"net/http"
)

func PrettyError(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(500)

	if _, e := w.Write([]byte("500 - Failed forwarding.")); e != nil {
		logger.Printf("Failed writing err=", e.Error())
	}
}

type ErrorHandler struct {
}

func (e *ErrorHandler) ServeHTTP(w http.ResponseWriter, req *http.Request, err error) {
	PrettyError(w)
}
