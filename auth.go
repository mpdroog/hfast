package main

import (
    "net/http"
    "crypto/subtle"
)

func BasicAuth(h http.Handler, realm string, userpass map[string]string) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        user, pass, ok := r.BasicAuth()

        for username, password := range userpass {
            if !ok || subtle.ConstantTimeCompare([]byte(user), []byte(username)) != 1 || subtle.ConstantTimeCompare([]byte(pass), []byte(password)) != 1 {
                w.Header().Set("WWW-Authenticate", `Basic realm="`+realm+`"`)
                w.WriteHeader(401)
                w.Write([]byte("Unauthorised.\n"))
                return
            }
        }

        h.ServeHTTP(w, r)
    })
}