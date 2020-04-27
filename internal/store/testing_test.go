// These are integration tests which validate the minimal domain specific error
// handling this store layers over the golang standard libraries for os-agnostic
// path resolution and disk io. Mocking out the filesystem for this (as seen in
// the archive package) seemed like overkill.
package store_test

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/tkellen/memorybox/internal/store"
	"io"
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"
	"testing/iotest"
)

func TestTestingStore_String(t *testing.T) {
	store := store.NewTestingStore([]store.TestingStoreFixture{})
	actual := store.String()
	expected := fmt.Sprintf("TestingStore")
	if expected != actual {
		t.Fatalf("expected %s, got %s", expected, actual)
	}
}

func TestTestingStore_Put(t *testing.T) {
	store := store.NewTestingStore([]store.TestingStoreFixture{})
	filename := "test"
	expected := []byte(filename)
	putErr := store.Put(bytes.NewReader(expected), filename)
	if putErr != nil {
		t.Fatal(putErr)
	}
	actual := store.Data[filename]
	if !bytes.Equal(expected, actual) {
		t.Fatalf("expected put file to contain %s, got %s", expected, actual)
	}
}

func TestTestingStore_Put_BadReader(t *testing.T) {
	store := store.NewTestingStore([]store.TestingStoreFixture{})
	putErr := store.Put(iotest.TimeoutReader(bytes.NewReader([]byte("test"))), "test")
	if putErr == nil {
		t.Fatal("expected put error")
	}
}

func TestTestingStore_Get(t *testing.T) {
	fixture := store.TestingStoreFixture{
		Name:    "test",
		Content: []byte("test"),
	}
	store := store.NewTestingStore([]store.TestingStoreFixture{fixture})
	data, getErr := store.Get(fixture.Name)
	defer data.Close()
	if getErr != nil {
		t.Fatal(getErr)
	}
	actual, readErr := ioutil.ReadAll(data)
	if readErr != nil {
		t.Fatalf("failed reading response: %s", readErr)
	}
	if !bytes.Equal(fixture.Content, actual) {
		t.Fatalf("expected get to contain %s, got %s", fixture.Content, actual)
	}
}

func TestTestingStore_GetMissing(t *testing.T) {
	store := store.NewTestingStore([]store.TestingStoreFixture{})
	_, err := store.Get("anything")
	if err == nil {
		t.Fatal("expected error on missing")
	}
}

func TestTestingStore_Exists(t *testing.T) {
	fixture := store.TestingStoreFixture{
		Name:    "test",
		Content: []byte("test"),
	}
	store := store.NewTestingStore([]store.TestingStoreFixture{fixture})
	if !store.Exists(fixture.Name) {
		t.Fatal("expected boolean true for file that exists")
	}
	if store.Exists("nope") {
		t.Fatal("expected boolean false for file that does not exist")
	}
}

func TestTestingStore_Search(t *testing.T) {
	fixtures := []store.TestingStoreFixture{
		{Name: "foo", Content: []byte("foo")},
		{Name: "bar", Content: []byte("baz")},
		{Name: "bar", Content: []byte("baz")},
	}
	store := store.NewTestingStore(fixtures)
	reader := func(content []byte) io.ReadCloser {
		return ioutil.NopCloser(bytes.NewReader(content))
	}
	for _, fixture := range fixtures {
		if err := store.Put(reader(fixture.Content), fixture.Name); err != nil {
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
				for index, match := range actualMatches {
					if match != test.expectedMatches[index] {
						t.Fatalf("expected %s for match, got %s", test.expectedMatches[index], match)
					}
				}
			}
		})
	}
}
