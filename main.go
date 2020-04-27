package main

import (
	"github.com/tkellen/memorybox/internal/simplecli"
	"log"
	"os"
)

func main() {
	// Ensure timestamp is not included in logging messages.
	log.SetFlags(0)
	if err := simplecli.Run(New(log.Printf), os.Args); err != nil {
		os.Stderr.WriteString(err.Error())
		os.Exit(1)
	}
}
