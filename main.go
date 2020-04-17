package main

import (
	"github.com/tkellen/memorybox/pkg"
	"github.com/tkellen/memorybox/pkg/simplecli"
	"log"
	"os"
)

func main() {
	// Ensure timestamp is not included in logging messages.
	log.SetFlags(0)
	if err := simplecli.Run(&memorybox.Runner{}, os.Args); err != nil {
		os.Stderr.WriteString(err.Error())
		os.Exit(1)
	}
}
