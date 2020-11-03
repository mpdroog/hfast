package handlers

import (
	"net/http"
	"golang.org/x/text/language"
	"strings"
	"github.com/mpdroog/hfast/config"
	"github.com/mpdroog/hfast/logger"
)

func Push(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Host = strings.ToLower(r.Host)

		w.Header().Set("Vary", "Accept-Encoding")
		if assets, ok := config.PushAssets[r.Host]; r.URL.Path == "/" && ok {
			if pusher, ok := w.(http.Pusher); ok {
				for _, asset := range assets {
					if err := pusher.Push(asset, nil); err != nil {
						logger.Printf("Failed push: %v", err)
						break
					}
				}
			}
		}

		match, ok := config.Langs[r.Host]
		if r.URL.Path == "/" && ok {
			// Multi-lang
			// TODO: err handle?
			t, _, _ := language.ParseAcceptLanguage(r.Header.Get("Accept-Language"))

			tag, _, _ := match.Match(t...)
			lang := tag.String()
			if strings.Contains(lang, "-") {
				lang = lang[0:strings.Index(lang, "-")]
			}

			target := "https://" + r.Host + "/" + lang + "/"
			http.Redirect(w, r, target, http.StatusFound)
			return
		}
		h.ServeHTTP(w, r)
	})
}