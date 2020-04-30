package testingstore

import (
	"bytes"
	"context"
	"fmt"
	"github.com/tkellen/memorybox/pkg/archive"
	"io"
	"io/ioutil"
	"strings"
	"sync"
	"testing/iotest"
)

// Store is a in-memory implementation of Store for testing.
type Store struct {
	Data                    sync.Map
	GetErrorWith            error
	SearchErrorWith         error
	GetReturnsTimeoutReader bool
}

// Fixture defines a fixture.
type Fixture struct {
	Name    string
	Content []byte
}

// New returns a Store pre-filled with supplied fixtures.
func New(fixtures []*archive.File) *Store {
	store := &Store{
		Data: sync.Map{},
	}
	for _, fixture := range fixtures {
		data, _ := ioutil.ReadAll(fixture)
		store.Data.Store(fixture.Name(), data)
	}
	return store
}

// String returns a human friendly representation of the Store.
func (s *Store) String() string {
	return fmt.Sprintf("TestingStore")
}

// Put assigns the content of an io.Reader to a string keyed in-memory map using
// the hash as a key.
func (s *Store) Put(_ context.Context, source io.Reader, hash string) error {
	data, err := ioutil.ReadAll(source)
	if err != nil {
		return err
	}
	s.Data.Store(hash, data)
	return nil
}

// Search finds matching items in storage by prefix.
func (s *Store) Search(_ context.Context, search string) ([]string, error) {
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
func (s *Store) Get(_ context.Context, request string) (io.ReadCloser, error) {
	if s.GetErrorWith != nil {
		return nil, s.GetErrorWith
	}
	if data, ok := s.Data.Load(request); ok {
		if s.GetReturnsTimeoutReader {
			return ioutil.NopCloser(iotest.TimeoutReader(bytes.NewReader(data.([]byte)))), nil
		}
		return ioutil.NopCloser(bytes.NewReader(data.([]byte))), nil
	}
	return nil, fmt.Errorf("not found")
}

// Exists determines if a requested object exists in the Store.
func (s *Store) Exists(_ context.Context, request string) bool {
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
