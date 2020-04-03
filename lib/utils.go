package memorybox

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
)

// Return a message digest calculated by hashing the content of a file on disk.
func hashFile(filepath string) (string, error) {
	hash := sha256.New()
	file, openErr := os.Open(filepath)
	if openErr != nil {
		return "", openErr
	}
	defer file.Close()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	// Include a prefix indicating what hashing algorithm was used to provide
	// an upgrade path for future versions which may need to change this.
	return "sha256-" + hex.EncodeToString(hash.Sum(nil)), nil
}

// Determine if an input string represents data that is not yet on disk. The
// conditionals within enumerate what external protocols are supported by this
// system (- is a posix convention that indicates the input is arriving via
// standard in).
func notOnDisk(input string) bool {
	return input == "-" || strings.HasPrefix(input, "http://") ||
		strings.HasPrefix(input, "https://")
}

// Write a provided input to a temporary location on disk and return the path.
// Note:
// It would be possible to calculate the sha256 hash of the stream as it is
// written to disk at the expense of a slightly more complex implementation.
func saveToTemp(input string, logger Logger) (string, error) {
	var stream io.Reader
	tempFile, err := ioutil.TempFile("", "mb")
	if err != nil {
		return "", err
	}
	defer tempFile.Close()
	if input == "-" {
		stream = os.Stdin
	} else {
		resp, getErr := http.Get(input)
		if getErr != nil {
			return "", err
		}
		defer resp.Body.Close()
		stream = resp.Body
	}
	if _, err := io.Copy(tempFile, stream); err != nil {
		return "", err
	}
	return tempFile.Name(), nil
}
