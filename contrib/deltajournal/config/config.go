package config

import (
	"fmt"
	"github.com/BurntSushi/toml"
	"os"
	"time"
)

type Email struct {
	Host    string
	Port    int
	User    string
	Pass    string
	Display string
	From    string
	To      []string
	Subject string
}
type ServiceFilter struct {
	Severity int
	Filters  []string
}
type Config struct {
	Email    Email
	Services map[string]ServiceFilter
	Ticker   string
	TickerD  time.Duration
}

var (
	C Config
)

func Init(f string) error {
	r, e := os.Open(f)
	if e != nil {
		return e
	}
	defer r.Close()
	if _, e := toml.DecodeReader(r, &C); e != nil {
		return fmt.Errorf("TOML: %s", e)
	}
	d, e := time.ParseDuration(C.Ticker)
	C.TickerD = d
	return e
}
