package main

import (
	"deltajournal/config"
	"deltajournal/journal"
	"flag"
	"fmt"
	"github.com/coreos/go-systemd/daemon"
	"io/ioutil"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

var (
	verbose   bool
	debug     bool
	readonly  bool
	host      string
	stateFile string
	cursor    string
)

func main() {
	var e error
	configPath := ""
	flag.BoolVar(&config.Verbose, "v", false, "Verbose-mode")
	flag.BoolVar(&config.Debug, "d", false, "Debug-mode")
	flag.BoolVar(&readonly, "r", false, "Readonly-mode")
	flag.StringVar(&configPath, "c", "./config.toml", "Path to config.json")
	flag.StringVar(&stateFile, "s", "/tmp/deltaj.pos", "State position file")
	flag.Parse()

	if e = config.Init(configPath); e != nil {
		panic(e)
	}

	host, e = os.Hostname()
	if e != nil {
		panic(e)
	}

	if _, e := os.Stat(stateFile); !os.IsNotExist(e) {
		b, e := ioutil.ReadFile(stateFile)
		if e != nil {
			panic(e)
		}
		cursor = string(b)
	}

	if config.Verbose {
		fmt.Printf("Cursor=%+v\n", cursor)
		fmt.Printf("Config=%+v\n", config.C)
	}

	if e := journal.Test(); e != nil {
		panic(e)
	}

	config.SigOS = make(chan os.Signal, 1)
	signal.Notify(config.SigOS, os.Interrupt)
	signal.Notify(config.SigOS, syscall.SIGTERM)
	ticker := time.Tick(config.C.TickerD)

	sent, e := daemon.SdNotify(false, "READY=1")
	if e != nil {
		panic(e)
	}
	if !sent {
		fmt.Printf("SystemD notify NOT sent\n")
	}

	// -------------------
	// Strict-mode from here
	// -------------------
	save := false
	cursor := ""
L:
	for {
		save = false
		buf, lastCursor, e := journal.ReadLines(cursor)
		if e == journal.ErrStopSignal {
			break L
		}
		if e != nil {
			fmt.Printf("ReadJournalLines e=%s\n", e.Error())
		}
		if len(buf) == 0 && lastCursor != "" {
			cursor = lastCursor
			save = true
		}

		if len(buf) > 0 {
			// We got something, MAIL IT
			if e := Email(config.C.Email, strings.Join(buf, "\n")); e != nil {
				fmt.Printf("Email failed e=%s\n", e.Error())
				continue
			}

			// Only move to new cursor if up until last cursor was mailed
			// else it will retry in the next run
			cursor = lastCursor
			save = true
		}

		if save {
			if config.Verbose {
				fmt.Printf("cursor.update(%s)=%s\n", stateFile, cursor)
			}
			if e := ioutil.WriteFile(stateFile, []byte(cursor), 0644); e != nil {
				fmt.Printf("WARN: stateFile.save e=%s\n", e.Error())
			}
		}

		if config.Verbose {
			fmt.Printf("EOF (await CTRL+C/5min)\n")
		}
		select {
		case _ = <-config.SigOS:
			// OS wants us dead
			break L
		case _ = <-ticker:
			// Next run, move back to start of for
		}
	}

	if config.Verbose {
		fmt.Printf("Stop\n")
	}
	if e := ioutil.WriteFile(stateFile, []byte(cursor), 0644); e != nil {
		fmt.Printf("WARN: stateFile.closeSave e=%s\n", e.Error())
	}
	os.Exit(0)
}
