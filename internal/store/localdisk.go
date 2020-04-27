package store

import (
	"fmt"
	"github.com/mitchellh/go-homedir"
	"io"
	"os"
	"path"
	"path/filepath"
)

// LocalDiskStore implements store.Store backed by local disk.
type LocalDiskStore struct {
	RootPath string
}

// NewLocalDiskStore returns a reference to a Store instance.
func NewLocalDiskStore(rootPath string) *LocalDiskStore {
	expanded, _ := homedir.Expand(rootPath)
	return &LocalDiskStore{RootPath: expanded}
}

// NewLocalDiskStoreFromConfig instantiates a Store using configuration values that were
// likely sourced from a configuration file target.
func NewLocalDiskStoreFromConfig(config map[string]string) *LocalDiskStore {
	return NewLocalDiskStore(config["home"])
}

// String returns a human friendly representation of the Store.
func (s *LocalDiskStore) String() string {
	return fmt.Sprintf("LocalDiskStore: %s", s.RootPath)
}

// Put writes the content of an io.Reader to local disk, naming the file with
// a hash of its contents.
func (s *LocalDiskStore) Put(source io.Reader, hash string) error {
	if err := os.MkdirAll(s.RootPath, 0755); err != nil {
		return fmt.Errorf("could not create %s: %w", s.RootPath, err)
	}
	fullPath := path.Join(s.RootPath, hash)
	file, err := os.Create(fullPath)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	if _, err := io.Copy(file, source); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	// Be absolutely sure the data has been persisted to disk.
	return file.Sync()
}

// Get finds an object in storage by name and returns an io.ReadCloser for it.
func (s *LocalDiskStore) Get(request string) (io.ReadCloser, error) {
	return os.Open(path.Join(s.RootPath, request))
}

// Search finds matching files in storage by prefix.
func (s *LocalDiskStore) Search(search string) ([]string, error) {
	matches := []string{}
	results, err := filepath.Glob(path.Join(s.RootPath, search) + "*")
	if err != nil {
		return nil, fmt.Errorf("local store search: %s", err)
	}
	for _, entry := range results {
		matches = append(matches, path.Base(entry))
	}
	return matches, nil
}

// Exists determines if an object exists in the local store already.
func (s *LocalDiskStore) Exists(search string) bool {
	_, err := os.Stat(path.Join(s.RootPath, search))
	return err == nil
}
