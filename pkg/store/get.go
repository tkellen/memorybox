package store

import (
	"context"
	"io"
)

// Get retrieves an object from a Store by hash and copies it to a sink.
func Get(ctx context.Context, store Store, hash string, sink io.Writer) error {
	match, findErr := findOne(ctx, store, hash)
	if findErr != nil {
		return findErr
	}
	data, err := store.Get(ctx, match)
	if err != nil {
		return err
	}
	defer data.Close()
	if _, err := io.Copy(sink, data); err != nil {
		return err
	}
	return nil
}
