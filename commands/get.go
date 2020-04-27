package commands

import (
	"fmt"
	"github.com/tkellen/memorybox/internal/store"
	"io"
)

// Get retrieves an object from a Store by hash and copies it to a sink.
func Get(store store.Store, hash string, sink io.Writer) error {
	matches, searchErr := store.Search(hash)
	if searchErr != nil {
		return fmt.Errorf("get: %w", searchErr)
	}
	if len(matches) != 1 {
		return fmt.Errorf("%d objects matched", len(matches))
	}
	data, err := store.Get(matches[0])
	if err != nil {
		return err
	}
	if _, err := io.Copy(sink, data); err != nil {
		return err
	}
	return nil
}
