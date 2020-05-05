package testingstore_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/mattetti/filebuffer"
	"github.com/tkellen/memorybox/internal/testingstore"
	"github.com/tkellen/memorybox/pkg/archive"
	"io/ioutil"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"testing/iotest"
)

func TestIdentityHash(t *testing.T) {
	input := []byte("test")
	expected := "test-identity"
	expectedSize := int64(len(input))
	actual, actualSize, goodErr := testingstore.IdentityHash(bytes.NewReader(input))
	if goodErr != nil {
		t.Fatal(goodErr)
	}
	if expected != actual {
		t.Fatalf("expected %s, got %s", expected, actual)
	}
	if expectedSize != actualSize {
		t.Fatalf("expected size %d, got %d", expectedSize, actualSize)
	}
	_, _, err := testingstore.IdentityHash(iotest.TimeoutReader(bytes.NewReader([]byte("testing12341234"))))
	if err == nil {
		t.Fatal("expected error on bad reader")
	}
}

func TestStore_String(t *testing.T) {
	store := testingstore.New([]*archive.File{})
	actual := store.String()
	expected := fmt.Sprintf("TestingStore")
	if expected != actual {
		t.Fatalf("expected %s, got %s", expected, actual)
	}
}

func TestStore_Put(t *testing.T) {
	store := testingstore.New([]*archive.File{})
	filename := "test"
	expected := []byte(filename)
	putErr := store.Put(context.Background(), bytes.NewReader(expected), filename)
	if putErr != nil {
		t.Fatal(putErr)
	}
	if actual, ok := store.Data.Load(filename); ok {
		if !bytes.Equal(expected, actual.([]byte)) {
			t.Fatalf("expected put file to contain %s, got %s", expected, actual)
		}
	} else {
		t.Fatal("expected item to be in store")
	}
}

func TestStore_Put_BadReader(t *testing.T) {
	store := testingstore.New([]*archive.File{})
	putErr := store.Put(context.Background(), iotest.TimeoutReader(bytes.NewReader([]byte("test"))), "test")
	if putErr == nil {
		t.Fatal("expected put error")
	}
}

func TestStore_Get(t *testing.T) {
	expectedContent := []byte("test")
	fixture, _ := archive.New("fixture", filebuffer.New(expectedContent), archive.Sha256)
	store := testingstore.New([]*archive.File{fixture})
	data, getErr := store.Get(context.Background(), fixture.Name())
	defer data.Close()
	if getErr != nil {
		t.Fatal(getErr)
	}
	actual, readErr := ioutil.ReadAll(data)
	if readErr != nil {
		t.Fatalf("failed reading response: %s", readErr)
	}
	if !bytes.Equal(expectedContent, actual) {
		t.Fatalf("expected get to contain %s, got %s", expectedContent, actual)
	}
}

func TestStore_GetMissing(t *testing.T) {
	store := testingstore.New([]*archive.File{})
	_, err := store.Get(context.Background(), "anything")
	if err == nil {
		t.Fatal("expected error on missing")
	}
}

func TestStore_Exists(t *testing.T) {
	fixture, _ := archive.New("fixture", filebuffer.New([]byte("test")), testingstore.IdentityHash)
	store := testingstore.New([]*archive.File{fixture})
	if !store.Exists(context.Background(), fixture.Name()) {
		t.Fatal("expected boolean true for file that exists")
	}
	if store.Exists(context.Background(), "nope") {
		t.Fatal("expected boolean false for file that does not exist")
	}
}

func TestStore_Search(t *testing.T) {
	var fixtures []*archive.File
	for _, fixture := range []string{"foo", "bar", "baz"} {
		fixture, _ := archive.New("fixture", filebuffer.New([]byte(fixture)), testingstore.IdentityHash)
		fixtures = append(fixtures, fixture)
	}
	store := testingstore.New(fixtures)
	for _, fixture := range fixtures {
		if err := store.Put(context.Background(), fixture, fixture.Name()); err != nil {
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
			expectedMatches: []string{"bar-identity", "baz-identity"},
			expectedErr:     nil,
		},
		"one match": {
			search:          "f",
			expectedMatches: []string{"foo-identity"},
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
			actualMatches, err := store.Search(context.Background(), test.search)
			if err != nil && test.expectedErr == nil {
				t.Fatal(err)
			}
			if err != nil && test.expectedErr != nil && !errors.Is(err, test.expectedErr) && !strings.Contains(err.Error(), test.expectedErr.Error()) {
				t.Fatalf("expected error: %s, got %s", test.expectedErr, err)
			}
			if err == nil {
				sort.Strings(actualMatches)
				for index, match := range actualMatches {
					if match != test.expectedMatches[index] {
						t.Fatalf("expected %s for match, got %s", test.expectedMatches[index], match)
					}
				}
			}
		})
	}
}
