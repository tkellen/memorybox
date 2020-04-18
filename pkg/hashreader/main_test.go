// I would prefer these be unit tests but there appears to be no coherent way in
// golang to mock *os.File, nor any clear way to remove the responsibility of
// performing real network/disk io in the implementation of this library. As a
// result, this test suite is mostly integration tests of the golang standard
// library for network/disk io (except in cases where failures needed to be
// simulated). More details here:
// https://github.com/golang/go/issues/14106
// https://github.com/golang/go/issues/21592
package hashreader

import (
	"bytes"
	"errors"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"testing"
	"testing/iotest"
)

func TestReadStdinInputSuccess(t *testing.T) {
	tempDir, tempErr := ioutil.TempDir("", "*")
	if tempErr != nil {
		t.Fatalf("test setup: %s", tempErr)
	}
	defer os.RemoveAll(tempDir)
	expectedBytes := []byte("test")
	expectedSize := int64(len(expectedBytes))
	expectedHash := "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08-sha256"
	sys := newSystem()
	sys.Stdin = ioutil.NopCloser(bytes.NewReader(expectedBytes))
	reader, actualSize, actualHash, err := sys.read("-", tempDir)
	if err != nil {
		t.Fatal(err)
	}
	actualBytes, readErr := ioutil.ReadAll(reader)
	if readErr != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(expectedBytes, actualBytes) {
		t.Fatalf("expected bytes %s, got %s", expectedBytes, actualBytes)
	}
	if expectedSize != actualSize {
		t.Fatalf("expected size %d, got %d", expectedSize, actualSize)
	}
	if expectedHash != actualHash {
		t.Fatalf("expected hash %s, got %s", expectedHash, actualHash)
	}
}

func TestReadURLInputSuccess(t *testing.T) {
	tempDir, tempErr := ioutil.TempDir("", "*")
	if tempErr != nil {
		t.Fatalf("test setup: %s", tempErr)
	}
	defer os.RemoveAll(tempDir)
	input := "http://totally.legit/url.html"
	expectedBytes := []byte("test")
	expectedSize := int64(len(expectedBytes))
	expectedHash := "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08-sha256"
	getCalled := false
	sys := newSystem()
	// Mock every http request to contain our expected bytes.
	sys.Get = func(url string) (resp *http.Response, err error) {
		if input != url {
			t.Fatalf("expected request for %s, got %s", input, url)
		}
		getCalled = true
		return &http.Response{
			Status:        "OK",
			StatusCode:    200,
			Proto:         "HTTP/1.1",
			ProtoMajor:    1,
			ProtoMinor:    1,
			Body:          ioutil.NopCloser(bytes.NewReader(expectedBytes)),
			ContentLength: int64(len(expectedBytes)),
		}, nil
	}
	reader, actualSize, actualHash, err := sys.read(input, tempDir)
	if err != nil {
		t.Fatal(err)
	}
	if !getCalled {
		t.Fatal("expected http request to be made")
	}
	actualBytes, readErr := ioutil.ReadAll(reader)
	if readErr != nil {
		t.Fatal(err)
	}
	if expectedSize != actualSize {
		t.Fatalf("expected size %d, got %d", expectedSize, actualSize)
	}
	if !bytes.Equal(expectedBytes, actualBytes) {
		t.Fatalf("expected bytes %s, got %s", expectedBytes, actualBytes)
	}
	if expectedHash != actualHash {
		t.Fatalf("expected hash %s, got %s", expectedHash, actualHash)
	}
}

func TestReadURLInputFailure(t *testing.T) {
	tempDir, tempErr := ioutil.TempDir("", "*")
	if tempErr != nil {
		t.Fatalf("test setup: %s", tempErr)
	}
	defer os.RemoveAll(tempDir)
	input := "http://totally.legit/url.html"
	expectedErr := errors.New("bad request")
	getCalled := false
	sys := newSystem()
	// Mock failed http request.
	sys.Get = func(url string) (resp *http.Response, err error) {
		getCalled = true
		return nil, expectedErr
	}
	_, _, _, actualErr := sys.read(input, tempDir)
	if expectedErr != actualErr {
		t.Fatalf("expected err %s, got %s", expectedErr, actualErr)
	}
	if !getCalled {
		t.Fatal("expected http request to be made")
	}
}

func TestReadWithFilepathSuccess(t *testing.T) {
	tempDir, tempErr := ioutil.TempDir("", "*")
	if tempErr != nil {
		t.Fatalf("test setup: %s", tempErr)
	}
	defer os.RemoveAll(tempDir)
	expectedBytes := []byte("test")
	expectedSize := int64(len(expectedBytes))
	expectedHash := "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08-sha256"
	// create the file we are about to read
	inputFile, tempFileErr := ioutil.TempFile(tempDir, "*")
	if tempFileErr != nil {
		t.Fatalf("test setup: %s", tempFileErr)
	}
	_, writeErr := inputFile.Write(expectedBytes)
	inputFile.Close()
	if writeErr != nil {
		t.Fatalf("test setup: %s", writeErr)
	}
	reader, actualSize, actualHash, err := HashReader(inputFile.Name(), tempDir)
	if err != nil {
		t.Fatal(err)
	}
	actualBytes, readErr := ioutil.ReadAll(reader)
	if readErr != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(expectedBytes, actualBytes) {
		t.Fatalf("expected bytes %s, got %s", expectedBytes, actualBytes)
	}
	if expectedSize != actualSize {
		t.Fatalf("expected size %d, got %d", expectedSize, actualSize)
	}
	if expectedHash != actualHash {
		t.Fatalf("expected hash %s, got %s", expectedHash, actualHash)
	}
}

func TestReadWithFilepathInputFailure(t *testing.T) {
	input := "path/to/nothing"
	_, _, _, err := HashReader(input, "")
	if err == nil {
		t.Fatalf("expected error on file not found")
	}
	if !os.IsNotExist(err) {
		t.Fatalf("expected file not found error, got: %s", err)
	}
}

func TestReadWithHashFailure(t *testing.T) {
	tempDir, tempErr := ioutil.TempDir("", "*")
	if tempErr != nil {
		t.Fatalf("test setup: %s", tempErr)
	}
	defer os.RemoveAll(tempDir)
	// create the file we are about to read
	inputFile, tempFileErr := ioutil.TempFile(tempDir, "*")
	if tempFileErr != nil {
		t.Fatalf("test setup: %s", tempFileErr)
	}
	inputFile.Close()
	sys := newSystem()
	sys.Open = func(name string) (*os.File, error) {
		file, err := os.Open(name)
		file.Close()
		return file, err
	}
	_, _, _, err := sys.read(inputFile.Name(), tempDir)
	if err == nil {
		t.Fatal(err)
	}
	if !strings.Contains(err.Error(), "closed") {
		t.Fatalf("expected file already closed error, got %s", err)
	}
}

func TestReadWithTempFileCreationFailure(t *testing.T) {
	input := "-"
	_, _, _, err := HashReader(input, "/tmp/nope/bad")
	if err == nil {
		t.Fatal(err)
	}
	if !os.IsNotExist(err) {
		t.Fatalf("expected file not found error, got: %s", err)
	}
}

// These unit tests are covered by the above integration tests. Worthless?
// Beauty lies in the eye of the beholder, I guess.
func TestInputOnStdin(t *testing.T) {
	if inputIsStdin("whatever") {
		t.Fatal("expected false")
	}
	if !inputIsStdin("-") {
		t.Fatal("expected true")
	}
}

func TestInputIsURL(t *testing.T) {
	if !inputIsURL("http://") {
		t.Fatal("expected true")
	}
	if !inputIsURL("https://") {
		t.Fatal("expected true")
	}
	if inputIsURL("http") {
		t.Fatal("expected false")
	}
}

func TestHash(t *testing.T) {
	input := []byte("test")
	expected := "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08-sha256"
	expectedSize := int64(len(input))
	actual, actualSize, goodErr := hash(bytes.NewReader(input))
	if goodErr != nil {
		t.Fatal(goodErr)
	}
	if expected != actual {
		t.Fatalf("expected %s, got %s", expected, actual)
	}
	if expectedSize != actualSize {
		t.Fatalf("expected size %d, got %d", expectedSize, actualSize)
	}
	_, _, err := hash(iotest.TimeoutReader(bytes.NewReader([]byte("test"))))
	if err == nil {
		t.Fatal("expected error on bad reader")
	}
}
