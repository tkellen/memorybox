// These are integration tests which validate the minimal domain specific error
// handling this store layers over the golang standard libraries for os-agnostic
// path resolution and disk io. The fact that these actually perform real disk
// io is non-optimal but mocking out the filesystem seemed like overkill.
package localstore

import (
	"bytes"
	"fmt"
	"github.com/tkellen/memorybox/pkg/test"
	"io/ioutil"
	"os"
	"path"
	"testing"
)

func TestNew(t *testing.T) {
	dir := test.TempDir()
	defer os.RemoveAll(dir)
	_, err := New(dir)
	if err != nil {
		t.Fatal("expected no errors")
	}
}

func TestNewCannotCreateHome(t *testing.T) {
	dir := test.TempDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("failed setting up test: %s", err)
	}
	file, tempErr := ioutil.TempFile(dir, "*")
	if tempErr != nil {
		t.Fatalf("failed setting up test: %s", tempErr)
	}
	defer os.RemoveAll(dir)
	_, err := New(file.Name())
	if err == nil {
		t.Fatal("expected error creating store root")
	}
}

func TestString(t *testing.T) {
	dir := test.TempDir()
	defer os.RemoveAll(dir)
	store, storeErr := New(dir)
	if storeErr != nil {
		t.Fatalf("failed setting up test: %s", storeErr)
	}
	actual := store.String()
	expected := fmt.Sprintf("LocalStore: %s", dir)
	if expected != actual {
		t.Fatalf("expected %s, got %s", expected, actual)
	}
}

func TestPut(t *testing.T) {
	dir := test.TempDir()
	defer os.RemoveAll(dir)
	store, storeErr := New(dir)
	if storeErr != nil {
		t.Fatalf("failed setting up test: %s", storeErr)
	}
	filename := "test"
	expected := []byte(filename)
	putErr := store.Put(test.GoodReadCloser(expected), filename)
	if putErr != nil {
		t.Fatal(putErr)
	}
	actual, readErr := ioutil.ReadFile(path.Join(dir, filename))
	if readErr != nil {
		t.Fatalf("reading bytes written: %s", readErr)
	}
	if !bytes.Equal(expected, actual) {
		t.Fatalf("expected put file to contain %s, got %s", expected, actual)
	}
}

func TestPutBadReader(t *testing.T) {
	dir := test.TempDir()
	defer os.RemoveAll(dir)
	store, storeErr := New(dir)
	if storeErr != nil {
		t.Fatalf("failed setting up test: %s", storeErr)
	}
	putErr := store.Put(test.TimeoutReadCloser([]byte("test")), "test")
	if putErr == nil {
		t.Fatal("expected put error")
	}
}

func TestPutCannotCreateFile(t *testing.T) {
	dir := test.TempDir()
	defer os.RemoveAll(dir)
	store, storeErr := New(dir)
	if storeErr != nil {
		t.Fatalf("failed setting up test: %s", storeErr)
	}
	putErr := store.Put(test.TimeoutReadCloser([]byte("test")), path.Join("test", "file"))
	if putErr == nil {
		t.Fatal("expected put error")
	}
}

func TestGet(t *testing.T) {
	dir := test.TempDir()
	defer os.RemoveAll(dir)
	store, storeErr := New(dir)
	if storeErr != nil {
		t.Fatalf("failed setting up test: %s", storeErr)
	}
	filename := "test"
	expected := []byte("test")
	writeErr := ioutil.WriteFile(path.Join(dir, filename), expected, 0644)
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

func TestExists(t *testing.T) {
	dir := test.TempDir()
	defer os.RemoveAll(dir)
	store, storeErr := New(dir)
	if storeErr != nil {
		t.Fatalf("failed setting up test: %s", storeErr)
	}
	filename := "test"
	writeErr := ioutil.WriteFile(path.Join(dir, filename), []byte("test"), 0644)
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
