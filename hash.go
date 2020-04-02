package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
)

// prepare reads input and computes a hash from its content.
func prepare(input string) (string, string, error) {
	var filepath string
	var err error
	hash := sha256.New()
	// determine if our input is not already on the local filesystem
	if isStreaming(input) {
		if filepath, err = hashToTempFile(input, hash); err != nil {
			return "", "", fmt.Errorf("preparing from stream failed: %s", err)
		}
	} else {
		filepath = input
		if err = hashFromDisk(input, hash); err != nil {
			return "", "", fmt.Errorf("preparing from disk failed: %s", err)
		}
	}
	// calculate the hex value of our hashing function to use as the filename
	return filepath, "sha256-" + hex.EncodeToString(hash.Sum(nil)), nil
}

// isStreaming returns true if a given input is a url or the data is expected
// to arrive on stdin.
func isStreaming(input string) bool {
	return input == "-" || strings.HasPrefix(input, "http://") || strings.HasPrefix(input, "https://")
}

// hashToTempFile stores our input in a temporary location on disk, pushing the
// bits through a supplied hashing function on the way.
func hashToTempFile(input string, hash hash.Hash) (string, error) {
	var stream io.Reader
	if input == "-" {
		stream = os.Stdin
	} else {
		resp, err := http.Get(input)
		if err != nil {
			return "", fmt.Errorf("failed to get url: %s", err)
		}
		defer resp.Body.Close()
		stream = resp.Body
	}
	tempFile, err := ioutil.TempFile("", tempPrefix)
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %s", err)
	}
	defer tempFile.Close()
	// pass the contents of the file through the hash function as it is being
	// downloaded to disk.
	tee := io.TeeReader(stream, hash)
	if _, err := io.Copy(tempFile, tee); err != nil {
		return "", fmt.Errorf("failed to buffer file: %s", err)
	}
	return tempFile.Name(), nil
}

func hashFromDisk(filepath string, hash hash.Hash) error {
	file, openErr := os.Open(filepath)
	if openErr != nil {
		return fmt.Errorf("unable to open file: %s", openErr)
	}
	defer file.Close()
	if _, err := io.Copy(hash, file); err != nil {
		return fmt.Errorf("unable to hash file: %s", err)
	}
	return nil
}
