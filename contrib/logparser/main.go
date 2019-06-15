/**
 * Read JSON logs and output stats
 */
package main

import (
	"encoding/json"
	"bufio"
    "flag"
    "log"
    "os"
)

type Msg struct {
	Method string
	Host string
	URL string
	Status int
	Remote string
	Ratelimit string
	Duration int64
	UA string
	Proto string
	Len uint64
	Date string
	Time string
	Referer string
}

type Output struct {
    Day map[string]uint64
    Referer map[string]uint64
}

func main() {
	path := ""
	host := ""
	flag.StringVar(&path, "p", "/var/log/hfast.access.log", "Path to hfast access log")
	flag.StringVar(&host, "h", "usenet.today", "Domain to scan for")
	flag.Parse()

    file, err := os.Open(path)
    if err != nil {
        log.Fatal(err)
    }
    defer file.Close()

    uniqReferer := make(map[string]uint64)
    uniqDay := make(map[string]uint64)

    msg := Msg{}
    scanner := bufio.NewScanner(file)
    for scanner.Scan() {
    	line := scanner.Bytes()
    	// TODO: Perf increase by scanning bytes for host?
    	if e := json.Unmarshal(line, &msg); e != nil {
    		log.Printf("Line(%s) e=%s\n", line, e)
            continue
    	}
    	if msg.Host != host {
    		continue
    	}

    	uniqDay[msg.Date]++
    	uniqReferer[msg.Referer]++
    }

    if err := scanner.Err(); err != nil {
        log.Fatal(err)
    }

    out := &Output{
        Day: uniqDay,
        Referer: uniqReferer,
    }
    enc := json.NewEncoder(os.Stdout)
    if e := enc.Encode(out); e != nil {
        log.Fatal(e)
    }
}