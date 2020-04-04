package memorybox

import "io"

// Store defines a storage engine that can save and index content.
type Store interface {
	Exists(string) bool
	Get(string) (io.ReadCloser, error)
	Put(io.ReadCloser, string) error
	Search(string) ([]string, error)
	String() string
}
