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

func HasLog(j *sdjournal.Journal, prevCursor string) (string, error) {
	if e := j.SeekTail(); e != nil {
		return "", e
	}

	r, e := j.Previous()
	if e != nil {
		return "", e
	}
	if r == 0 {
		panic("DevErr: Unexpected?")
	}
	d, e := j.GetEntry()
	if e != nil {
		return "", e
	}
	if verbose {
		fmt.Printf("Journal.SeekTail(prev=%s, cur=%s)\n", prevCursor, d.Cursor)
	}
	if d.Cursor == prevCursor {
		if verbose {
			fmt.Printf("SeekTail still points to the prev cursor\n")
		}
		return "", nil
	}
	return d.Cursor, nil
}

func main() {
	var e error
	configPath := ""
	flag.BoolVar(&verbose, "v", false, "Verbose-mode")
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

	if verbose {
		fmt.Printf("Cursor=%+v\n", cursor)
		fmt.Printf("Config=%+v\n", config.C)
	}

	j, e := sdjournal.NewJournal()
	if e != nil {
		panic(e)
	}
	defer j.Close()

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

		lastCursor, e := HasLog(j, cursor)
		if e != nil {
			fmt.Printf("Journal.HasLog(%s)\n", e.Error())
		}

		// Reverse read till cursor of last run
		if lastCursor != "" {
			if e := j.SeekTail(); e != nil {
				fmt.Printf("WARN: Journal.SeekTail e=%s\n", e.Error())
				continue
			}

			for {
				if verbose {
					fmt.Printf("Journal.PreviousEntry\n")
				}
				if stop() {
					break run
				}
				r, e := j.Previous()
				if e != nil {
					fmt.Printf("Journal.Previous(%s)\n", e.Error())
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
				if verbose {
					fmt.Printf("Journal.Previous(seek=%s, cur=%s)\n", cursor, d.Cursor)
				}
				if d.Cursor == cursor {
					// Done, we're at the position of last time
					if verbose {
						fmt.Printf("Reached cursor(%s) of last time.\n", cursor)
					}
					break
				}
				severity := 5
				filters := []string{}

				unit := d.Fields["_SYSTEMD_UNIT"]
				if len(unit) > 0 {
					// Strip off .service
					unit = unit[0:strings.Index(unit, ".")]
				}

				if override, ok := config.C.Services["default"]; ok {
					severity = override.Severity
					filters = override.Filters
				}
				if override, ok := config.C.Services[unit]; ok {
					severity = override.Severity
					filters = override.Filters
				}

				var prio int
				if d.Fields["PRIORITY"] == "" {
					// We have no prio so make it severe
					prio = severity
				} else {
					prio, e = strconv.Atoi(d.Fields["PRIORITY"])
					if e != nil {
						fmt.Printf("WARN: strconv.Atoi(%s) e=%s\n", d.Fields["PRIORITY"], e.Error())
					}
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
						fmt.Printf("WARN: filepath.Match=%s\n", e.Error())
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

				if buflen > 512*1024 {
					// Exceed 512KB, just stop!
					break
				}
			}

			if len(buf) == 0 {
				// Update cursor if we have nothing to mail
				cursor = lastCursor
			} else {
				// We got something, MAIL IT
				if readonly {
					fmt.Println(strings.Join(buf, "\n"))
					cursor = lastCursor
				} else {
					if e := Email(config.C.Email, strings.Join(buf, "\n")); e != nil {
						fmt.Printf("Email failed e=%s\n", e.Error())
					} else {
						// Only move to new cursor if up until last cursor was mailed
						// else it will retry in the next run
						cursor = lastCursor
					}
				}
			}

			if verbose {
				fmt.Printf("cursor.update(%s)=%s\n", stateFile, cursor)
			}
			if e := ioutil.WriteFile(stateFile, []byte(cursor), 0644); e != nil {
				fmt.Printf("WARN: stateFile.save e=%s\n", e.Error())
			}
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
