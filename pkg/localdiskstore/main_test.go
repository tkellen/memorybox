// These are integration tests which validate the minimal domain specific error
// handling this store layers over the golang standard libraries for os-agnostic
// path resolution and disk io. Mocking out the filesystem for this (as seen in
// the fetch package) seemed like overkill.
package localdiskstore_test

import (
	"bytes"
	"context"
	"fmt"
	"github.com/tkellen/memorybox/internal/test"
	"github.com/tkellen/memorybox/pkg/localdiskstore"
	"io/ioutil"
	"os"
	"path"
	"testing"
	"time"
)

func TestStoreSuite(t *testing.T) {
	tempDir, tempErr := ioutil.TempDir("", "*")
	if tempErr != nil {
		t.Fatalf("test setup: %s", tempErr)
	}
	defer os.RemoveAll(tempDir)
	store := localdiskstore.New(tempDir)
	test.StoreSuite(t, store)
}

func TestNewFromConfig(t *testing.T) {
	expected := "test"
	actual := localdiskstore.NewFromConfig(map[string]string{
		"path": expected,
	})
	if expected != actual.RootPath {
		t.Fatalf("expected rootPath of %s, got %s", expected, actual.RootPath)
	}
}

func TestStore_String(t *testing.T) {
	store := localdiskstore.New("/")
	actual := store.String()
	expected := fmt.Sprintf("localDisk: %s", "/")
	if expected != actual {
		t.Fatalf("expected %s, got %s", expected, actual)
	}
}

func TestStore_Put_CannotCreateHome(t *testing.T) {
	file, tempErr := ioutil.TempFile("", "*")
	if tempErr != nil {
		t.Fatalf("failed setting up test: %s", tempErr)
	}
	defer os.Remove(file.Name())
	store := localdiskstore.New(file.Name())
	filename := "test"
	expected := []byte(filename)
	putErr := store.Put(context.Background(), bytes.NewReader(expected), filename, time.Now())
	if putErr == nil {
		t.Fatal("expected error creating path directory")
	}
}

func TestStore_Put_CannotCreateFile(t *testing.T) {
	tempDir, tempErr := ioutil.TempDir("", "*")
	if tempErr != nil {
		t.Fatalf("test setup: %s", tempErr)
	}
	defer os.RemoveAll(tempDir)
	store := localdiskstore.New(tempDir)
	putErr := store.Put(context.Background(), bytes.NewReader([]byte("test")), path.Join("test", "file"), time.Now())
	if putErr == nil {
		t.Fatal("expected put error")
	}
}
