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
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
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
	if e := j.SeekTail(); e != nil {
		panic(e)
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
		var buf []string
		buflen := 0

		// Keep reading till end of log
		lastCursor := ""
		for {
			if verbose {
				fmt.Printf("Journal.NextEntry\n")
			}
			if stop() {
				break run
			}
			r, e := j.Previous()
			if e != nil {
				fmt.Printf("WARN: Journal.Previous e=%s\n", e.Error())
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
			if d.Cursor == cursor {
				// Done, we're at the position of last time
				if verbose {
					fmt.Printf("Reached cursor(%s) of last time.\n", cursor)
				}
				break
			}
			lastCursor = d.Cursor
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
				prio = severity
			}

			if prio < severity {
				if verbose {
					fmt.Printf("IGNORE [%s!%d>=%d] %s\n", unit, prio, severity, d.Fields["MESSAGE"])
				}
				continue
			}
			// Filter messages out
			skip := false
			msg := strings.TrimSpace(d.Fields["MESSAGE"])
			for _, filter := range filters {
				m, e := filepath.Match(filter, msg)
				if e != nil {
					fmt.Printf("WARN: filepath.Match=" + e.Error())
				}
				if verbose {
					fmt.Printf("Match(%s <=> %s) = %t\n", filter, msg, m)
				}
				if m {
					skip = true
					break
				}
			}
			if skip {
				if verbose {
					fmt.Printf("IGNORE [%s!%d>%d] %s\n", unit, prio, severity, msg)
				}
				continue
			}

			// https://www.freedesktop.org/software/systemd/man/systemd.journal-fields.html
			line := fmt.Sprintf("[%s!%d] %s", unit, prio, msg)
			buf = append([]string{line}, buf...)
			buflen += len(line)
			if buflen > 1024*1024 {
				// Exceed 1MB, just stop!
				break
			}
		}

		if len(buf) > 0 {
			// We got something, MAIL IT
			if e := Email(config.C.Email, strings.Join(buf, "\n")); e != nil {
				fmt.Printf("WARN: Email e=%s\n" + e.Error())
			} else {
				// Only move to new cursor if up until last cursor was mailed
				cursor = lastCursor
			}
		}

		if e := ioutil.WriteFile(stateFile, []byte(cursor), 0644); e != nil {
			fmt.Printf("WARN: stateFile.save e=%s\n", e.Error())
		}

		select {
		case _ = <-sigOS:
			// OS wants us dead
			break run
		case _ = <-ticker:
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
