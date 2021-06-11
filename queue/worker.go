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
	"encoding/binary"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/mpdroog/hfast/config"
	"log"
	"net"
	"strings"
	"time"
)

type Msg struct {
	Id []byte
}

var Listen net.Listener
var queue map[string]chan Msg
var requeue map[string]chan Msg // re=retry

// Limit amount of connected workers
const WORKER_MAX = 100

// Default listener
const WORKER_LISTEN = "localhost:11300"

func init() {
	queue = make(map[string]chan Msg)
	requeue = make(map[string]chan Msg)
}

func Serve(listen string) (func() error, error) {
	ln, e := net.Listen("tcp", listen)
	if e != nil {
		return nil, e
	}
	Listen = ln

	return func() error {
		for {
			conn, e := Listen.Accept()
			if e != nil {
				log.Printf("queue.Serve(%s) e=%s", listen, e.Error())
				continue
			}
			go handle(conn)
		}
	}, nil
}

func handle(conn net.Conn) {
	defer conn.Close()
	if _, e := conn.Write([]byte("HELLO hfast.v1\r\n")); e != nil {
		log.Printf("queue.handle(HELLO) e=%s", e.Error())
		return
	}

	state := 0
	var channel string
	var curmsg *Msg
	buf := bufio.NewReader(conn)
	for {
		msg, e := buf.ReadString('\n')
		msg = strings.TrimSpace(msg)
		if e != nil {
			// Error reading
			log.Printf("buf.ReadString e=%s", e.Error())
			break
		}
		tok := strings.SplitN(msg, " ", 2) // Limit 2 tokens
		if config.Verbose {
			fmt.Printf("queue.handle(state=%d) msg=%+v\n", state, tok)
		}

		if state == 0 {
			if tok[0] == "CHAN" {
				channel = tok[1]
				if _, e := conn.Write([]byte("READY\r\n")); e != nil {
					log.Printf("queue.handle(CHAN) e=%s", e.Error())
					break
				}
				state = 1
			} else {
				if _, e := conn.Write([]byte("INVALID\r\n")); e != nil {
					log.Printf("queue.handle(CHAN) e=%s", e.Error())
				}
				break
			}
		} else if state == 1 {
			if tok[0] == "READY" {
				var msg Msg
				select {
				case msg = <-requeue[channel]:
				case msg = <-queue[channel]:
				}
				if config.Verbose {
					fmt.Printf("queue.handle(READY) msg=%+v\n", msg)
				}
				curmsg = &msg
				state = 2

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

				id := int64(binary.LittleEndian.Uint64(msg.Id))
				if _, e := conn.Write([]byte(fmt.Sprintf("JOB %d %d\r\n", id, len(v)))); e != nil {
					log.Printf("queue.handle(CHAN) e=%s", e.Error())
					break
				}
				if _, e := conn.Write(v); e != nil {
					log.Printf("queue.handle(CHAN) e=%s", e.Error())
					break
				}
				// Worker has 1-minute
				if e := conn.SetDeadline(time.Now().Add(time.Minute)); e != nil {
					log.Printf("queue.SetDeadline e=%s", e.Error())
					break
				}
			} else {
				// Invalid cmd
				if _, e := conn.Write([]byte("INVALID\r\n")); e != nil {
					log.Printf("queue.handle(CHAN) e=%s", e.Error())
				}
				break
			}
		} else {
			if tok[0] == "PONG" {
				// TODO: Add arg to prevent ahead of time messups
				// Give 2 more minutes for processing
				if e := conn.SetDeadline(time.Now().Add(2 * time.Minute)); e != nil {
					log.Printf("queue.SetDeadline e=%s", e.Error())
					break
				}

			} else if tok[0] == "OK" {
				// Job is processed, nothing to do
				curmsg = nil
				state = 1
			} else {
				// Invalid cmd
				if _, e := conn.Write([]byte("INVALID\r\n")); e != nil {
					log.Printf("queue.handle(CHAN) e=%s", e.Error())
				}
				break
			}
		}
	}

	if state == 2 {
		// processing failed
		requeue[channel] <- *curmsg
	}
}
