package main

import (
	"context"
	"errors"
	"github.com/tkellen/memorybox/internal/simplecli"
	"io/ioutil"
	"log"
	"os"
)

func main() {
	log.SetFlags(0)
	logger := log.New(ioutil.Discard, "", 0)
	if err := simplecli.Run(New(logger), os.Args); err != nil {
		if !errors.Is(err, context.Canceled) {
			os.Stderr.WriteString(err.Error() + "\n")
		}
		os.Exit(1)
	}
}
