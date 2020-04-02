package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/docopt/docopt-go"
	"github.com/mitchellh/go-homedir"
	"github.com/segmentio/ksuid"
	"io"
	"log"
	"os"
	"path"
	"sync"
)

const VERSION = "dev"
const USAGE = `memorybox

Usage:
  mb save [options] <file>...

Options:
  -h --help       Show this screen.
  -v --version    Show version.
`

// Store defines a storage engine that can save and index content.
type Store interface {
	Save(*io.Reader) (string, error)
	Index(string, string) error
}

// LocalStore is a Store implementation that uses local disk.
type LocalStore struct {
	RootPath string
}

// NewLocalStore returns a reference to a LocalStore instance that, by default,
// writes to ~/memorybox.
// TODO: allow this to be configured with a command line flag.
func NewLocalStore() (*LocalStore, error) {
	homeDir, err := homedir.Dir()
	if err != nil {
		return nil, fmt.Errorf("unable to locate home directory: %s", err)
	}
	rootPath := path.Join(homeDir, "memorybox")
	err = os.MkdirAll(rootPath, 0755)
	if err != nil {
		return nil, fmt.Errorf("could not create %s: %s", rootPath, err)
	}
	return &LocalStore{RootPath: rootPath}, nil
}

// Save reads the content of the io.Reader passed to it and returns the path to
// where it was written.
func (s *LocalStore) Save(src *io.Reader) (string, error) {
	// Generate a UUID to use as a filename for the bits we are about to save.
	temp := ksuid.New().String()
	// Create a file using the UUID.
	dest, err := os.Create(path.Join(s.RootPath, temp))
	if err != nil {
		return "", fmt.Errorf("local store failed: %s", err)
	}
	// Copy the file into place, returning an error if any.
	if _, err := io.Copy(dest, *src); err != nil {
		return "", fmt.Errorf("local store write failed: %s", err)
	}
	// Return the UUID used for the filenaming. The indexing process will
	// move it to its final location (a file whose name is a hash of the content
	// of the file itself.
	return temp, nil
}

// Index copies a temporary file sent to the store to its final location.
// TODO: perform indexing operations here
func (s *LocalStore) Index(src string, hash string) error {
	return os.Rename(path.Join(s.RootPath, src), path.Join(s.RootPath, hash))
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
	// copy the file to the configured storage engine and give us back the path
	// to where it was temporarily written.
	tempPath, saveErr := store.Save(&tee)
	if saveErr != nil {
		return fmt.Errorf("saving failed: %s", saveErr)
	}
	// the save method above "pulled" the entire contents of the file through
	// our hashing function to produce a "message digest" (a fixed length string
	// that uniquely identifies the content of the file). now get a hexadecimal
	// representation of it so we can properly index the file using our storage
	// engine.
	digest := hex.EncodeToString(hash.Sum(nil))
	// rename the temporary file in our store to its content hash and also index
	// the details about it.
	indexErr := store.Index(tempPath, digest)
	if indexErr != nil {
		log.Printf("indexing failed: %s", indexErr)
	}
	log.Printf("stored %s at %s", input, path.Join(store.(*LocalStore).RootPath, digest))
	return nil
}

func main() {
	// don't show timestamp when logging
	log.SetFlags(0)
	// parse command line arguments
	cli, _ := docopt.ParseArgs(USAGE, os.Args[1:], VERSION)
	// determine which command we are running
	saving := cli["save"].(bool)
	// determine which files we are acting on
	files := cli["<file>"].([]string)
	// generate a local store for saving data
	// TODO: support s3 object storage
	store, err := NewLocalStore()
	if err != nil {
		log.Fatal(err)
	}
	// if we are in save mode, process every input file concurrently
	// TODO: put some hard limit on the concurrency perhaps?
	if saving {
		var wg sync.WaitGroup
		wg.Add(len(files))
		for _, input := range files {
			go func(input string) {
				defer wg.Done()
				if err := process(input, store); err != nil {
					log.Printf("bad deal: %s", err)
				}
			}(input)
		}
		wg.Wait()
	}
}