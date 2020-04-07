// These are integration tests that validate the low level methods that perform
// disk/network IO.
package cli

import (
	"bytes"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"path"
	"runtime"
	"strings"
	"testing"
	"testing/iotest"
)

// get a path to a temporary directory that matches the callers function name
func testTempDir() string {
	pc := make([]uintptr, 10)
	runtime.Callers(2, pc)
	parts := strings.Split(runtime.FuncForPC(pc[0]).Name(), ".")
	path := path.Join(os.TempDir(), parts[len(parts)-1])
	os.RemoveAll(path)
	return path
}

func TestInputReaderWithStdinSource(t *testing.T) {
	t.Log("TODO: can this even be integration tested?")
}

func TestInputReaderWithHttpResource(t *testing.T) {
	tempDir := testTempDir()
	defer os.RemoveAll(tempDir)
	expectedOutput := []byte("test")
	expectedHash := "sha256-9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
	listen, listenErr := net.Listen("tcp", "127.0.0.1:0")
	if listenErr != nil {
		t.Fatal(listenErr)
	}
	go http.Serve(listen, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(expectedOutput)
	}))
	response, hash, err := inputReader("http://"+listen.Addr().String(), tempDir)
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
	if hash != expectedHash {
		t.Fatalf("expected hash of %s, got %s", hash, expectedHash)
	}
}

func TestInputReaderIoCopyFailure(t *testing.T) {
	t.Log("TODO")
}

func TestOutputWriter(t *testing.T) {
	t.Log("TODO")
}

func TestWipeDir(t *testing.T) {
	dir := testTempDir()
	if err := os.Mkdir(dir, 0700); err != nil && !os.IsExist(err) {
		log.Fatalf("creating temp directory :%s", err)
	}
	file, tempErr := ioutil.TempFile(dir, "*")
	if tempErr != nil {
		t.Fatalf("creating file in %s: %s", dir, tempErr)
	}
	file.Close()
	err := wipeDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("expected %s to not exist", dir)
	}
}

func TestInputToFile(t *testing.T) {
	t.Log("TODO")
}

func TestWriteToTemp(t *testing.T) {
	tempDir := testTempDir()
	defer os.RemoveAll(tempDir)
	expected := []byte("test")
	reader := ioutil.NopCloser(bytes.NewReader(expected))
	filepath, err := writeToTemp(reader, tempDir)
	if err != nil {
		t.Fatal(err)
	}
	actual, readErr := ioutil.ReadFile(filepath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !bytes.Equal(expected, actual) {
		t.Fatalf("expected %s to be written to temp dir, got %s", expected, actual)
	}
}

func TestWriteToTempCreateDirFailure(t *testing.T) {
	data := ioutil.NopCloser(bytes.NewReader([]byte("test")))
	filepath, err := writeToTemp(data, "bad/nested/path")
	if err == nil {
		t.Fatal("expected failure to create directory")
	}
	if filepath != "" {
		t.Fatalf("got %s, expected empty string", filepath)
	}
}

func TestWriteToTempCreateFileFailure(t *testing.T) {
	data := ioutil.NopCloser(bytes.NewReader([]byte("test")))
	// so brittle. assumes you can't write here.
	filepath, err := writeToTemp(data, "/")
	if err == nil {
		t.Fatal("expected failure to create temporary file")
	}
	if filepath != "" {
		t.Fatalf("got %s, expected empty string", filepath)
	}
}

func TestWriteToTempWriteFailure(t *testing.T) {
	tempDir := testTempDir()
	defer os.RemoveAll(tempDir)
	data := ioutil.NopCloser(iotest.TimeoutReader(bytes.NewReader([]byte("test"))))
	filepath, err := writeToTemp(data, tempDir)
	if err == nil {
		t.Fatal("expected error failing to read input")
	}
	if !strings.Contains(err.Error(), "timeout") {
		t.Fatalf("expected timout error: %s", err)
	}
	if filepath != "" {
		t.Fatalf("got %s, expected empty string", filepath)
	}
}
