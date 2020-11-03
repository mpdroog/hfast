/**
 * Package queue implements a simple 'dynamic' queue, letting
 * external services write to you so you can process them async
 * and without any pressure to handle edge-cases like failure.
 *
 * - /queue/<name>/<MD5(secret+name)>(.ok)
 *   <name> is channel to put HTTP-req in
 *   (.ok) is optional and when set returns OK as body (if remote party needs this)
 *   <secret> is key in overrides.toml
 * - func health() returns health-status for queues
 *   <name> = {pending: X, first: <when it started processing>, last: <last time job was processed> lastOK: <last time job was sucessfully processed>, ok=N err=N}
 *
 * This package uses boltdb to save the entries in buckets.
 * /var/hfast.db > queue-bucket > name-bucket
 */
package queue

import (
	"crypto/md5"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/boltdb/bolt"
	"net/http"
	"strings"
	"sync"
)

var (
	db    *bolt.DB
	state *sync.Map
)

type Job struct {
	Id uint64
}

func Init() error {
	state = new(sync.Map)

	db, err := bolt.Open("/var/hfast.db", 0600, nil)
	if err != nil {
		return err
	}

	return db.View(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte("queue"))
		if err != nil {
			return fmt.Errorf("create bucket: %s", err)
		}

		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			fmt.Printf("key=%s, value=%s\n", k, v)
			if v != nil {
				return fmt.Errorf("bucket has values? val: %s", k)
			}

			cs := tx.Bucket(k).Cursor()
			for k, _ := cs.First(); k != nil; k, _ = cs.Next() {
				queue[string(v)] <- Msg{Id: k}
			}
		}

		// TODO: Fill state machines so we're in the state we were
		//  before restart.
		return nil
	})
}

func Req(w http.ResponseWriter, r *http.Request) {
	// URL=/v1/url.json
	toks := strings.SplitN(r.URL.Path, "/", 4)
	if len(toks) != 4 {
		w.WriteHeader(400)
		w.Write([]byte("Invalid path, example=/queue/<channel>/MD5(secret+name)(.ok)\n"))
		return
	}

	channel := toks[2]
	hashArg := strings.SplitN(toks[3], ".", 2)
	hash := hashArg[0]
	expect := "SILENT"

	if len(hashArg) > 1 {
		expect = strings.ToLower(hashArg[1])
		if expect != "ok" {
			w.WriteHeader(400)
			w.Write([]byte("Invalid return type, only .ok\n"))
			return
		}
	}

	/*
		r.Header().Set("X-Domain", domain)
		r.Header().Set("X-Secretkey", secretkey)
	*/
	domain := r.Header.Get("X-Domain")
	secret := r.Header.Get("X-Secretkey")

	if secret == "" {
		panic("Missing secret-key")
	}
	if hash != fmt.Sprintf("%x", md5.Sum([]byte(secret+channel))) {
		w.WriteHeader(403)
		w.Write([]byte("Invalid hash.\n"))
		return
	}

	e := db.Update(func(tx *bolt.Tx) error {
		b, e := tx.CreateBucketIfNotExists([]byte("queue"))
		if e != nil {
			return e
		}

		bucketName := domain + "_" + channel
		bchan, e := b.CreateBucketIfNotExists([]byte(bucketName))
		if e != nil {
			return e
		}

		id, e := bchan.NextSequence()
		if e != nil {
			return e
		}
		req, e := json.Marshal(r)
		if e != nil {
			return e
		}
		idb := []byte{}
		binary.LittleEndian.PutUint64(idb, id)
		if e := bchan.Put(idb, req); e != nil {
			return e
		}

		queue[bucketName] <- Msg{Id: idb}
		return nil
	})
	if e != nil {
		w.WriteHeader(500)
		w.Write([]byte("Failed storing msg.\n"))
		return
	}

	if expect == "ok" {
		w.Write([]byte("OK"))
	}
}
