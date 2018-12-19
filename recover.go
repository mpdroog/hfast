package main

import (
	"net/http"
	"errors"
	"github.com/mpdroog/hfast/logger"
)

func RecoverWrap(h http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        var err error
        defer func() {
            r := recover()
            if r != nil {
                switch t := r.(type) {
                case string:
                    err = errors.New(t)
                case error:
                    err = t
                default:
                    err = errors.New("Unknown error")
                }
                logger.Printf(err.Error())
                http.Error(w, "Error, reported!", http.StatusInternalServerError)
            }
        }()
        h.ServeHTTP(w, r)
    })
}
