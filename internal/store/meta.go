package store

import (
	"fmt"
	"github.com/tkellen/memorybox/internal/archive"
	"io"
)

// MetaGet gets a metadata file from a defined store.
func MetaGet(store Store, hash string, sink io.Writer) error {
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

// MetaSet adds a key to a metadata file and persists it to the store.
func MetaSet(store Store, search string, key string, value interface{}) error {
	metaFile, findErr := findMeta(store, search)
	if findErr != nil {
		return findErr
	}
	metaFile.MetaSet(key, value.(string))
	return store.Put(metaFile, metaFile.Name())
}

// MetaDelete removes a key from a metadata file and persists it to the store.
func MetaDelete(store Store, search string, key string) error {
	metaFile, findErr := findMeta(store, search)
	if findErr != nil {
		return findErr
	}
	metaFile.MetaDelete(key)
	return store.Put(metaFile, metaFile.Name())
}

func findMeta(store Store, search string) (*archive.File, error) {
	// First determine if the file to annotate even exists.
	matches, searchErr := store.Search(search)
	if searchErr != nil {
		return nil, fmt.Errorf("get: %w", searchErr)
	}
	if len(matches) != 1 {
		return nil, fmt.Errorf("%d objects matched", len(matches))
	}
	// If there is exactly one match, try to fetch the metadata file for it.
	reader, getErr := store.Get(archive.MetaFileName(matches[0]))
	if getErr != nil {
		return nil, getErr
	}
	return archive.NewFromReader(Sha256, reader)
}
