package cli

import (
	"bytes"
	"github.com/tkellen/memorybox/pkg/test"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
)

func TestReadWithStdinInput(t *testing.T) {
	tempDir := test.TempDir()
	defer os.RemoveAll(tempDir)
	expectedOutput := []byte("test")
	reader, filepath, err := read("-", test.GoodReadCloser(expectedOutput), tempDir)
	if err != nil {
		t.Fatal(err)
	}
	actualOutput, readErr := ioutil.ReadAll(reader)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !bytes.Equal(expectedOutput, actualOutput) {
		t.Fatalf("expected %s, got %s", expectedOutput, actualOutput)
	}
	tempFileBytes, tempFileErr := ioutil.ReadFile(filepath)
	if tempFileErr != nil {
		t.Fatalf("expected temp file: %s", tempFileErr)
	}
	if !bytes.Equal(expectedOutput, tempFileBytes) {
		t.Fatalf("expected %s, got %s", expectedOutput, tempFileBytes)
	}
}

func TestReadWithURLInput(t *testing.T) {
	tempDir := test.TempDir()
	defer os.RemoveAll(tempDir)
	expectedOutput := []byte("test")
	listen, listenErr := net.Listen("tcp", "127.0.0.1:0")
	if listenErr != nil {
		t.Fatal(listenErr)
	}
	go http.Serve(listen, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(expectedOutput)
	}))
	response, filepath, err := read("http://"+listen.Addr().String(), os.Stdout, tempDir)
	if err != nil {
		t.Fatal(err)
	}
	actualOutput, readErr := ioutil.ReadAll(response)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !bytes.Equal(expectedOutput, actualOutput) {
		t.Fatalf("expected %s, got %s", expectedOutput, actualOutput)
	}
	tempFileBytes, tempFileErr := ioutil.ReadFile(filepath)
	if tempFileErr != nil {
		t.Fatalf("expected temp file: %s", tempFileErr)
	}
	if !bytes.Equal(expectedOutput, tempFileBytes) {
		t.Fatalf("expected %s, got %s", expectedOutput, tempFileBytes)
	}
}

func TestReadWithURLInputFailure(t *testing.T) {
	_, _, err := read("http://invalidurl.ouch", os.Stdin, "/")
	if err == nil {
		t.Fatal("expected failure to fetch url")
	}
	if !strings.Contains(err.Error(), "no such host") {
		t.Fatalf("expected host lookup failure, got %s", err)
	}
}

func TestReadWithFilepathInput(t *testing.T) {
	expectedOutput := []byte("test")
	file, tempErr := ioutil.TempFile("", "*")
	if tempErr != nil {
		t.Fatalf("setting up test: %s", tempErr)
	}
	filepath := file.Name()
	defer os.RemoveAll(filepath)
	_, writeErr := io.Copy(file, test.GoodReadCloser(expectedOutput))
	if writeErr != nil {
		t.Fatalf("setting up test: %s", writeErr)
	}
	response, resolvedFilepath, err := read(filepath, os.Stdin, "/")
	if err != nil {
		t.Fatal(err)
	}
	actualOutput, readErr := ioutil.ReadAll(response)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !bytes.Equal(expectedOutput, actualOutput) {
		t.Fatalf("expected %s, got %s", expectedOutput, actualOutput)
	}
	if filepath != resolvedFilepath {
		t.Fatalf("expected no temp file when input is already on disk")
	}
}

func TestTempTeeReader(t *testing.T) {
	tempDir := test.TempDir()
	defer os.RemoveAll(tempDir)
	expected := []byte("test")
	reader, filepath, err := tempTeeReader(test.GoodReadCloser(expected), tempDir)
	if err != nil {
		t.Fatal(err)
	}
	readerActual, readAllErr := ioutil.ReadAll(reader)
	if readAllErr != nil {
		t.Fatalf("reading returned reader: %s", err)
	}
	if !bytes.Equal(expected, readerActual) {
		t.Fatalf("expected reader to contain %s, got %s", expected, readerActual)
	}
	actual, readErr := ioutil.ReadFile(filepath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !bytes.Equal(expected, actual) {
		t.Fatalf("expected %s to be written to temp dir, got %s", expected, actual)
	}
}

func TestTempTeeReaderCreateDirFailure(t *testing.T) {
	data := test.GoodReadCloser([]byte("test"))
	_, filepath, err := tempTeeReader(data, "bad/nested/path")
	if err == nil {
		t.Fatal("expected failure to create directory")
	}
	if filepath != "" {
		t.Fatalf("got %s, expected empty string", filepath)
	}
}

func TestTempTeeReaderCreateFileFailure(t *testing.T) {
	data := test.GoodReadCloser([]byte("test"))
	// so brittle. assumes you can't write here.
	_, filepath, err := tempTeeReader(data, "/")
	if err == nil {
		t.Fatal("expected failure to create temporary file")
	}
	if filepath != "" {
		t.Fatalf("got %s, expected empty string", filepath)
	}
}
