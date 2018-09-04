package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"github.com/coreos/go-systemd/daemon"
	"github.com/otiai10/copy"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

var Verbose bool

type Job struct {
	User string
	Pass string
}

func replace(file string, j Job) error {
	input, err := ioutil.ReadFile(file)
	if err != nil {
		return err
	}

	lines := strings.Split(string(input), "\n")
	for i, line := range lines {
		line = strings.Replace(line, "{USER}", j.User, -1)
                line = strings.Replace(line, "{user}", j.User, -1)
                line = strings.Replace(line, "{pass}", j.Pass, -1)
		lines[i] = strings.Replace(line, "{PASS}", j.Pass, -1)
	}
	output := strings.Join(lines, "\n")
	err = ioutil.WriteFile(file, []byte(output), 0644)
	return err
}

func build(w http.ResponseWriter, r *http.Request) {
	var j Job
	if r.Body == nil {
		http.Error(w, "Please send a request body", 400)
		return
	}
	err := json.NewDecoder(r.Body).Decode(&j)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	defaultConfig := "/var/www/usenet.today/contrib/clientconf"
	tmpDir, err := ioutil.TempDir("", "clientconf")
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if Verbose {
			log.Println("Delete tmp=" + tmpDir)
		}
		if err := os.RemoveAll(tmpDir); err != nil {
			log.Fatal(err)
		}
	}()

	if Verbose {
		log.Println("Copy " + defaultConfig + " to " + tmpDir)
	}
	if err := copy.Copy(defaultConfig, tmpDir); err != nil {
		log.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		log.Fatal(err)
	}

	if err := replace("main.nsis", j); err != nil {
		log.Fatal(err)
	}
	// HACKY
        if err := replace("servers.xml", j); err != nil {
                log.Fatal(err)
        }
        if err := replace("grabit.reg", j); err != nil {
                log.Fatal(err)
        }

	cmd := exec.Command("/usr/bin/makensis", "main.nsis")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		log.Fatal(err)
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	if Verbose {
		in := bufio.NewScanner(stdout)
		for in.Scan() {
			log.Println("NSIS: " + in.Text()) // write each line to your log, or anything you need
		}
		if err := in.Err(); err != nil {
			log.Printf("NSIS: %s", err)
		}
	}

	select {
	case <-time.After(3 * time.Second):
		if err := cmd.Process.Kill(); err != nil {
			log.Fatal("failed to kill process: ", err)
		}
		log.Println("process killed as timeout reached")
	case err := <-done:
		if err != nil {
			log.Fatalf("process finished with error = %v", err)
		}

		// write to client
		f, err := os.Open("autoconfig.exe")
		if err != nil {
			log.Fatal("failed to open autoconfig.exe: ", err)
		}

		if _, e := io.Copy(w, bufio.NewReader(f)); e != nil {
			log.Fatal("failed to proxy autoconfig.exe: ", err)
		}
	}
}

func main() {
	flag.BoolVar(&Verbose, "v", false, "Verbose-mode")
	flag.Parse()

	mux := &http.ServeMux{}
	mux.HandleFunc("/autoconf", build)

	s := &http.Server{
		Addr:         "127.0.0.1:2000",
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	sent, e := daemon.SdNotify(false, "READY=1")
	if e != nil {
		log.Fatal(e)
	}
	if !sent {
		log.Printf("SystemD notify NOT sent\n")
	}

	log.Panic(s.ListenAndServe())
}
