// Package localdiskstore is a memorybox.Store compatible abstraction over the
// golang standard library for os-agnostic path resolution and disk io. Context
// values are ignored in this package because no goroutines are used in any of
// the methods.
package localdiskstore

import (
	"context"
	"fmt"
	"github.com/mitchellh/go-homedir"
	"io"
	"os"
	"path/filepath"
)

// Store implements store.Store backed by local disk.
type Store struct {
	RootPath string
}

// New returns a reference to a Store instance.
func New(rootPath string) *Store {
	expanded, _ := homedir.Expand(rootPath)
	return &Store{RootPath: expanded}
}

// NewFromConfig instantiates a Store using configuration values that were
// likely sourced from a configuration file target.
func NewFromConfig(config map[string]string) *Store {
	return New(config["path"])
}

// String returns a human friendly representation of the Store.
func (s *Store) String() string {
	return fmt.Sprintf("LocalDiskStore: %s", s.RootPath)
}

// Put writes the content of an io.Reader to local disk, naming the file with
// a hash of its contents.
func (s *Store) Put(_ context.Context, source io.Reader, hash string) error {
	if err := os.MkdirAll(s.RootPath, 0755); err != nil {
		return fmt.Errorf("could not create %s: %w", s.RootPath, err)
	}
	fullPath := filepath.Join(s.RootPath, hash)
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
func (s *Store) Get(_ context.Context, request string) (io.ReadCloser, error) {
	return os.Open(filepath.Join(s.RootPath, request))
}

// Search finds matching files in storage by prefix.
func (s *Store) Search(_ context.Context, search string) ([]string, error) {
	var matches []string
	results, err := filepath.Glob(filepath.Join(s.RootPath, search+"*"))
	if err != nil {
		return nil, fmt.Errorf("local store search: %s", err)
	}
	for _, entry := range results {
		matches = append(matches, filepath.Base(entry))
	}
	return matches, nil
}

// Exists determines if an object exists in the local store already.
func (s *Store) Exists(_ context.Context, search string) bool {
	_, err := os.Stat(filepath.Join(s.RootPath, search))
	return err == nil
}
