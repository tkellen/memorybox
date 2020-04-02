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
	"path"
)

const MAX_CONCURRENCY = 100
const VERSION = "dev"
const USAGE = `memorybox

Usage:
  mb local save [options] <file>...
  mb s3 save <bucket> <file>...

Options:
  -h --help          Show this screen.
  -r --root=<path>   Root storage location [default: ~/memorybox]
  -v --version       Show version.
`

// Store defines a storage engine that can save and index content.
type Store interface {
	Save(io.Reader, string) error
	Index(string, string) error
	Root() string
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
	// prepare a hash function to generate a fixed size message digest that
	// uniquely identifies the contents of the file.
	hash := sha256.New()
	// prepare to pass the contents of the file through the hash function as it
	// is being read by the storage engine.
	tee := io.TeeReader(file, hash)
	// generate a UUID to use as the destination file name until we know the
	// hash of the contents of the file.
	temp := ksuid.New().String()
	// send the file to the configured storage engine
	if err := store.Save(tee, temp); err != nil {
		return fmt.Errorf("saving failed: %s", err)
	}
	// the save method above "pulled" the entire contents of the file through
	// our hashing function to produce a "message digest" (a fixed length string
	// that uniquely identifies the content of the file). now get a hexadecimal
	// representation of it so we can properly index the file using our storage
	// engine.
	digest := hex.EncodeToString(hash.Sum(nil))
	// rename the temporary file in our store to its content hash and also index
	// the details about it.
	indexErr := store.Index(temp, digest)
	if indexErr != nil {
		log.Printf("indexing failed: %s", indexErr)
	}
	log.Printf("stored %s at %s", input, path.Join(store.Root(), digest))
	return nil
}

func main() {
	// don't show timestamp when logging
	log.SetFlags(0)
	// parse command line arguments
	cli, _ := docopt.ParseArgs(USAGE, os.Args[1:], VERSION)
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
		limit := limiter.NewConcurrencyLimiter(MAX_CONCURRENCY)
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
