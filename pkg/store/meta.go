package store

import (
	"context"
	"github.com/mattetti/filebuffer"
	"github.com/tkellen/memorybox/pkg/archive"
	"io"
	"log"
)

// MetaGet gets a metadata file from a defined store.
func MetaGet(ctx context.Context, store Store, hash string, stdout *log.Logger) error {
	metaFile, findErr := findMeta(ctx, store, hash)
	if findErr != nil {
		return findErr
	}
	if _, err := io.Copy(stdout.Writer(), metaFile); err != nil {
		return err
	}
	return nil
}

// MetaSet adds a key to a metadata file and persists it to the store.
func MetaSet(ctx context.Context, store Store, search string, key string, value interface{}, stderr *log.Logger, stdout *log.Logger) error {
	metaFile, findErr := findMeta(ctx, store, search)
	if findErr != nil {
		return findErr
	}
	metaFile.MetaSet(key, value.(string))
	return putFile(ctx, store, metaFile, stderr, stdout)
}

// MetaDelete removes a key from a metadata file and persists it to the store.
func MetaDelete(ctx context.Context, store Store, search string, key string, stderr *log.Logger, stdout *log.Logger) error {
	metaFile, findErr := findMeta(ctx, store, search)
	if findErr != nil {
		return findErr
	}
	metaFile.MetaDelete(key)
	return putFile(ctx, store, metaFile, stderr, stdout)
}

func findMeta(ctx context.Context, store Store, search string) (*archive.File, error) {
	match, findErr := findOne(ctx, store, search)
	if findErr != nil {
		return nil, findErr
	}
	// If there is exactly one match, try to fetch the metadata file for it.
	reader, getErr := store.Get(ctx, archive.MetaFileNameFrom(match))
	if getErr != nil {
		return nil, getErr
	}
	file, fileErr := filebuffer.NewFromReader(reader)
	if fileErr != nil {
		return nil, fileErr
	}
	return archive.NewSha256("meta-search", file)
}
