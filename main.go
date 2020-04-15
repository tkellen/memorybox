package main

import (
	"github.com/tkellen/memorybox/src"
	"os"
)

func main() {
	if err := memorybox.Run(os.Args[1:]); err != nil {
		os.Stderr.WriteString(err.Error())
		os.Exit(1)
	}
}
