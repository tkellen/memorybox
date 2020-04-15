package localdiskstore

import (
	"fmt"
	"github.com/mitchellh/go-homedir"
	"io"
	"os"
	"path"
)

// Store implements memorybox.Store backed by local disk.
type Store struct {
	RootPath string
}

// New returns a reference to a Store instance.
func New(rootPath string) *Store {
	expanded, _ := homedir.Expand(rootPath)
	return &Store{RootPath: expanded}
}

// NewFromTarget instantiates a Store using configuration values that were
// likely sourced from a configuration file target.
func NewFromTarget(config map[string]string) *Store {
	return New(config["home"])
}

// String returns a human friendly representation of the Store.
func (s *Store) String() string {
	return fmt.Sprintf("LocalDiskStore: %s", s.RootPath)
}

// Put writes the content of an io.Reader to local disk, naming the file with
// a hash of its contents.
func (s *Store) Put(source io.ReadCloser, hash string) error {
	if err := os.MkdirAll(s.RootPath, 0755); err != nil {
		return fmt.Errorf("could not create %s: %w", s.RootPath, err)
	}
	defer source.Close()
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

// Get finds an object in storage by name and returns an io.Reader for it.
func (s *Store) Get(request string) (io.ReadCloser, error) {
	return os.Open(path.Join(s.RootPath, request))
}

// Exists determines if a given file exists in the local store already.
func (s *Store) Exists(search string) bool {
	_, err := os.Stat(path.Join(s.RootPath, search))
	return err == nil
}
