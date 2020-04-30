package memorybox

import (
	"context"
	"fmt"
	"io"
)

// Get retrieves an object from a Store by hash and copies it to a sink.
func Get(ctx context.Context, store Store, hash string, sink io.Writer) error {
	matches, searchErr := store.Search(ctx, hash)
	if searchErr != nil {
		return fmt.Errorf("get: %w", searchErr)
	}
	if len(matches) != 1 {
		return fmt.Errorf("%d objects matched", len(matches))
	}
	data, err := store.Get(ctx, matches[0])
	if err != nil {
		return err
	}
	defer data.Close()
	if _, err := io.Copy(sink, data); err != nil {
		return err
	}
	return nil
}
