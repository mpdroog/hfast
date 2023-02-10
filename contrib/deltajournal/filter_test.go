package main

import (
	"strings"
	"testing"
)

func TestFilter(t *testing.T) {
	msg := strings.TrimSpace("@github.com/mpdroog/hfast/handlers.(*RedirectHandler).ServeHTTP: Unmatched host: 178.20.173.184")
	filter := "Unmatched host: "

	if !strings.Contains(msg, filter) {
		t.Errorf("Failed matching filter")
	}
}
