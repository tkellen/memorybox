// Package meta is a high level abstraction over store.Store for managing
// annotations on objects. It is designed to execute operations that are
// supported by the cli.
package meta

import (
	"errors"
	"github.com/tkellen/memorybox/pkg/commands/get"
	"github.com/tkellen/memorybox/pkg/store"
	"io"
)

// Commands defines the interface needed to expose this functionality in a way
// that can be mocked for testing by consumers.
type Commands interface {
	Main(store.Store, string, io.Writer) error
	Set(store.Store, string, string, string) error
	Delete(store.Store, string, string) error
}

// Command implements Commands as the public API of this package.
type Command struct{}

// Main is not done yet.
func (Command) Main(store store.Store, hash string, sink io.Writer) error {
	return (get.Command{}).Main(store, "meta-"+hash, sink)
}

// Set is not done yet.
func (Command) Set(store store.Store, hash string, key string, value string) error {
	return errors.New("delete not implemented")
}

// Delete is not done yet.
func (Command) Delete(store store.Store, hash string, key string) error {
	return errors.New("delete not implemented")
}
