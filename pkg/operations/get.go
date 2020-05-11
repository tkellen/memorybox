package operations

import (
	"context"
	"github.com/tkellen/memorybox/pkg/store"
	"io"
)

// Get retrieves an object from a Store by hash and copies it to a sink.
func Get(ctx context.Context, logger *Logger, s store.Store, request string) error {
	match, findErr := findOne(ctx, s, request)
	if findErr != nil {
		return findErr
	}
	reader, getErr := s.Get(ctx, match)
	if getErr != nil {
		return getErr
	}
	defer reader.Close()
	if _, err := io.Copy(logger.Stdout.Writer(), reader); err != nil {
		return err
	}
	return nil
}
