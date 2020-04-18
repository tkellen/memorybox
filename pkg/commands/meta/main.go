// Package meta is a high level abstraction over store.Store for managing
// metadata about objects. It is designed to execute operations that are
// supported by the cli.
package meta

import (
	"fmt"
	"github.com/tkellen/memorybox/pkg/metadata"
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
	reader, _, lookupErr := getReader(store, hash)
	if lookupErr != nil {
		return lookupErr
	}
	if _, err := io.Copy(sink, reader); err != nil {
		return err
	}
	return nil
}

// Set adds a key to a metadata file.
func (c Command) Set(store store.Store, hash string, key string, value interface{}) error {
	meta, name, lookupErr := get(store, hash)
	if lookupErr != nil {
		return lookupErr
	}
	meta.Set(key, value)
	reader, readerErr := meta.ToReader()
	if readerErr != nil {
		return readerErr
	}
	return store.Put(reader, name)
}

// Delete remove a key from a metadata file.
func (Command) Delete(store store.Store, hash string, key string) error {
	meta, name, lookupErr := get(store, hash)
	if lookupErr != nil {
		return lookupErr
	}
	meta.Delete(key)
	reader, readerErr := meta.ToReader()
	if readerErr != nil {
		return readerErr
	}
	return store.Put(reader, name)
}

func getReader(store store.Store, search string) (io.ReadCloser, string, error) {
	// First determine if the object to annotate even exists.
	matches, searchErr := store.Search(search)
	if searchErr != nil {
		return nil, "", fmt.Errorf("get: %s", searchErr)
	}
	if len(matches) != 1 {
		return nil, "", fmt.Errorf("%d objects matched", len(matches))
	}
	// If there is exactly one match, calculate the name of the meta object.
	metaObjectName := "meta-" + matches[0]
	// Try to fetch an existing meta object.
	reader, err := store.Get(metaObjectName)
	// Return whatever we found.
	return reader, metaObjectName, err
}

func get(store store.Store, search string) (*metadata.Metadata, string, error) {
	meta := &metadata.Metadata{}
	reader, name, err := getReader(store, search)
	// If there is existing metadata, attempt to decode it.
	if err == nil {
		defer reader.Close()
		if meta, err = metadata.NewFromReader(reader); err != nil {
			// If decoding failed, report the failure so the user can decide
			// what to do with the potentially malformed file.
			return nil, "", err
		}
	}
	// If this is reached, an object exists for the requested hash but it is
	// uncertain if a metadata object for it exists (a error finding it
	// could be due to a network / service failure in the store requesting
	// it). By returning without an error, an assumption has been made that
	// the file definitively does not yet exist and the consumer should
	// begin populating an empty one with their data.
	// TODO:
	//   Dig into failures on store.Get to ensure the failure isn't related to
	//   the backing store being unable to process requests. The lack of this
	//   checking makes it possible for an existing meta object to be clobbered
	//   by a consumer who thinks it isn't there.
	return meta, name, nil
}
