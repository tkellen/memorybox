package file

import (
	"encoding/hex"
	"fmt"
	hash "github.com/minio/sha256-simd"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
)

// system defines a set of methods for network and disk io. This is an attempt
// to make the thinnest possible abstraction to support mocking to achieve 100%
// test coverage without introducing a runtime dependency on a mocking library.
type system struct {
	Get     func(url string) (*http.Response, error)
	Open    func(string) (*os.File, error)
	Stdin   io.ReadCloser
	TempDir string
}

func newSystem() (*system, error) {
	tempDir, err := ioutil.TempDir(os.TempDir(), "*")
	if err != nil {
		return nil, err
	}
	return &system{
		Get:     http.Get,
		Open:    os.Open,
		Stdin:   os.Stdin,
		TempDir: tempDir,
	}, nil
}

// read produces an io.ReadCloser from any supported source and ensures
// the backing data can be read multiple times.
func (sys *system) read(src interface{}) (io.ReadCloser, string, error) {
	// If the input is an io.ReadCloser already, create a temporary file that
	// will be populated by reading it.
	if reader, ok := src.(io.ReadCloser); ok {
		return sys.teeTempReader(reader, sys.TempDir)
	}
	input, ok := src.(string)
	if !ok {
		return nil, "", fmt.Errorf("unsupposed source: %s", src)
	}
	// If the input string was determined to represent stdin, create a temporary
	// file that will be populated by reading it.
	if inputIsStdin(input) {
		return sys.teeTempReader(sys.Stdin, sys.TempDir)
	}
	// If the input string was determined to be a URL, attempt a http request to
	// get the contents and create a temporary file that will be populated with
	// the body of the request as it is read.
	if inputIsURL(input) {
		resp, err := sys.Get(input)
		if err != nil {
			return nil, "", err
		}
		if !(resp.StatusCode >= 200 && resp.StatusCode <= 299) {
			return nil, "", fmt.Errorf("http code: %d", resp.StatusCode)
		}
		return sys.teeTempReader(resp.Body, sys.TempDir)
	}
	// If we made it here, assume the input is on disk and just open it.
	reader, err := sys.Open(input)
	if err != nil {
		return nil, "", err
	}
	return reader, input, nil
}

// teeTempReader returns an io.readCloser that, when read, will populate a temp
// file in the supplied folder with the exact content that was read. See the
// read method to understand why this exists.
func (sys *system) teeTempReader(reader io.ReadCloser, tempDir string) (io.ReadCloser, string, error) {
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

// sha256 computes a sha256 message digest for a provided io.readCloser.
func sha256(source io.Reader) (string, int64, error) {
	hash := hash.New()
	size, err := io.Copy(hash, source)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(hash.Sum(nil)) + "-sha256", size, nil
}
