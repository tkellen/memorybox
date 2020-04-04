// All network and disk IO operations are contained in these utility functions.
// The cli package which uses this functionality thus has no direct access to
// the network or local disk. This separation should be maintained to ensure
// modularity and ease of testing.
package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"strings"
)

// During normal operation temporary files may be stored on disk here.
var tempDir = path.Join(os.TempDir(), "mb")

// Reader finds the data backing an opaque input string and returns an
// io.ReadCloser and message digest of it's contents. Many efficiency
// improvements could be made here at the expense of a more complex
// implementation. Such as:
// 1. Buffer to memory instead of disk?
// 2. For content arriving via stdin or network requests, compute content hash
//    as the data is buffered rather than re-reading it after?
// 3. Send the data directly to the store under a UUID filename, computing the
//    hash as it is persisted and renaming at the end?
func inputReader(input string) (io.ReadCloser, string, error) {
	var source io.ReadCloser
	var hash string
	var err error
	// Buffer input to disk (if it isn't already there).
	if source, err = inputToFile(input); err == nil {
		// Hash input if data source could be reached.
		if hash, err = digest(source); err == nil {
			// Re-open file we just hashed so it can be read again.
			source, err = os.Open(source.(*os.File).Name())
		}
	}
	return source, hash, err
}

// outputWriter writes the content of data to stdout.
func outputWriter(data io.ReadCloser) error {
	_, err := io.Copy(os.Stdout, data)
	data.Close()
	return err
}

// Cleanup does just what you think it does.
func cleanup() error {
	return os.RemoveAll(tempDir)
}

// inputToFile converts an opaque string into an io.ReadCloser that contains the
// data the input string referred to (data on stdin, available via http request,
// or a simply the path to a file on disk).
func inputToFile(input string) (io.ReadCloser, error) {
	var source io.ReadCloser
	// Determine if we can find our input on stdin. Per common convention, we
	// recognize a single dash ("-") as meaning this.
	if input == "-" {
		// Assign source of data as being os.Stdin.
		source = os.Stdin
		defer os.Stdin.Close()
	}
	// Determine if we can find our input by making a http request.
	if strings.HasPrefix(input, "http://") || strings.HasPrefix(input, "https://") {
		// Make a HTTP request to get our data.
		// TODO: is the standard HTTP client configuration here fine?
		resp, err := http.Get(input)
		if err != nil {
			return nil, err
		}
		// Assign source of data to response body of HTTP request just made.
		source = resp.Body
		defer resp.Body.Close()
	}
	// If source is defined here, we can assume the input we want is arriving on
	// stdin or via a network request. Buffer the data to disk so it can be read
	// multiple times (once for hashing, once for sending to the store).
	if source != nil {
		filepath, err := writeToTemp(source)
		if err != nil {
			return nil, fmt.Errorf("writeToTemp: %w", err)
		}
		// Our input started out as a reference to stdin or a URL. The content
		// from it is now on disk in a temporary directory. Reassign our input
		// as a path to that file.
		input = filepath
	}
	// Return an io.ReadCloser by reading the input filepath.
	return os.Open(input)
}

// writeToTemp persists the data in an io.ReadCloser (likely populated by data
// from stdin or a network request) to a temporary file on disk and returns a
// path to it.
func writeToTemp(source io.ReadCloser) (string, error) {
	// Ensure our temporary directory exists.
	if err := os.Mkdir(tempDir, 0700); err != nil {
		return "", err
	}
	// Generate a uniquely named file under our temporary directory.
	file, err := ioutil.TempFile(tempDir, "*")
	if err != nil {
		return "", err
	}
	defer file.Close()
	// Write our incoming data to the temporary file.
	if _, err := io.Copy(file, source); err != nil {
		return "", err
	}
	return file.Name(), nil
}
