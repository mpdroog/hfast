package main

import (
	"deltajournal/config"
	"flag"
	"fmt"
	"github.com/coreos/go-systemd/daemon"
	"github.com/coreos/go-systemd/sdjournal"
	"io/ioutil"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
	"path/filepath"
)

var (
	verbose   bool
	readonly  bool
	host      string
	stateFile string
	cursor    string
	sigOS     chan os.Signal
)

func stop() bool {
	select {
	case _ = <-sigOS:
		return true
	default:
		return false
	}
}

func main() {
	var e error
	configPath := ""
	flag.BoolVar(&verbose, "v", false, "Verbose-mode")
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

	if verbose {
		fmt.Printf("Cursor=%+v\n", cursor)
		fmt.Printf("Config=%+v\n", config.C)
	}

	j, e := sdjournal.NewJournal()
	if e != nil {
		panic(e)
	}

	// Seek last position
	if cursor != "" {
		if e := j.SeekCursor(cursor); e != nil {
			panic(e)
		}
	}

	sigOS = make(chan os.Signal, 1)
	signal.Notify(sigOS, os.Interrupt)
	signal.Notify(sigOS, syscall.SIGTERM)
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
run:
	for {
		txt := ""

		// Keep reading till end of log
		lastCursor := ""
		for {
			if verbose {
				fmt.Printf("Journal.NextEntry\n")
			}
			if stop() {
				break run
			}
			r, e := j.Next()
			if e != nil {
				fmt.Printf("WARN: Journal.Next e=%s\n", e.Error())
				break
			}
			if r == 0 {
				if verbose {
					fmt.Printf("End-Of-Log\n")
				}
				break
			}

			d, e := j.GetEntry()
			if e != nil {
				fmt.Printf("WARN: Journal.GetEntry e=%s\n", e.Error())
				break
			}
			severity := 5
			filters := []string{}

			unit := d.Fields["_SYSTEMD_UNIT"]
			if len(unit) > 0 {
				// Strip off .service
				unit = unit[0:strings.Index(unit, ".")]
			}

			if override, ok := config.C.Services[unit]; ok {
				severity = override.Severity
				filters = override.Filters
			}

			prio, e := strconv.Atoi(d.Fields["PRIORITY"])
			if e != nil {
				fmt.Printf("WARN: strconv.Atoi(%s) e=%s\n", d.Fields["PRIORITY"], e.Error())
			}

			if prio > severity {
				if verbose {
					fmt.Printf("IGNORE [%s!%d>%d] %s\n", unit, prio, severity, d.Fields["MESSAGE"])
				}
				continue
			}
			// Filter messages out
			skip := false
			for _, filter := range filters {
				m, e := filepath.Match(filter, d.Fields["MESSAGE"])
				if e != nil {
					fmt.Printf("WARN: filepath.Match=" + e.Error())
				}
				if verbose {
					fmt.Printf("Match(%s <=> %s) = %d", filter, d.Fields["MESSAGE"], m)
				}
				if m {
					skip = true
					break
				}
			}
			if skip {
				if verbose {
					fmt.Printf("IGNORE [%s!%d>%d] %s\n", unit, prio, severity, d.Fields["MESSAGE"])
				}
				continue
			}


			lastCursor = d.Cursor
			// https://www.freedesktop.org/software/systemd/man/systemd.journal-fields.html
			txt += fmt.Sprintf("[%s!%s] %s\n", d.Fields["_SYSTEMD_UNIT"], d.Fields["PRIORITY"], d.Fields["MESSAGE"])
		}
		if txt != "" {
			// We got something, MAIL IT
			if e := Email(config.C.Email, txt); e != nil {
				fmt.Printf("WARN: Email e=%s\n" + e.Error())
			} else {
				// Only move to new cursor if up until last cursor was mailed
				cursor = lastCursor
			}
		}

		select {
		case _ = <-sigOS:
			// OS wants us dead
			break run
		case _ = <-ticker:
			if e := ioutil.WriteFile(stateFile, []byte(cursor), 0644); e != nil {
				fmt.Printf("WARN: stateFile.save e=%s\n", e.Error())
			}
			// Next run, move back to start of for
		}
	}

	if verbose {
		fmt.Printf("Stop\n")
	}
	if e := ioutil.WriteFile(stateFile, []byte(cursor), 0644); e != nil {
		fmt.Printf("WARN: stateFile.closeSave e=%s\n", e.Error())
	}
	os.Exit(0)

}
