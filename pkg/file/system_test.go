// I would prefer these be unit tests but there appears to be no coherent way in
// golang to mock *os.File, nor any clear way to remove the responsibility of
// performing real network/disk io in the implementation of this library. As a
// result, this test suite is mostly integration tests of the golang standard
// library for network/disk io (except in cases where failures needed to be
// simulated). More details here:
// https://github.com/golang/go/issues/14106
// https://github.com/golang/go/issues/21592
package file

import (
	"bytes"
	"errors"
	"io/ioutil"
	"net/http"
	"os"
	"testing"
	"testing/iotest"
)

func TestReadStdinInputSuccess(t *testing.T) {
	expectedBytes := []byte("test")
	sys, sysErr := newSystem()
	if sysErr != nil {
		t.Fatalf("test setup: %s", sysErr)
	}
	defer os.RemoveAll(sys.TempDir)
	sys.Stdin = ioutil.NopCloser(bytes.NewReader(expectedBytes))
	reader, _, err := sys.read("-")
	defer reader.Close()
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
}

func TestReadURLInputSuccess(t *testing.T) {
	input := "http://totally.legit/url.html"
	expectedBytes := []byte("test")
	getCalled := false
	sys, sysErr := newSystem()
	if sysErr != nil {
		t.Fatalf("test setup: %s", sysErr)
	}
	defer os.RemoveAll(sys.TempDir)
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
	reader, _, err := sys.read(input)
	defer reader.Close()
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
	if !bytes.Equal(expectedBytes, actualBytes) {
		t.Fatalf("expected bytes %s, got %s", expectedBytes, actualBytes)
	}
}

func TestReadURLInputFailure(t *testing.T) {
	input := "http://totally.legit/url.html"
	expectedErr := errors.New("bad request")
	getCalled := false
	sys, sysErr := newSystem()
	if sysErr != nil {
		t.Fatalf("test setup: %s", sysErr)
	}
	defer os.RemoveAll(sys.TempDir)
	// Mock failed http request.
	sys.Get = func(url string) (*http.Response, error) {
		getCalled = true
		return nil, expectedErr
	}
	_, _, actualErr := sys.read(input)
	if expectedErr != actualErr {
		t.Fatalf("expected err %s, got %s", expectedErr, actualErr)
	}
	if !getCalled {
		t.Fatal("expected http request to be made")
	}
}

func TestReadWithFilepathSuccess(t *testing.T) {
	expectedBytes := []byte("test")
	sys, sysErr := newSystem()
	if sysErr != nil {
		t.Fatalf("test setup: %s", sysErr)
	}
	defer os.RemoveAll(sys.TempDir)
	// create the file we are about to read
	inputFile, tempFileErr := ioutil.TempFile(os.TempDir(), "*")
	if tempFileErr != nil {
		t.Fatalf("test setup: %s", tempFileErr)
	}
	defer os.Remove(inputFile.Name())
	_, writeErr := inputFile.Write(expectedBytes)
	inputFile.Close()
	if writeErr != nil {
		t.Fatalf("test setup: %s", writeErr)
	}
	reader, _, err := sys.read(inputFile.Name())
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
}

func TestReadWithFilepathInputFailure(t *testing.T) {
	input := "path/to/nothing"
	sys, sysErr := newSystem()
	if sysErr != nil {
		t.Fatalf("test setup: %s", sysErr)
	}
	defer os.RemoveAll(sys.TempDir)
	_, _, err := sys.read(input)
	if err == nil {
		t.Fatalf("expected error on file not found")
	}
	if !os.IsNotExist(err) {
		t.Fatalf("expected file not found error, got: %s", err)
	}
}

func TestReadWithTempFileCreationFailure(t *testing.T) {
	input := "-"
	sys, sysErr := newSystem()
	if sysErr != nil {
		t.Fatalf("test setup: %s", sysErr)
	}
	defer os.RemoveAll(sys.TempDir)
	sys.TempDir = "/tmp/nope/bad"
	_, _, err := sys.read(input)
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

func TestSha256(t *testing.T) {
	input := []byte("test")
	expected := "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08-sha256"
	expectedSize := int64(len(input))
	actual, actualSize, goodErr := sha256(bytes.NewReader(input))
	if goodErr != nil {
		t.Fatal(goodErr)
	}
	if expected != actual {
		t.Fatalf("expected %s, got %s", expected, actual)
	}
	if expectedSize != actualSize {
		t.Fatalf("expected size %d, got %d", expectedSize, actualSize)
	}
	_, _, err := sha256(iotest.TimeoutReader(bytes.NewReader([]byte("test"))))
	if err == nil {
		t.Fatal("expected error on bad reader")
	}
}
