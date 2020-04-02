package main

import (
	"fmt"
	"github.com/mitchellh/go-homedir"
	"io"
	"os"
	"path"
)

// LocalStore is a Store implementation that uses local disk.
type LocalStore struct {
	RootPath string
}

// NewLocalStore returns a reference to a LocalStore instance that, by default,
// writes to ~/memorybox.
// TODO: allow this to be configured with a command line flag.
func NewLocalStore(root string) (*LocalStore, error) {
	rootPath, err := homedir.Expand(root)
	if err != nil {
		return nil, fmt.Errorf("unable to locate home directory: %s", err)
	}
	err = os.MkdirAll(rootPath, 0755)
	if err != nil {
		return nil, fmt.Errorf("could not create %s: %s", rootPath, err)
	}
	return &LocalStore{RootPath: rootPath}, nil
}

func (s *LocalStore) Root() string {
	return s.RootPath
}

// Save writes the content of the io.Reader to the destination on disk.
func (s *LocalStore) Save(src io.Reader, destPath string) error {
	file, err := os.Create(path.Join(s.RootPath, destPath))
	if err != nil {
		return fmt.Errorf("local store failed: %s", err)
	}
	// Copy the file into place, returning an error if any.
	if _, err := io.Copy(file, src); err != nil {
		return fmt.Errorf("local store copy failed: %s", err)
	}
	return nil
}

// Index copies a temporary file sent to the store to its final location.
// TODO: perform indexing operations here
func (s *LocalStore) Index(temp string, hash string) error {
	return os.Rename(path.Join(s.RootPath, temp), path.Join(s.RootPath, hash))
}
