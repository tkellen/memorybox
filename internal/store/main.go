package store

import (
	"fmt"
	"io"
)

// Store defines a storage engine that can persist and retrieve content.
type Store interface {
	Exists(string) bool
	Get(string) (io.ReadCloser, error)
	Put(io.Reader, string) error
	Search(string) ([]string, error)
	String() string
}

// New creates the appropriate type of store given the configuration supplied.
func New(config map[string]string) (Store, error) {
	storeType := config["type"]
	if storeType == "localDisk" {
		return NewLocalDiskStoreFromConfig(config), nil
	}
	if storeType == "s3" {
		return NewObjectStoreFromConfig(config), nil
	}
	if storeType == "testing" {
		return NewTestingStore([]TestingStoreFixture{}), nil
	}
	return nil, fmt.Errorf("unknown store type %s", storeType)
}
