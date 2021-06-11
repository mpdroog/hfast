package queue

import (
	"bufio"
	"crypto/md5"
	"fmt"
	"github.com/mpdroog/hfast/config"
	"io"
	"io/ioutil"
	"net"
	"net/http/httptest"
	"net/textproto"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestWork(t *testing.T) {
	if testing.Verbose() {
		config.Verbose = true
	}

	// Init test
	dbPath = "/tmp/test.db"
	if _, err := os.Stat(dbPath); err == nil {
		if err := os.Remove(dbPath); err != nil {
			t.Fatal(err)
		}
	}

	// Spawn instances
	if e := Init(); e != nil {
		t.Fatal(e)
	}
	fnListen, e := Serve(WORKER_LISTEN)
	if e != nil {
		t.Fatal(e)
	}

	go fnListen()
	defer Listen.Close()

	// Write task to queue
	// /queue/<channel>/MD5(<secretkey>+<channel>)
	secret := []byte("prjkey" + "test")
	hash := fmt.Sprintf("%x", md5.Sum(secret))
	req := httptest.NewRequest("POST", "/queue/test/"+hash, nil)
	req.Header.Set("X-Domain", "test.com")
	req.Header.Set("X-Secretkey", "prjkey")
	res := httptest.NewRecorder()

	Handle().ServeHTTP(res, req)

	b, e := ioutil.ReadAll(res.Result().Body)
	if e != nil {
		t.Fatal(e)
	}
	if res.Result().StatusCode != 200 {
		t.Fatal(fmt.Errorf("HTTP not 200 but %d res=%s", res.Result().StatusCode, string(b)))
	}
	if res.Result().StatusCode == 200 && string(b) != "" {
		t.Fatal(fmt.Errorf("HTTP 200 should return empty body, res=%s", string(b)))
	}

	// Now process in worker
	from, to := net.Pipe()
	go handle(to)

	reader := bufio.NewReader(from)
	tp := textproto.NewReader(reader)

	{
		line, e := tp.ReadLine()
		if e != nil {
			t.Fatal(e)
		}
		fmt.Println("<< " + line)
		if line != "HELLO hfast.v1" {
			t.Fatal(e)
		}
	}

	{
		// domain + "_" + channel
		fmt.Println(">> CHAN test.com_test")
		_, e = from.Write([]byte("CHAN test.com_test\r\n"))
		if e != nil {
			t.Fatal(e)
		}

		line, e := tp.ReadLine()
		if e != nil {
			t.Fatal(e)
		}
		fmt.Println("<< " + line)
		if line != "READY" {
			t.Fatal(e)
		}
	}

	bodyLen := 0
	{
		fmt.Println(">> READY")
		_, e = from.Write([]byte("READY\r\n"))
		if e != nil {
			t.Fatal(e)
		}

		line, e := tp.ReadLine()
		if e != nil {
			t.Fatal(e)
		}
		fmt.Println("<< " + line)
		if !strings.HasPrefix(line, "JOB ") {
			t.Fatal(e)
		}

		strlen := strings.Split(line, " ")[2]
		bodyLen, e = strconv.Atoi(strlen)
		if e != nil {
			t.Fatal(e)
		}
	}

	p := make([]byte, bodyLen)
	{
		n, e := io.ReadFull(reader, p)
		if e != nil {
			t.Fatal(e)
		}
		if n != bodyLen {
			t.Fatal(fmt.Errorf("bodylen mismatch"))
		}
	}

	fmt.Printf("body=%s\n", string(p))
}
