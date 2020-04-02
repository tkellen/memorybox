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
func (s *LocalStore) Save(src io.Reader, temp string, filename func() string) error {
	tempPath := path.Join(s.RootPath, temp)
	file, err := os.Create(tempPath)
	if err != nil {
		return fmt.Errorf("local store failed: %s", err)
	}
	if _, err := io.Copy(file, src); err != nil {
		return fmt.Errorf("local store copy failed: %s", err)
	}
	destPath := path.Join(s.RootPath, filename()) // must be called _after_ copy
	if err := os.Rename(tempPath, destPath); err != nil {
		return fmt.Errorf("local store rename from %s to %s failed: %s", tempPath, destPath, err)
	}
	return nil
}
