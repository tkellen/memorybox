// These are integration tests which validate the minimal domain specific error
// handling this store layers over the golang standard libraries for os-agnostic
// path resolution and disk io. Mocking out the filesystem for this (as seen in
// the archive package) seemed like overkill.
package localdiskstore_test

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/tkellen/memorybox/internal/store/localdiskstore"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"testing/iotest"
)

func TestNewFromConfig(t *testing.T) {
	expected := "test"
	actual := localdiskstore.NewFromConfig(map[string]string{
		"home": expected,
	})
	if expected != actual.RootPath {
		t.Fatalf("expected rootPath of %s, got %s", expected, actual.RootPath)
	}
}

func TestStore_String(t *testing.T) {
	store := localdiskstore.New("/")
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
	store := localdiskstore.New(tempDir)
	filename := "test"
	expected := []byte(filename)
	putErr := store.Put(bytes.NewReader(expected), filename)
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
	store := localdiskstore.New(file.Name())
	filename := "test"
	expected := []byte(filename)
	putErr := store.Put(bytes.NewReader(expected), filename)
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
	store := localdiskstore.New(tempDir)
	putErr := store.Put(iotest.TimeoutReader(bytes.NewReader([]byte("test"))), "test")
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
	store := localdiskstore.New(tempDir)
	putErr := store.Put(iotest.TimeoutReader(bytes.NewReader([]byte("test"))), path.Join("test", "file"))
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
	store := localdiskstore.New(tempDir)
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
	store := localdiskstore.New(tempDir)
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

func TestStore_Search(t *testing.T) {
	tempDir, tempErr := ioutil.TempDir("", "*")
	if tempErr != nil {
		t.Fatalf("test setup: %s", tempErr)
	}
	defer os.RemoveAll(tempDir)
	store := localdiskstore.New(tempDir)
	expectedFiles := []string{"foo", "bar", "baz"}
	reader := func(content string) io.ReadCloser {
		return ioutil.NopCloser(bytes.NewReader([]byte(content)))
	}
	for _, file := range expectedFiles {
		if err := store.Put(reader(file), file); err != nil {
			t.Fatalf("test setup: %s", err)
		}
	}
	table := map[string]struct {
		search          string
		expectedMatches []string
		expectedErr     error
	}{
		"multiple matches": {
			search:          "b",
			expectedMatches: []string{"bar", "baz"},
			expectedErr:     nil,
		},
		"one match": {
			search:          "f",
			expectedMatches: []string{"foo"},
			expectedErr:     nil,
		},
		"no matches": {
			search:          "nope",
			expectedMatches: []string{},
			expectedErr:     nil,
		},
		"failure due to bad globbing pattern": {
			search:          "[]a]",
			expectedMatches: []string{},
			expectedErr:     filepath.ErrBadPattern,
		},
	}
	for name, test := range table {
		test := test
		t.Run(name, func(t *testing.T) {
			actualMatches, err := store.Search(test.search)
			if err != nil && test.expectedErr == nil {
				t.Fatal(err)
			}
			if err != nil && test.expectedErr != nil && !errors.Is(err, test.expectedErr) && !strings.Contains(err.Error(), test.expectedErr.Error()) {
				t.Fatalf("expected error: %s, got %s", test.expectedErr, err)
			}
			if err == nil {
				// not comparing the entire matches because windows returns
				// full paths for file globs.
				for index, match := range actualMatches {
					if filepath.Base(match) != filepath.Base(test.expectedMatches[index]) {
						t.Fatalf("expected %s for match, got %s", test.expectedMatches[index], match)
					}
				}
			}
		})
	}
}
