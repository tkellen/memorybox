package main

import (
	"fmt"
	"github.com/mitchellh/go-homedir"
	"io"
	"os"
	"path"
)

// LocalStore implements Store backed by local disk.
type LocalStore struct {
	RootPath string
}

// NewLocalStore returns a reference to a LocalStore instance.
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

// Save writes the content of an io.Reader to local disk, naming the file with
// a hash of its contents.
func (s *LocalStore) Save(src io.Reader, hash string) error {
	destPath := path.Join(s.RootPath, hash)
	file, err := os.Create(destPath)
	defer file.Close()
	if err != nil {
		return fmt.Errorf("local store failed to create file: %s", err)
	}
	if _, err := io.Copy(file, src); err != nil {
		return fmt.Errorf("local store failed to save file: %s", err)
	}
	return nil
}

// Exists determines if a given file exists in the local store already.
func (s *LocalStore) Exists(filepath string) bool {
	_, err := os.Stat(path.Join(s.RootPath, filepath))
	if err != nil {
		return false
	}
	return true
}