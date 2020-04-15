// These are integration tests which validate the minimal domain specific error
// handling this store layers over the golang standard libraries for os-agnostic
// path resolution and disk io. Mocking out the filesystem for this (as seen in
// the hashreader package) seemed like overkill.
package localdiskstore

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"testing"
	"testing/iotest"
)

func TimeoutReadCloser(input []byte) io.ReadCloser {
	return ioutil.NopCloser(iotest.TimeoutReader(bytes.NewReader(input)))
}

func ReadCloser(input []byte) io.ReadCloser {
	return ioutil.NopCloser(bytes.NewReader(input))
}

func TestNewFromTarget(t *testing.T) {
	expected := "test"
	actual := NewFromTarget(map[string]string{
		"home": expected,
	})
	if expected != actual.RootPath {
		t.Fatalf("expected rootPath of %s, got %s", expected, actual.RootPath)
	}
}

func TestStore_String(t *testing.T) {
	store := New("/")
	actual := store.String()
	expected := fmt.Sprintf("LocalDiskStore: %s", "/")
	if expected != actual {
		t.Fatalf("expected %s, got %s", expected, actual)
	}
}

func TestStore_Put(t *testing.T) {
	tempDir, tempErr := ioutil.TempDir("", "*")
	if tempErr != nil {
		t.Fatalf("test setup: %s", tempErr)
	}
	defer os.RemoveAll(tempDir)
	store := New(tempDir)
	filename := "test"
	expected := []byte(filename)
	putErr := store.Put(ReadCloser(expected), filename)
	if putErr != nil {
		t.Fatal(putErr)
	}
	actual, readErr := ioutil.ReadFile(path.Join(tempDir, filename))
	if readErr != nil {
		t.Fatalf("reading bytes written: %s", readErr)
	}
	if !bytes.Equal(expected, actual) {
		t.Fatalf("expected put file to contain %s, got %s", expected, actual)
	}
}

func TestStore_Put_CannotCreateHome(t *testing.T) {
	file, tempErr := ioutil.TempFile("", "*")
	if tempErr != nil {
		t.Fatalf("failed setting up test: %s", tempErr)
	}
	defer os.Remove(file.Name())
	store := New(file.Name())
	filename := "test"
	expected := []byte(filename)
	putErr := store.Put(ReadCloser(expected), filename)
	if putErr == nil {
		t.Fatal("expected error creating home directory")
	}
}

func TestStore_Put_BadReader(t *testing.T) {
	tempDir, tempErr := ioutil.TempDir("", "*")
	if tempErr != nil {
		t.Fatalf("test setup: %s", tempErr)
	}
	defer os.RemoveAll(tempDir)
	store := New(tempDir)
	putErr := store.Put(TimeoutReadCloser([]byte("test")), "test")
	if putErr == nil {
		t.Fatal("expected put error")
	}
}

func TestStore_Put_CannotCreateFile(t *testing.T) {
	tempDir, tempErr := ioutil.TempDir("", "*")
	if tempErr != nil {
		t.Fatalf("test setup: %s", tempErr)
	}
	defer os.RemoveAll(tempDir)
	store := New(tempDir)
	putErr := store.Put(TimeoutReadCloser([]byte("test")), path.Join("test", "file"))
	if putErr == nil {
		t.Fatal("expected put error")
	}
}

func TestStore_Get(t *testing.T) {
	tempDir, tempErr := ioutil.TempDir("", "*")
	if tempErr != nil {
		t.Fatalf("test setup: %s", tempErr)
	}
	defer os.RemoveAll(tempDir)
	store := New(tempDir)
	filename := "test"
	expected := []byte("test")
	writeErr := ioutil.WriteFile(path.Join(tempDir, filename), expected, 0644)
	if writeErr != nil {
		t.Fatalf("failed setting up test: %s", writeErr)
	}
	data, getErr := store.Get("test")
	defer data.Close()
	if getErr != nil {
		t.Fatal(getErr)
	}
	actual, readErr := ioutil.ReadAll(data)
	if readErr != nil {
		t.Fatalf("failed reading response: %s", readErr)
	}
	if !bytes.Equal(expected, actual) {
		t.Fatalf("expected get to contain %s, got %s", expected, actual)
	}
}

func TestStore_Exists(t *testing.T) {
	tempDir, tempErr := ioutil.TempDir("", "*")
	if tempErr != nil {
		t.Fatalf("test setup: %s", tempErr)
	}
	defer os.RemoveAll(tempDir)
	store := New(tempDir)
	filename := "test"
	writeErr := ioutil.WriteFile(path.Join(tempDir, filename), []byte("test"), 0644)
	if writeErr != nil {
		t.Fatalf("failed setting up test: %s", writeErr)
	}
	if !store.Exists(filename) {
		t.Fatal("expected boolean true for file that exists")
	}
	if store.Exists("nope") {
		t.Fatal("expected boolean false for file that does not exist")
	}
}
