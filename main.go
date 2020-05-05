package main

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
)

const version = "dev"

func main() {
	log.SetOutput(ioutil.Discard)
	code, err := Run(os.Args, os.Stdout, os.Stderr)
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			fmt.Fprintf(os.Stderr, err.Error())
		}
		os.Exit(code)
	}
}
