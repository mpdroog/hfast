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
    "net"
    "strings"
    "time"
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
    Visitors map[string]map[string]uint64
    StatusCodes map[string]map[int]int
    Referer map[string]uint64
    Warnings map[string]Msg
    LastUpdate string
}

func FormatMsg(m Msg) Msg {
    if strings.HasPrefix(m.Referer, "https://www.paypal.com/webapps/hermes") {
        m.Referer = "https://www.paypal.com/webapps/hermes"
    }
    if strings.HasPrefix(m.Referer, "https://betalen.rabobank.nl/ideal-betaling/") {
        m.Referer = "https://betalen.rabobank.nl/ideal-betaling/"
    }
    if strings.HasPrefix(m.Referer, "https://usenet.today") {
        m.Referer = ""
    }
    return m
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
    uniqDay := make(map[string]map[string]uint64)
    dayStatusCodes := make(map[string]map[int]int)
    warnings := make(map[string]Msg)

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
        if msg.Status < 200 || msg.Status > 304 {
            warnings[msg.URL] = msg
        }
        msg = FormatMsg(msg)

        host, _, e := net.SplitHostPort(msg.Remote)
        if e != nil {
            log.Fatal(e)
        }
        if _, ok := uniqDay[msg.Date]; !ok {
            uniqDay[msg.Date] = make(map[string]uint64)
        }
        if _, ok := dayStatusCodes[msg.Date]; !ok {
            dayStatusCodes[msg.Date] = make(map[int]int)
        }
    	uniqDay[msg.Date][host]++
        dayStatusCodes[msg.Date][msg.Status]++
    	uniqReferer[msg.Referer]++
    }

    if err := scanner.Err(); err != nil {
        log.Fatal(err)
    }

    out := &Output{
        Visitors: uniqDay,
        StatusCodes: dayStatusCodes,
        Referer: uniqReferer,
        Warnings: warnings,
        LastUpdate: time.Now().Format("2006-01-02 15:04:05"),
    }
    enc := json.NewEncoder(os.Stdout)
    if e := enc.Encode(out); e != nil {
        log.Fatal(e)
    }
}