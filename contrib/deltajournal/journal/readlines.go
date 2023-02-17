package journal

import (
	"deltajournal/config"
	"fmt"
	"github.com/coreos/go-systemd/sdjournal"
	"strconv"
	"strings"
)

var (
	ErrStopSignal = fmt.Errorf("ErrStopSignal")
)

func ReadLines(cursor string) ([]string, string, error) {
	j, e := sdjournal.NewJournal()
	if e != nil {
		return nil, "", e
	}
	defer j.Close()

	lastCursor, e := HasLog(j, cursor)
	if e != nil {
		return nil, "", e
	}
	if lastCursor == "" {
		// Nothing here
		return nil, "", nil
	}

	if e := j.SeekTail(); e != nil {
		return nil, "", e
	}

	var buf []string
	buflen := 0

	// Collect the lines (max 1024)
	for i := 0; i < 1024; i++ {
		if config.Verbose {
			fmt.Printf("Journal.PreviousEntry\n")
		}
		if config.Stop() {
			return buf, "", ErrStopSignal
		}
		r, e := j.Previous()
		if e != nil {
			return nil, "", e
		}
		if r == 0 {
			if config.Verbose {
				fmt.Printf("End-Of-Log\n")
			}
			break
		}
		d, e := j.GetEntry()
		if e != nil {
			return nil, "", e
		}
		if config.Verbose {
			fmt.Printf("Journal.Previous(seek=%s, cur=%s)\n", cursor, d.Cursor)
		}
		if d.Cursor == cursor {
			// Done, we're at the position of last time
			if config.Verbose {
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
			// Strip off @someid
			unit, _, _ = strings.Cut(unit, "@")
		}

		if override, ok := config.C.Services["default"]; ok {
			severity = override.Severity
			filters = override.Filters
		}
		hasCustom := false
		if override, ok := config.C.Services[unit]; ok {
			severity = override.Severity
			filters = override.Filters
			hasCustom = true
		}

		if !hasCustom {
			// Strip off -
			subunit, _, _ := strings.Cut(unit, "-")
			if override, ok := config.C.Services[subunit]; ok {
				unit = subunit
				severity = override.Severity
				filters = override.Filters
				hasCustom = true
			}
		}

		var prio int
		if d.Fields["PRIORITY"] == "" {
			// We have no prio so make it severe
			prio = severity
		} else {
			prio, e = strconv.Atoi(d.Fields["PRIORITY"])
			if e != nil {
				return nil, "", fmt.Errorf("WARN: strconv.Atoi(%s) e=%s\n", d.Fields["PRIORITY"], e.Error())
			}
		}

		if prio < severity {
			if config.Debug {
				fmt.Printf("IGNORE [%s!%d>=%d] %s\n", unit, prio, severity, d.Fields["MESSAGE"])
			}
			continue
		}
		// Filter messages out
		skip := false
		msg := strings.TrimSpace(d.Fields["MESSAGE"])
		for _, filter := range filters {
			if strings.Contains(msg, filter) {
				if config.Debug {
					fmt.Printf("Match(%s <=> %s)\n", filter, msg)
				}
				skip = true
				break
			}
		}
		if skip {
			if config.Debug {
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

	return buf, lastCursor, nil
}
