// Package meta is a high level abstraction over store.Store for managing
// metadata about objects. It is designed to execute operations that are
// supported by the cli.
package meta

import (
	"fmt"
	"github.com/tkellen/memorybox/pkg/file"
	"github.com/tkellen/memorybox/pkg/store"
	"io"
)

// Commands defines the interface needed to expose this functionality in a way
// that can be mocked for testing by consumers.
type Commands interface {
	Main(store.Store, string, io.Writer) error
	Set(store.Store, string, string, interface{}) error
	Delete(store.Store, string, string) error
}

// Command implements Commands as the public API of this package.
type Command struct{}

// Main gets a metadata object from a defined store.
func (Command) Main(store store.Store, hash string, sink io.Writer) error {
	metaFile, findErr := findMeta(store, hash)
	if findErr != nil {
		return findErr
	}
	defer metaFile.Close()
	if _, err := io.Copy(sink, metaFile); err != nil {
		return err
	}
	return nil
}

// Set adds a key to a metadata file and persists it to the store.
func (Command) Set(store store.Store, search string, key string, value interface{}) error {
	metaFile, findErr := findMeta(store, search)
	if findErr != nil {
		return findErr
	}
	metaFile.SetMeta(key, value)
	return store.Put(metaFile, metaFile.Name())
}

// Delete removes a key from a metadata file and persists it to the store.
func (Command) Delete(store store.Store, search string, key string) error {
	metaFile, findErr := findMeta(store, search)
	if findErr != nil {
		return findErr
	}
	metaFile.DeleteMeta(key)
	return store.Put(metaFile, metaFile.Name())
}

func findMeta(store store.Store, search string) (*file.File, error) {
	// First determine if the object to annotate even exists.
	matches, searchErr := store.Search(search)
	if searchErr != nil {
		return nil, fmt.Errorf("get: %s", searchErr)
	}
	if len(matches) != 1 {
		return nil, fmt.Errorf("%d objects matched", len(matches))
	}
	// If there is exactly one match, try to fetch it.
	reader, getErr := store.Get(file.MetaFileName(matches[0]))
	if getErr != nil {
		return nil, getErr
	}
	return file.New(reader)
}
