package hashreader

import (
	"encoding/hex"
	"fmt"
	"github.com/minio/sha256-simd"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
)

// system defines a set of methods for network and disk IO. This is an attempt
// to make the thinnest possible abstraction to support mocking to achieve 100%
// test coverage without introducing a runtime dependency on a mocking library.
type system struct {
	Get   func(url string) (*http.Response, error)
	Open  func(string) (*os.File, error)
	Stdin io.ReadCloser
}

func newSystem() *system {
	return &system{
		Get:   http.Get,
		Open:  os.Open,
		Stdin: os.Stdin,
	}
}

// realIO provides concrete methods for the network / disk io operations that
// this package requires.
var realIO *system = newSystem()

// Read takes an input string from any supported source (local file, url or
// stdin) and returns an io.readCloser for it along with a hash of its contents.
func Read(input string, tempDir string) (io.ReadCloser, string, error) {
	return realIO.read(input, tempDir)
}

// read returns an io.readCloser and a hash of the contents it contains. Inputs
// that do not originate on local disk are stored in a temporary file to allow
// them to be read multiple times (once for hashing and once for consumers who
// wish to do something with both the content and the hash).
func (sys *system) read(input string, tempDir string) (io.ReadCloser, string, error) {
	var reader io.ReadCloser
	var err error
	// If the input string was determined to represent stdin, create a temporary
	// file that will be populated by reading it.
	if inputIsStdin(input) {
		// Note that the input string is rewritten here. It now points to
		// the temp file that will hold the content of stdin after we've read
		// and hashed it. We'll need this file so the caller has access to both
		// the hash and the data.
		reader, input, err = sys.teeFileReader(sys.Stdin, tempDir)
	}
	// If the input string was determined to be a URL, attempt a http request to
	// get the contents and create a temporary file that will be populated with
	// the body of the request.
	if inputIsURL(input) {
		var resp *http.Response
		// Make a http request for our data and continue only if it succeeded.
		if resp, err = sys.Get(input); err == nil {
			// TODO: handle non 200 responses as errors

			// Note that the input string is rewritten here. It now points to
			// the temp file that will hold the response body of our request
			// after we've read / hashed it. We'll need this file so the
			// caller has access to both the hash and the data.
			reader, input, err = sys.teeFileReader(resp.Body, tempDir)
		}
	}
	// If we don't have a reader or errors, assume the input is on disk and open
	// the file it points to.
	if reader == nil && err == nil {
		reader, err = sys.Open(input)
	}
	// By now we should have a reader and no errors. If any of the previous
	// attempts at getting a reader caused an error, bail out now.
	if err != nil {
		return nil, "", err
	}
	// Hash the contents of the reader we have obtained.
	digest, digestErr := hash(reader)
	if digestErr != nil {
		return nil, "", fmt.Errorf("hashing: %s", digestErr)
	}
	reader.Close()
	// Now that we have hashed the data our input points to, get another reader
	// so calls read the data too. In the case of an input that already resided
	// on the local disk, this is just re-opening the file. In all other cases
	// this is opening a temporary file that we expect to clean up when the
	// application finishes execution.
	reader, err = sys.Open(input)
	return reader, digest, err
}

// teeFileReader returns an io.readCloser that, when read, will populate a temp
// file in the supplied folder with the exact content that was read. See the
// read method to understand why this exists.
func (sys *system) teeFileReader(reader io.ReadCloser, tempDir string) (io.ReadCloser, string, error) {
	file, err := ioutil.TempFile(tempDir, "*")
	if err != nil {
		return nil, "", err
	}
	tee := io.TeeReader(reader, file)
	return ioutil.NopCloser(tee), file.Name(), nil
}

// inputIsStdin determines if a provided input points to data arriving over
// stdin. Per common convention, we recognize a single dash ("-") as meaning
// this.
func inputIsStdin(input string) bool {
	return input == "-"
}

// inputIsURL determines if we can find our input by making a http request.
func inputIsURL(input string) bool {
	return strings.HasPrefix(input, "http://") ||
		strings.HasPrefix(input, "https://")
}

// hash computes a sha256 message digest for a provided io.readCloser.
func hash(source io.ReadCloser) (string, error) {
	defer source.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, source); err != nil {
		return "", err
	}
	return "sha256-" + hex.EncodeToString(hash.Sum(nil)), nil
}
