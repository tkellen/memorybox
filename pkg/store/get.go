package store

import (
	"context"
	"io"
	"log"
)

// Get retrieves an object from a Store by hash and copies it to a sink.
func Get(ctx context.Context, store Store, hash string, stdout *log.Logger) error {
	match, findErr := findOne(ctx, store, hash)
	if findErr != nil {
		return findErr
	}
	data, err := store.Get(ctx, match)
	if err != nil {
		return err
	}
	defer data.Close()
	if _, err := io.Copy(stdout.Writer(), data); err != nil {
		return err
	}
	return nil
}
