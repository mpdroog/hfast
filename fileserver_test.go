package main

import (
	"crypto/rand"
	"fmt"
	"io/ioutil"
	"log"
	mbig "math/big"
	"net/http"
	"testing"
	"time"
)

var (
	client http.Client
	server *http.Server
)

func init() {
	fs := FileServer(Dir("./testdata"))
	http.Handle("/", fs)

	d, err := rand.Int(rand.Reader, mbig.NewInt(0xFFFF))
	if err != nil {
		log.Fatal(err)
	}

	server = &http.Server{Addr: fmt.Sprintf("127.0.0.1:%d", d), Handler: http.DefaultServeMux}
	//defer server.Close()
	go func() {
		e := server.ListenAndServe()
		if e != nil {
			panic(e)
		}
	}()

	client = http.Client{
		Timeout: time.Second * 2, // Maximum of 2 secs
	}
}

func call(path string) (*http.Response, error) {
	url := "http://" + server.Addr + path
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	//req.Header.Set("User-Agent", "spacecount-tutorial")

	return client.Do(req)
}

func TestIndex(t *testing.T) {
	res, e := call("/")
	if e != nil {
		log.Fatal(e)
	}

	if res.StatusCode != http.StatusForbidden {
		log.Printf("Err, wrong HTTP-code")
	}
	body, e := ioutil.ReadAll(res.Body)
	if e != nil {
		log.Fatal(e)
	}
	if string(body) != "Action prohibited.\n" {
		log.Printf("Err, prohibit-msg not shows, body=" + string(body))
	}
}

func TestDotPrefixed(t *testing.T) {
	res, e := call("/.htaccess")
	if e != nil {
		log.Fatal(e)
	}

	if res.StatusCode != http.StatusForbidden {
		log.Printf("Err, wrong HTTP-code")
	}
	body, e := ioutil.ReadAll(res.Body)
	if e != nil {
		log.Fatal(e)
	}
	if string(body) != "Action prohibited.\n" {
		log.Printf("Err, prohibit-msg not shows, body=" + string(body))
	}
}

func TestRegular(t *testing.T) {
	res, e := call("/file.txt")
	if e != nil {
		log.Fatal(e)
	}

	if res.StatusCode != http.StatusOK {
		log.Printf("Err, wrong HTTP-code")
	}
	body, e := ioutil.ReadAll(res.Body)
	if e != nil {
		log.Fatal(e)
	}
	if string(body) != "Works!" {
		log.Printf("Err, file not returned, body=" + string(body))
	}
}

func TestDotDir(t *testing.T) {
	res, e := call("/.repo/unsafe.txt")
	if e != nil {
		log.Fatal(e)
	}

	if res.StatusCode != http.StatusForbidden {
		log.Printf("Err, wrong HTTP-code")
	}
	body, e := ioutil.ReadAll(res.Body)
	if e != nil {
		log.Fatal(e)
	}
	if string(body) != "Action prohibited.\n" {
		log.Printf("Err, prohibit-msg not shows, body=" + string(body))
	}
}

func TestDir(t *testing.T) {
	res, e := call("/safe/file.txt")
	if e != nil {
		log.Fatal(e)
	}

	if res.StatusCode != http.StatusOK {
		log.Printf("Err, wrong HTTP-code")
	}
	body, e := ioutil.ReadAll(res.Body)
	if e != nil {
		log.Fatal(e)
	}
	if string(body) != "Works!" {
		log.Printf("Err, file not returned, body=" + string(body))
	}
}

func TestPHP(t *testing.T) {
	res, e := call("/script.php")
	if e != nil {
		log.Fatal(e)
	}

	if res.StatusCode != http.StatusForbidden {
		log.Printf("Err, wrong HTTP-code")
	}
	body, e := ioutil.ReadAll(res.Body)
	if e != nil {
		log.Fatal(e)
	}
	if string(body) != "Action prohibited.\n" {
		log.Printf("Err, prohibit-msg not shows, body=" + string(body))
	}
}

func TestRevisioned(t *testing.T) {
	m := map[string]string{
		"/test.js": "/test.js",
		"/test.1001.css": "/test.1001.css",
		"/test.v1001.css": "/test.css",
	}

	for in, out := range m {
		got := revisioned(in)
		if out != got {
			log.Printf("Mismatch %s!=%s, got=%s\n", in, out, got)
		}
	}
}
