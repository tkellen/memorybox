package memorybox

import "io"

// Store defines a storage engine that can save and retrieve content.
type Store interface {
	Exists(string) bool
	Get(string) (io.ReadCloser, error)
	Put(io.ReadCloser, string) error
	String() string
}

// Index defines an interface for indexing and searching content.
type Index interface {
	Search(string) ([]string, error)
}
