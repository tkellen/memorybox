package test

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"strings"
)

// Store is a in-memory mock implementation for testing.
// Better than using a mocking library? ¯\_(ツ)_/¯.
type Store struct {
	Data          map[string][]byte
	ForceGetError error
}

// String returns a human friendly representation of the Store.
func (s *Store) String() string {
	return fmt.Sprintf("Store")
}

// Put assigns the content of an io.Reader to a string keyed in-memory map using
// the hash as a key.
func (s *Store) Put(source io.ReadCloser, hash string) error {
	defer source.Close()
	data, err := ioutil.ReadAll(source)
	if err != nil {
		return err
	}
	s.Data[hash] = data
	return nil
}

// Search finds matching items in storage by prefix.
func (s *Store) Search(search string) ([]string, error) {
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
	if s.ForceGetError != nil {
		return nil, s.ForceGetError
	}
	data := s.Data[request]
	if data != nil {
		return ioutil.NopCloser(bytes.NewReader(data)), nil
	}
	return nil, fmt.Errorf("not found")
}

// Exists determines if a requested object exists in the Store.
func (s *Store) Exists(request string) bool {
	return s.Data[request] != nil
}