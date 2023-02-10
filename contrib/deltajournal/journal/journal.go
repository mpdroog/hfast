package journal

import (
	"deltajournal/config"
	"fmt"
	"github.com/coreos/go-systemd/sdjournal"
)

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
	if config.Verbose {
		fmt.Printf("Journal.SeekTail(prev=%s, cur=%s)\n", prevCursor, d.Cursor)
	}
	if d.Cursor == prevCursor {
		if config.Verbose {
			fmt.Printf("SeekTail still points to the prev cursor\n")
		}
		return "", nil
	}
	return d.Cursor, nil
}

// Test the journal once
func Test() error {
	j, e := sdjournal.NewJournal()
	if e != nil {
		return e
	}
	if e := j.Close(); e != nil {
		return e
	}
	return nil
}
