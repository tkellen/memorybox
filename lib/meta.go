package memorybox

import (
	"context"
	"fmt"
	"github.com/mattetti/filebuffer"
	"github.com/tkellen/memorybox/pkg/archive"
	"io"
	"io/ioutil"
)

// MetaGet gets a metadata file from a defined store.
func MetaGet(ctx context.Context, store Store, hash string, sink io.Writer) error {
	metaFile, findErr := findMeta(ctx, store, hash)
	if findErr != nil {
		return findErr
	}
	if _, err := io.Copy(sink, metaFile); err != nil {
		return err
	}
	return nil
}

// MetaSet adds a key to a metadata file and persists it to the store.
func MetaSet(ctx context.Context, store Store, search string, key string, value interface{}) error {
	metaFile, findErr := findMeta(ctx, store, search)
	if findErr != nil {
		return findErr
	}
	metaFile.MetaSet(key, value.(string))
	return store.Put(ctx, metaFile, metaFile.Name())
}

// MetaDelete removes a key from a metadata file and persists it to the store.
func MetaDelete(ctx context.Context, store Store, search string, key string) error {
	metaFile, findErr := findMeta(ctx, store, search)
	if findErr != nil {
		return findErr
	}
	metaFile.MetaDelete(key)
	return store.Put(ctx, metaFile, metaFile.Name())
}

func findMeta(ctx context.Context, store Store, search string) (*archive.File, error) {
	// First determine if the file to annotate even exists.
	matches, searchErr := store.Search(ctx, search)
	if searchErr != nil {
		return nil, fmt.Errorf("get: %w", searchErr)
	}
	if len(matches) != 1 {
		return nil, fmt.Errorf("%d objects matched", len(matches))
	}
	// If there is exactly one match, try to fetch the metadata file for it.
	reader, getErr := store.Get(ctx, archive.MetaFileNameFrom(matches[0]))
	if getErr != nil {
		return nil, getErr
	}
	bytes, readErr := ioutil.ReadAll(reader)
	if readErr != nil {
		return nil, readErr
	}
	return archive.NewSha256("meta-search", filebuffer.New(bytes))
}
