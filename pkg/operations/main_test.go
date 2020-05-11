package operations_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/mattetti/filebuffer"
	"github.com/tkellen/memorybox/pkg/archive"
	"github.com/tkellen/memorybox/pkg/operations"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"testing/iotest"
)

func fixtureData() []*archive.File {
	dataFile, _ := archive.NewSha256("fixture", filebuffer.New([]byte("test")))
	metaFile := dataFile.MetaFile()
	return []*archive.File{dataFile, metaFile}
}

func fixtureServer(t *testing.T, inputs [][]byte) ([]string, func() error) {
	listen, listenErr := net.Listen("tcp", "127.0.0.1:0")
	if listenErr != nil {
		t.Fatal(listenErr)
	}
	var urls []string
	for _, content := range inputs {
		urls = append(urls, fmt.Sprintf("http://%s/%s", listen.Addr().String(), content))
	}
	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for _, content := range inputs {
				if string(content) == path.Base(r.URL.Path) {
					w.WriteHeader(http.StatusOK)
					w.Write(content)
					return
				}
			}
			w.WriteHeader(http.StatusNotFound)
		}),
	}
	go server.Serve(listen)
	return urls, server.Close
}

func discardLogger() *operations.Logger {
	return &operations.Logger{
		Stdout:  log.New(ioutil.Discard, "", 0),
		Verbose: log.New(ioutil.Discard, "", 0),
		Stderr:  log.New(ioutil.Discard, "", 0),
	}
}

// TestingStore is a in-memory implementation of Store to ease testing.
type TestingStore struct {
	Data                   sync.Map
	GetErrorWith           error
	SearchErrorWith        error
	GetReturnsClosedReader bool
}

// Fixture defines a fixture.
type Fixture struct {
	Name    string
	Content []byte
}

// IdentityHash is a noop hashing function for testing that returns a string
// value of the input (assumes ASCII input).
func IdentityHash(source io.Reader) (string, int64, error) {
	bytes, err := ioutil.ReadAll(source)
	if err != nil {
		return "", 0, err
	}
	return string(bytes) + "-identity", int64(len(bytes)), nil
}

// NewTestingStore returns a TestingStore pre-filled with supplied fixtures.
func NewTestingStore(fixtures []*archive.File) *TestingStore {
	store := &TestingStore{
		Data: sync.Map{},
	}
	for _, fixture := range fixtures {
		data, _ := ioutil.ReadAll(fixture)
		store.Data.Store(fixture.Name(), data)
	}
	return store
}

// String returns a human friendly representation of the TestingStore.
func (s *TestingStore) String() string {
	return fmt.Sprintf("TestingStore")
}

// Put assigns the content of an io.Reader to a string keyed in-memory map using
// the hash as a key.
func (s *TestingStore) Put(_ context.Context, source io.Reader, hash string) error {
	data, err := ioutil.ReadAll(source)
	if err != nil {
		return err
	}
	s.Data.Store(hash, data)
	return nil
}

// Search finds matching items in storage by prefix.
func (s *TestingStore) Search(_ context.Context, search string) ([]string, error) {
	if s.SearchErrorWith != nil {
		return nil, s.SearchErrorWith
	}
	var matches []string
	s.Data.Range(func(key interface{}, value interface{}) bool {
		if strings.HasPrefix(key.(string), search) {
			matches = append(matches, key.(string))
		}
		return true
	})
	return matches, nil
}

// Get finds an object in storage by name and returns an io.ReadCloser for it.
func (s *TestingStore) Get(_ context.Context, request string) (io.ReadCloser, error) {
	if s.GetErrorWith != nil {
		return nil, s.GetErrorWith
	}
	if data, ok := s.Data.Load(request); ok {
		file := filebuffer.New(data.([]byte))
		if s.GetReturnsClosedReader {
			file.Close()
		}
		return file, nil
	}
	return nil, fmt.Errorf("not in store")
}

// Exists determines if a requested object exists in the TestingStore.
func (s *TestingStore) Exists(_ context.Context, request string) bool {
	exists := false
	s.Data.Range(func(key interface{}, value interface{}) bool {
		if key.(string) == request {
			exists = true
			return false
		}
		return true
	})
	return exists
}

func TestIdentityHash(t *testing.T) {
	input := []byte("test")
	expected := "test-identity"
	expectedSize := int64(len(input))
	actual, actualSize, goodErr := IdentityHash(bytes.NewReader(input))
	if goodErr != nil {
		t.Fatal(goodErr)
	}
	if expected != actual {
		t.Fatalf("expected %s, got %s", expected, actual)
	}
	if expectedSize != actualSize {
		t.Fatalf("expected size %d, got %d", expectedSize, actualSize)
	}
	_, _, err := IdentityHash(iotest.TimeoutReader(bytes.NewReader([]byte("testing12341234"))))
	if err == nil {
		t.Fatal("expected error on bad reader")
	}
}

func TestStore_String(t *testing.T) {
	store := NewTestingStore([]*archive.File{})
	actual := store.String()
	expected := fmt.Sprintf("TestingStore")
	if expected != actual {
		t.Fatalf("expected %s, got %s", expected, actual)
	}
}

func TestStore_Put(t *testing.T) {
	store := NewTestingStore([]*archive.File{})
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
	store := NewTestingStore([]*archive.File{})
	putErr := store.Put(context.Background(), iotest.TimeoutReader(bytes.NewReader([]byte("test"))), "test")
	if putErr == nil {
		t.Fatal("expected put error")
	}
}

func TestStore_Get(t *testing.T) {
	expectedContent := []byte("test")
	fixture, _ := archive.New("fixture", filebuffer.New(expectedContent), archive.Sha256)
	store := NewTestingStore([]*archive.File{fixture})
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
	store := NewTestingStore([]*archive.File{})
	_, err := store.Get(context.Background(), "anything")
	if err == nil {
		t.Fatal("expected error on missing")
	}
}

func TestStore_Exists(t *testing.T) {
	fixture, _ := archive.New("fixture", filebuffer.New([]byte("test")), IdentityHash)
	store := NewTestingStore([]*archive.File{fixture})
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
		fixture, _ := archive.New("fixture", filebuffer.New([]byte(fixture)), IdentityHash)
		fixtures = append(fixtures, fixture)
	}
	store := NewTestingStore(fixtures)
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
