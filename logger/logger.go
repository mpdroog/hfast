// package logger implements a small log abstraction
// Why yet another logger?
// Because I wanted a descent log filter
// What I came up with:
// Only log todo's and allow strict filtering by prefixing funcs
// @package.func: logmsg
package logger

import (
	"log"
	"os"
	"fmt"
	"runtime"
)

var L *log.Logger

func init() {
	L = log.New(os.Stderr, "", 0)
}

// https://github.com/op/go-logging/blob/master/format.go
func fnName() string {
	v := "???"
	if pc, _, _, ok := runtime.Caller(2); ok {
		if f := runtime.FuncForPC(pc); f != nil {
			v = f.Name()
		}
	}
	return v
}

func Printf(msg string, args ...interface{}) {
	L.Printf(
		fmt.Sprintf("@%s: %s", fnName(), msg),
		args...
	)
}

func Fatal(e error) {
	if e != nil {
		Printf(e.Error())
		os.Exit(1)
	}
}