// All disk IO operations are abstracted in the methods below.
// This separation should be maintained for ease of testing.
package cli

import (
	"io"
	"io/ioutil"
	"net/http"
	"os"
)

// Data being stored in this system must be read twice, once to compute the
// content hash and once to stream to the backing store. This is easy when the
// data to be stored is on local disk, just read the file twice. It's a bit
// trickier when the data resides at a remote url or is arriving from stdin.
// In the case of stdin, the data cannot be read twice. In the case of a URL,
// requesting it from the network twice could severely hamper performance.
// For input strings that represent data not yet on disk, the io.ReadCloser
// is tee'd through a file in the temporary directory. As the data is streamed
// through a hashing function it is also written to disk. When it is time to
// send the data to the backing store, it is read from the temporary file.
// Once all of this work is done, the temporary file is deleted.
func read(input string, stdin io.ReadCloser, tempDir string) (io.ReadCloser, string, error) {
	if inputIsStdin(input) {
		// Stdin is provided by the second argument in this function so it can
		// be properly mocked in testing. It's a bit confusing, but it works.
		return tempTeeReader(stdin, tempDir)
	}
	if inputIsURL(input) {
		// If we think the input is on the internet, go get it.
		resp, err := http.Get(input)
		if err != nil {
			return nil, "", err
		}
		return tempTeeReader(resp.Body, tempDir)
	}
	// If we made it here, we assume the file we want is on disk already and
	// just return a handle to it.
	reader, err := os.Open(input)
	return reader, input, err
}

// teeTempReader generates a file in the specified temporary directory and
// returns an io.ReadCloser that, when read, will also populate the temporary
// file with the contents being read. See the documentation for `read` to
// understand why this exists.
func tempTeeReader(reader io.ReadCloser, tempDir string) (io.ReadCloser, string, error) {
	// Ensure our temporary dir exists (ignore error if it is already there)
	if err := os.Mkdir(tempDir, 0700); err != nil && !os.IsExist(err) {
		return nil, "", err
	}
	// Generate a uniquely named file under our temporary directory.
	file, err := ioutil.TempFile(tempDir, "*")
	if err != nil {
		return nil, "", err
	}
	// Generate a reader that writes the contents of the supplied io.ReadCloser
	// to our temporary file when it is read.
	tee := io.TeeReader(reader, file)
	// Return our tee'd reader and a path to our temporary file.
	return ioutil.NopCloser(tee), file.Name(), nil
}
