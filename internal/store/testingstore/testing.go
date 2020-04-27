package testingstore

import (
	"bytes"
	"fmt"
	"github.com/tkellen/memorybox/internal/archive"
	"io"
	"io/ioutil"
	"strings"
)

// Store is a in-memory implementation of Store for testing.
type Store struct {
	Data                    map[string][]byte
	GetErrorWith            error
	SearchErrorWith         error
	GetReturnsTimeoutReader bool
}

// Fixture defines a fixture.
type Fixture struct {
	Name    string
	Content []byte
}

// NewFixture generates a content-hashed fixture for testing.
func NewFixture(content string, isMeta bool, hashFn func(source io.Reader) (string, int64, error)) Fixture {
	contentAsBytes := []byte(content)
	name, _, _ := hashFn(bytes.NewBuffer(contentAsBytes))
	if isMeta {
		name = archive.MetaFileName(name)
		f, _ := archive.NewFromReader(hashFn, ioutil.NopCloser(bytes.NewReader(contentAsBytes)))
		defer f.Close()
		contentAsBytes, _ = ioutil.ReadAll(archive.NewMetaFile(f))
	}
	return Fixture{
		Name:    name,
		Content: contentAsBytes,
	}
}

// New returns a Store pre-filled with supplied fixtures.
func New(fixtures []Fixture) *Store {
	store := &Store{
		Data: map[string][]byte{},
	}
	for _, fixture := range fixtures {
		store.Data[fixture.Name] = fixture.Content
	}
	return store
}

// String returns a human friendly representation of the Store.
func (s *Store) String() string {
	return fmt.Sprintf("TestingStore")
}

// Put assigns the content of an io.Reader to a string keyed in-memory map using
// the hash as a key.
func (s *Store) Put(source io.Reader, hash string) error {
	data, err := ioutil.ReadAll(source)
	if err != nil {
		return err
	}
	s.Data[hash] = data
	return nil
}

// Search finds matching items in storage by prefix.
func (s *Store) Search(search string) ([]string, error) {
	if s.SearchErrorWith != nil {
		return nil, s.SearchErrorWith
	}
	var matches []string
	for key := range s.Data {
		if strings.HasPrefix(key, search) {
			matches = append(matches, key)
		}
	}
	return matches, nil
}

// Get finds an object in storage by name and returns an io.ReadCloser for it.
func (s *Store) Get(request string) (io.ReadCloser, error) {
	if s.GetErrorWith != nil {
		return nil, s.GetErrorWith
	}
	if data, ok := s.Data[request]; ok {
		return ioutil.NopCloser(bytes.NewReader(data)), nil
	}
	return nil, fmt.Errorf("not found")
}

// Exists determines if a requested object exists in the Store.
func (s *Store) Exists(request string) bool {
	return s.Data[request] != nil
}
