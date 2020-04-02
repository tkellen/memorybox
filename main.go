package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/docopt/docopt-go"
	"github.com/korovkin/limiter"
	"github.com/segmentio/ksuid"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
)

const concurrency = 100
const version = "dev"
const usage = `memorybox

Usage:
  $0 local save [options] <file>...
  $0 s3 save <bucket> <file>...

Options:
  -h --help          Show this screen.
  -r --root=<path>   Root storage location [default: ~/memorybox]
  -v --version       Show version.
`

// Store defines a storage engine that can save and index content.
type Store interface {
	Save(io.Reader, string, func() string) error
}

func main() {
	// don't show timestamp when logging
	log.SetFlags(0)
	// respect what the user named the binary
	usage := strings.ReplaceAll(usage, "$0", filepath.Base(os.Args[0]))
	// parse command line arguments
	cli, _ := docopt.ParseArgs(usage, os.Args[1:], version)
	// determine which command we are running
	saving := cli["save"].(bool)
	local := cli["local"].(bool)
	s3 := cli["s3"].(bool)
	// prep our backing store
	var store Store
	var err error
	if local {
		store, err = NewLocalStore(cli["--root"].(string))
	}
	if s3 {
		store, err = NewObjectStore(cli["<bucket>"].(string))
	}
	if err != nil {
		log.Fatal(err)
	}
	// determine which files we are operating on
	files := cli["<file>"].([]string)
	// execute save method
	if saving {
		limit := limiter.NewConcurrencyLimiter(concurrency)
		for _, input := range files {
			path := input
			limit.Execute(func() {
				if err := process(path, store); err != nil {
					log.Printf("bad deal: %s", err)
				}
			})
		}
		limit.Wait()
	}
}

// process takes an input file path and stores the file found there using the
// provided storage engine.
func process(input string, store Store) error {
	// open the input file
	file, err := os.Open(input)
	if err != nil {
		return fmt.Errorf("unable to open file: %s: %s", input, err)
	}
	defer file.Close()
	// generate a UUID to use as the destination file name until we know the
	// hash of the contents of the file.
	temp := ksuid.New().String()
	// prepare a hash function to generate a fixed size message digest that
	// uniquely identifies the contents of the file.
	hash := sha256.New()
	// prepare to pass the contents of the file through the hash function as it
	// is being read by the storage engine.
	tee := io.TeeReader(file, hash)
	// provide a function to abstract calculating the hex representation of the
	// hash of our file contents. note: this must be called _after_ the file has
	// been read fully (that is, after all its bits have passed through the hash
	// function).
	filename := func() string {
		return "sha256-" + hex.EncodeToString(hash.Sum(nil))
	}
	// send the file to the configured storage engine
	if err := store.Save(tee, temp, filename); err != nil {
		return fmt.Errorf("saving failed: %s", err)
	}
	log.Printf("stored %s at %s", input, filename())
	return nil
}
