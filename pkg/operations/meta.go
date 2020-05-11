package operations

import (
	"context"
	"github.com/mattetti/filebuffer"
	"github.com/tkellen/memorybox/pkg/archive"
	"github.com/tkellen/memorybox/pkg/store"
	"io"
)

// MetaGet gets a metadata file from a defined store.
func MetaGet(ctx context.Context, logger *Logger, s store.Store, hash string) error {
	metaFile, findErr := findMeta(ctx, s, hash)
	if findErr != nil {
		return findErr
	}
	if _, err := io.Copy(logger.Stdout.Writer(), metaFile); err != nil {
		return err
	}
	return nil
}

// MetaSet adds a key to a metadata file and persists it to the store.
func MetaSet(ctx context.Context, logger *Logger, s store.Store, search string, key string, value interface{}) error {
	metaFile, findErr := findMeta(ctx, s, search)
	if findErr != nil {
		return findErr
	}
	metaFile.MetaSet(key, value.(string))
	return putFile(ctx, logger, s, metaFile)
}

// MetaDelete removes a key from a metadata file and persists it to the store.
func MetaDelete(ctx context.Context, logger *Logger, s store.Store, search string, key string) error {
	metaFile, findErr := findMeta(ctx, s, search)
	if findErr != nil {
		return findErr
	}
	metaFile.MetaDelete(key)
	return putFile(ctx, logger, s, metaFile)
}

func findMeta(ctx context.Context, s store.Store, search string) (*archive.File, error) {
	match, findErr := findOne(ctx, s, search)
	if findErr != nil {
		return nil, findErr
	}
	// If there is exactly one match, try to fetch the metadata file for it.
	reader, getErr := s.Get(ctx, archive.MetaFileNameFrom(match))
	if getErr != nil {
		return nil, getErr
	}
	// Make this an io.ReadSeeker.
	file, fileErr := filebuffer.NewFromReader(reader)
	if fileErr != nil {
		return nil, fileErr
	}
	return archive.NewSha256("meta-search", file)
}
