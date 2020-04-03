package memorybox

import (
	"fmt"
	"io"
	"os"
)

// Store defines a storage engine that can save and index content.
type Store interface {
	Put(io.Reader, string) error
	Get(string) (io.Reader, error)
	Search(string) ([]string, error)
	Exists(string) bool
}

// Logger defines a method prototype for logging output
type Logger func(string, ...interface{})

// Put is a generic method sends input to any Store.
func Put(input string, store Store, logger Logger) error {
	// Save the input the user specified so we can log appropriately.
	userInput := input
	// If the input isn't on disk yet because it is arriving via stdin or is
	// hosted on the internet, buffer it to disk.
	if notOnDisk(input) {
		filepath, err := saveToTemp(input, logger)
		if err != nil {
			return fmt.Errorf("buffering: %s", err)
		}
		// Ensure we clean up the temp file after work is done.
		defer os.Remove(filepath)
		input = filepath
	}
	// Hash the contents of our file.
	digest, err := hashFile(input)
	if err != nil {
		return fmt.Errorf("hashing: %s", err)
	}
	// Skip file if it has already been stored.
	if store.Exists(digest) {
		logger("%s -> %s (skipped, already exists)", userInput, digest)
		return nil
	}
	// Open the file we're about to copy.
	file, openErr := os.Open(input)
	if openErr != nil {
		return openErr
	}
	defer file.Close()
	// Stream file to backing store, using the hash of its content as the name.
	if err = store.Put(file, digest); err != nil {
		return fmt.Errorf("%s %s: %s", store, userInput, err)
	}
	logger("%s -> %s", userInput, digest)
	return nil
}

// Search is a generic method that finds files matching a prefix in any Store.
func Search(request string, store Store) ([]string, error) {
	matches, err := store.Search(request)
	if err != nil {
		return matches, err
	}
	return matches, nil
}

// Get is a generic method streams a file from any Store to stdout.
func Get(request string, store Store, logger Logger) error {
	matches, err := Search(request, store)
	if err != nil {
		return fmt.Errorf("search: %s", err)
	}
	if len(matches) != 1 {
		return fmt.Errorf("%d files matched", len(matches))
	}
	file, getErr := store.Get(matches[0])
	if getErr != nil {
		return err
	}
	// TODO: deal with responsibility of file closing
	io.Copy(os.Stdout, file)
	return nil
}
