// package logger implements a small logger that writes the last func
// for easier debugging
package logger

import (
	"fmt"
	"log"
	"os"
	"runtime"
)

var (
	L *log.Logger
)

func init() {
	L = log.New(os.Stderr, "", 0)
}

// https://github.com/op/go-logging/blob/master/format.go
func fnName(pos int) string {
	v := "???"
	if pc, _, _, ok := runtime.Caller(pos); ok {
		if f := runtime.FuncForPC(pc); f != nil {
			v = f.Name()
		}
	}
	return v
}

func Printf(msg string, args ...interface{}) {
	L.Printf(
		fmt.Sprintf("@%s: %s", fnName(2), msg),
		args...,
	)
}

func Fatal(e error) {
	if e != nil {
		Printf(e.Error())
		os.Exit(1)
	}
}

func Logger(prefix string) *log.Logger {
	return log.New(os.Stderr, prefix, 0)
}
