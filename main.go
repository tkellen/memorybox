package main

import (
	"fmt"
	"github.com/docopt/docopt-go"
	"github.com/korovkin/limiter"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
)

const concurrency = 10
const version = "dev"
const tempPrefix = "memorybox"
const usage = `memorybox

Usage:
  $0 local save [--root=<path>] <file>...
  $0 s3 save <bucket> <file>...

Options:
  -h --help          Show this screen.
  -r --root=<path>   Root storage path (local mode only) [default: ~/memorybox]
  -v --version       Show version.
`

// Store defines a storage engine that can save and index content.
type Store interface {
	Save(io.Reader, string) error
	Exists(string) bool
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

// process takes an input (stdin, file or url) and sends the bits within to the
// provided store under a content-addressable location.
func process(input string, store Store) error {
	filepath, digest, err := prepare(input)
	if err != nil {
		log.Fatalf("hashing failed: %s", err)
	}
	// if our input was buffered to a temporary file, remove it when we're done
	if filepath != input {
		defer os.Remove(filepath)
	}
	// skip file if it has already been stored
	if store.Exists(digest) {
		log.Printf("skipped: %s (exists at %s)", input, digest)
		return nil
	}
	// no matter where the data came from, it should now be in a file on disk.
	// open it for streaming to the backing store.
	file, openErr := os.Open(filepath)
	if openErr != nil {
		return fmt.Errorf("unable to open file: %s", openErr)
	}
	defer file.Close()
	if err = store.Save(file, digest); err != nil {
		return fmt.Errorf("saving failed: %s", err)
	}
	log.Printf("stored: %s (%s)", input, digest)
	return nil
}
