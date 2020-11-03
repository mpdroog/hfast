/**
 * TCP localhost:11300 for workers to connect to
 * Every minute PING/PONG
 * It's a server push protocol, the server will inform event-driven
 * the request.
 * > HELLO hfast.v1
 * < CHAN chan1 chan2 ...
 * > READY
 * > PING
 * < PONG
 * > JOB <id> <byte_len>
 * > ............
 * > ............
 * < DONE <id>
 */
package queue

import (
	"bufio"
	"fmt"
	"github.com/boltdb/bolt"
	"log"
	"net"
	"strings"
	"time"
)

type Msg struct {
	Id []byte
}

var queue map[string]chan Msg
var requeue map[string]chan Msg // re=retry

// Limit amount of connected workers
const WORKER_MAX = 100

// Default listener
const WORKER_LISTEN = "localhost:11300"

// WARN: This function is blocking
func Serve(listen string) error {
	ln, e := net.Listen("tcp", listen)
	if e != nil {
		return e
	}

	for {
		conn, e := ln.Accept()
		if e != nil {
			log.Printf("queue.Serve(%s) e=%s", listen, e.Error())
			continue
		}
		go handle(conn)
	}
}

func handle(conn net.Conn) {
	defer conn.Close()
	if _, e := conn.Write([]byte("HELLO hfast.v1\r\n")); e != nil {
		log.Printf("queue.handle(HELLO) e=%s", e.Error())
		return
	}

	lockDeadline := false
	timer := time.NewTicker(time.Minute)
	defer timer.Stop()

	var channel string
	buf := bufio.NewReader(conn)
	for {
		msg, e := buf.ReadString('\n')
		if e != nil {
			// Erro reading
			log.Printf("buf.ReadString e=%s", e.Error())
			break
		}
		if !lockDeadline {
			if e := conn.SetDeadline(time.Now().Add(2 * time.Minute)); e != nil {
				log.Printf("queue.SetDeadline e=%s", e.Error())
				break
			}
		}
		tok := strings.SplitN(msg, " ", 2) // Limit 2 tokens
		if tok[0] == "CHAN" {
			channel = tok[1]
			if _, e := conn.Write([]byte("READY\r\n")); e != nil {
				log.Printf("queue.handle(CHAN) e=%s", e.Error())
				break
			}
		} else if tok[0] == "PONG" {
			// TODO: Add arg to prevent ahead of time messups
		} else if tok[0] == "OK" {
			// Job is processed
			lockDeadline = false
		} else {
			if _, e := conn.Write([]byte("INVALID\r\n")); e != nil {
				log.Printf("queue.handle(CHAN) e=%s", e.Error())
			}
			break
		}

		select {
		case <-timer.C:
			if _, e := conn.Write([]byte("PING\r\n")); e != nil {
				log.Printf("queue.handle(PING) e=%s", e.Error())
				break
			}
		case msg := <-queue[channel]:
			// TODO: Memory friendly?
			var v []byte
			e := db.View(func(tx *bolt.Tx) error {
				b := tx.Bucket([]byte("queue"))
				bjob := b.Bucket([]byte(channel))
				v = bjob.Get([]byte(msg.Id))
				return nil
			})
			if e != nil {
				log.Printf("queue.handle(msg) e=%s", e.Error())
				break
			}

			if _, e := conn.Write([]byte(fmt.Sprintf("JOB %d %d\r\n", msg.Id, len(v)))); e != nil {
				log.Printf("queue.handle(CHAN) e=%s", e.Error())
				break
			}

			// Worker has 1-minute
			lockDeadline = true
			if e := conn.SetDeadline(time.Now().Add(time.Minute)); e != nil {
				log.Printf("queue.SetDeadline e=%s", e.Error())
				break
			}
		}
	}
}
