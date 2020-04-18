// Package get is a high level abstraction over store.Store for retrieving
// content. It is designed to execute operations that are supported by the cli.
package get

import (
	"fmt"
	"github.com/tkellen/memorybox/pkg/store"
	"io"
)

// Commands defines the interface needed to expose this functionality in a way
// that can be mocked for testing by consumers.
type Commands interface {
	Main(store store.Store, hash string, sink io.Writer) error
}

// Command implements Commands as the public API of this package.
type Command struct{}

// Main retrieves an object from a Store by hash and copies it to the sink.
func (Command) Main(store store.Store, hash string, sink io.Writer) error {
	matches, searchErr := store.Search(hash)
	if searchErr != nil {
		return fmt.Errorf("get: %s", searchErr)
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
