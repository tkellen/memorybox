package memorybox

import (
	"fmt"
	"github.com/mitchellh/go-homedir"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// LocalStore implements Store backed by local disk.
type LocalStore struct {
	RootPath string
}

// NewLocalStore returns a reference to a LocalStore instance.
func NewLocalStore(rootPath string) (*LocalStore, error) {
	expandedPath, err := homedir.Expand(rootPath)
	if err != nil {
		return nil, fmt.Errorf("unable to expand home directory: %w", err)
	}
	if err := os.MkdirAll(expandedPath, 0755); err != nil {
		return nil, fmt.Errorf("could not create %s: %w", expandedPath, err)
	}
	return &LocalStore{RootPath: expandedPath}, nil
}

// String returns a human friendly representation of the Store.
func (s *LocalStore) String() string {
	return fmt.Sprintf("LocalStore: %s", s.RootPath)
}

// Put writes the content of an io.Reader to local disk, naming the file with
// a hash of its contents.
func (s *LocalStore) Put(src io.ReadCloser, hash string) error {
	fullPath := path.Join(s.RootPath, hash)
	file, err := os.Create(fullPath)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	if _, err := io.Copy(file, src); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	// Be absolutely sure the data has been persisted to disk.
	return file.Sync()
}

// Search finds matching files in storage by prefix.
func (s *LocalStore) Search(search string) ([]string, error) {
	var matches []string
	results, err := filepath.Glob(path.Join(s.RootPath, search) + "*")
	if err != nil {
		return nil, fmt.Errorf("local store search: %w", err)
	}
	for _, entry := range results {
		matches = append(matches, strings.TrimPrefix(entry, s.RootPath))
	}
	return matches, nil
}

// Get finds an object in storage by name and returns an io.Reader for it.
func (s *LocalStore) Get(request string) (io.ReadCloser, error) {
	file, err := os.Open(path.Join(s.RootPath, request))
	if err != nil {
		return nil, fmt.Errorf("local store get: %w", err)
	}
	// who owns closing this file?
	return file, nil
}

// Exists determines if a given file exists in the local store already.
func (s *LocalStore) Exists(search string) bool {
	_, err := os.Stat(path.Join(s.RootPath, search))
	return err == nil
}
