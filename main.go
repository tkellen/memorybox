package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/docopt/docopt-go"
	"github.com/korovkin/limiter"
	"hash"
	"io"
	"io/ioutil"
	"log"
	"net/http"
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

// isRemote returns true if a given input is a url, false otherwise (assumes a
// local file).
func isRemote(input string) bool {
	return strings.HasPrefix(input, "http://") ||
		strings.HasPrefix(input, "https://")
}

// localRead calculates the sha256 digest of a file on disk and returns the
// path to the file and the result.
func localRead(filepath string, hash hash.Hash) (string, error) {
	file, openErr := os.Open(filepath)
	if openErr != nil {
		return "", fmt.Errorf("unable to open file: %s", openErr)
	}
	defer file.Close()
	_, err := io.Copy(hash, file)
	if err != nil {
		return "",fmt.Errorf("unable to hash file: %s", err)
	}
	return filepath, nil
}

// remoteRead downloads a file from a remote endpoint and computes the sha256
// digest of its contents as it stores it to a temporary location on disk. both
// of these values are returned to the consumer.
func remoteRead(url string, hash hash.Hash) (string, error) {
	// make a get request for our file
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to get url: %s", err)
	}
	defer resp.Body.Close()
	// prepare a temporary file on disk to store the download
	file, ioErr := ioutil.TempFile("", tempPrefix)
	if ioErr != nil {
		return "", fmt.Errorf("failed to create temp file: %s", ioErr)
	}
	defer file.Close()
    // pass the contents of the file through the hash function as it is being
    // downloaded to disk.
    io.TeeReader(file, hash)
    if _, err := io.Copy(file, resp.Body); err != nil {
    	return "", fmt.Errorf("failed to download file: %s", err)
	}
	return file.Name(), nil
}

// process takes an input (file or url) and copies what exists there to the
// provided store under a content-addressable location.
func process(input string, store Store) error {
	var filepath string
	var err error
	var reader func(string, hash.Hash) (string, error)
	hash := sha256.New()
	// determine if our input is on local disk or if it is coming from
	if isRemote(input) {
		// use the remote read strategy of downloading the input to a temporary
		// file and hashing it in the process.
		reader = remoteRead
		// we hash remote files by downloading them to a local temp file
		// before sending them to the provided backing store. this makes sure
		// we clean up after ourselves
		defer func() {
			os.Remove(filepath)
		}()
	} else {
		// use the local read strategy of just hashing the contents before we
		// start.
		reader = localRead
	}
	// get the path to file and push its contents through our hashing function
	filepath, err = reader(input, hash)
	if err != nil {
		return fmt.Errorf("unable to process input: %s", err)
	}
	// calculate the hex value of our hashing function to use as the filename
	// in our storage engine.
	dest := "sha256-" + hex.EncodeToString(hash.Sum(nil))
	// skip file if it has already been stored
	if store.Exists(dest) {
		log.Printf("skipped: %s (exists at %s)", input, dest)
		return nil
	}
	// no matter where the file came from, it should now be on disk. open it
	// for streaming to the backing store.
	file, openErr := os.Open(filepath)
	if openErr != nil {
		return fmt.Errorf("unable to open file: %s", openErr)
	}
	defer file.Close()
	if err := store.Save(file, dest); err != nil {
		return fmt.Errorf("saving failed: %s", err)
	}
	log.Printf("stored: %s (%s)", input, dest)
	return nil
}

