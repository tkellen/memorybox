package main

import (
	"github.com/tkellen/memorybox/internal/cli"
	"github.com/tkellen/memorybox/internal/simplecli"
	"io/ioutil"
	"log"
	"os"
)

func main() {
	log.SetFlags(0)
	logger := log.New(ioutil.Discard, "", 0)
	if err := simplecli.Run(cli.New(logger), os.Args); err != nil {
		os.Stderr.WriteString(err.Error())
		os.Exit(1)
	}
}
