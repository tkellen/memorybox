package localstore

import (
	"fmt"
	"io"
	"os"
	"path"
)

// Store implements memorybox.Store backed by local disk.
type Store struct {
	RootPath string
}

// New returns a reference to a Store instance.
func New(rootPath string) (*Store, error) {
	if err := os.MkdirAll(rootPath, 0755); err != nil {
		return nil, fmt.Errorf("could not create %s: %w", rootPath, err)
	}
	return &Store{RootPath: rootPath}, nil
}

// String returns a human friendly representation of the Store.
func (s *Store) String() string {
	return fmt.Sprintf("LocalStore: %s", s.RootPath)
}

// Put writes the content of an io.Reader to local disk, naming the file with
// a hash of its contents.
func (s *Store) Put(src io.ReadCloser, hash string) error {
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

// Get finds an object in storage by name and returns an io.Reader for it.
func (s *Store) Get(request string) (io.ReadCloser, error) {
	return os.Open(path.Join(s.RootPath, request))
}

// Exists determines if a given file exists in the local store already.
func (s *Store) Exists(search string) bool {
	_, err := os.Stat(path.Join(s.RootPath, search))
	return err == nil
}
