package store

import (
	"context"
	"errors"
	"fmt"
	"github.com/tkellen/memorybox/pkg/localdiskstore"
	"github.com/tkellen/memorybox/pkg/objectstore"
	"io"
)

// Store defines a storage engine that can persist and retrieve content.
type Store interface {
	Exists(context.Context, string) bool
	Get(context.Context, string) (io.ReadCloser, error)
	Put(context.Context, io.Reader, string) error
	Delete(context.Context, string) error
	Search(context.Context, string) ([]string, error)
	String() string
}

// New creates the appropriate type of store given the configuration supplied.
func New(config map[string]string) (Store, error) {
	storeType := config["type"]
	if storeType == "localDisk" {
		return localdiskstore.NewFromConfig(config), nil
	}
	if storeType == "s3" {
		return objectstore.NewFromConfig(config), nil
	}
	return nil, fmt.Errorf("unknown store type %s", storeType)
}

var errCorrupted = errors.New("store corrupted")

// IsCorrupted wraps an error if an error from an operation on a store is the
// result of corruption in the store from one of the following:
// 1. Meta file missing data file.
// 2. Data file missing meta file.
// 3. Meta file pointing to wrong data file.
// 4. Data file hashes to different filename.
func IsCorrupted(err error) bool {
	return errors.Is(err, errCorrupted)
}

// Corrupted determines if an error about a store indicates internal corruption.
func Corrupted(err error) error {
	return fmt.Errorf("%w: %s", errCorrupted, err)
}
