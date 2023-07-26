package main

import (
	"fmt"
	"github.com/BurntSushi/toml"
	"github.com/mpdroog/hfast/config"
	"golang.org/x/net/netutil"
	"io/ioutil"
	"net"
	"os"
	"strings"
)

func getDomains() ([]string, error) {
	fileInfo, err := ioutil.ReadDir(config.Webdir)
	if err != nil {
		return nil, err
	}

	out := []string{}
	for _, file := range fileInfo {
		if file.IsDir() {
			if strings.ToLower(file.Name()) != file.Name() {
				return nil, fmt.Errorf(config.Webdir + file.Name() + " not lowercase!")
			}
			out = append(out, file.Name())
		}
	}
	return out, nil
}

func exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return true, err
}

func getOverride(path string) (config.Override, error) {
	c := config.Override{}

	if _, e := os.Stat(path); os.IsNotExist(e) {
		return c, nil
	}

	r, e := os.Open(path)
	if e != nil {
		return c, e
	}
	defer r.Close()
	_, e = toml.DecodeReader(r, &c)
	return c, e
}

func listener(addr string) (net.Listener, error) {
	ln, e := net.Listen("tcp", addr)
	if e != nil {
		return nil, e
	}
	aln := TCPKeepAliveListener{ln.(*net.TCPListener)}
	return netutil.LimitListener(aln, config.MAX_WORKERS), nil
}
