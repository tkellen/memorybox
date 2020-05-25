// Package localdiskstore is a archive.Store compatible abstraction over the
// golang standard library for os-agnostic path resolution and disk io.
package localdiskstore

import (
	"context"
	"fmt"
	"github.com/mitchellh/go-homedir"
	"github.com/tkellen/memorybox/pkg/file"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Store implements archive.Store backed by local disk.
type Store struct {
	RootPath string
}

// Name is used in the memorybox configuration file to determine which type of
// store to instantiate.
const Name = "localDisk"

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
	return fmt.Sprintf("%s: %s", Name, s.RootPath)
}

// Put writes the content of a supplied reader to local disk.
func (s *Store) Put(_ context.Context, source io.Reader, name string, lastModified time.Time) error {
	if err := os.MkdirAll(s.RootPath, 0755); err != nil {
		return fmt.Errorf("could not create %s: %w", s.RootPath, err)
	}
	fullPath := filepath.Join(s.RootPath, name)
	f, err := os.Create(fullPath)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	if _, err := io.Copy(f, source); err != nil {
		f.Close()
		os.Remove(f.Name())
		return fmt.Errorf("write file: %w", err)
	}
	defer f.Close()
	defer os.Chtimes(f.Name(), lastModified, lastModified)
	return f.Sync()
}

// Get finds an object in storage by name.
func (s *Store) Get(ctx context.Context, name string) (*file.File, error) {
	f, statErr := s.Stat(ctx, name)
	if statErr != nil {
		return nil, statErr
	}
	body, openErr := os.Open(filepath.Join(s.RootPath, name))
	if openErr != nil {
		return nil, openErr
	}
	f.Body = body
	return f, nil
}

// Delete removes an object in storage by name.
func (s *Store) Delete(_ context.Context, name string) error {
	return os.Remove(filepath.Join(s.RootPath, name))
}

// Search finds matching files in storage by prefix.
func (s *Store) Search(ctx context.Context, search string) (file.List, error) {
	var matches file.List
	results, err := filepath.Glob(filepath.Join(s.RootPath, search+"*"))
	if err != nil {
		return nil, fmt.Errorf("local store search: %s", err)
	}
	for _, entry := range results {
		if object, err := s.Stat(ctx, filepath.Base(entry)); err == nil {
			matches = append(matches, object)
		}
	}
	sort.Sort(matches)
	return matches, nil
}

// Concat an array of byte arrays ordered identically with the input files
// supplied. Note that this loads the entire dataset into memory.
func (s *Store) Concat(ctx context.Context, concurrency int, files []string) ([][]byte, error) {
	var err error
	result := make([][]byte, len(files))
	sem := semaphore.NewWeighted(int64(concurrency))
	eg, egCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		for index, item := range files {
			index, item := index, item // https://golang.org/doc/faq#closures_and_goroutines
			if err := sem.Acquire(egCtx, 1); err != nil {
				return err
			}
			eg.Go(func() error {
				defer sem.Release(1)
				result[index], err = ioutil.ReadFile(filepath.Join(s.RootPath, item))
				return err
			})
		}
		return nil
	})
	if err := eg.Wait(); err != nil {
		return nil, err
	}
	return result, nil
}

// Stat gets details about an object in the store.
func (s *Store) Stat(_ context.Context, search string) (*file.File, error) {
	stat, err := os.Stat(filepath.Join(s.RootPath, search))
	if err != nil {
		return nil, err
	}
	return file.NewStub(filepath.Base(search), stat.Size(), stat.ModTime()), nil
}
