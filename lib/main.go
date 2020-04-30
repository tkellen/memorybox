package memorybox

import (
	"context"
	"fmt"
	"github.com/tkellen/memorybox/internal/testingstore"
	"github.com/tkellen/memorybox/pkg/archive"
	"github.com/tkellen/memorybox/pkg/localdiskstore"
	"github.com/tkellen/memorybox/pkg/objectstore"
	"io"
)

// Store defines a storage engine that can persist and retrieve content.
type Store interface {
	Exists(context.Context, string) bool
	Get(context.Context, string) (io.ReadCloser, error)
	Put(context.Context, io.Reader, string) error
	Search(context.Context, string) ([]string, error)
	String() string
}

// NewStore creates the appropriate type of store given the configuration supplied.
func NewStore(config map[string]string) (Store, error) {
	storeType := config["type"]
	if storeType == "localDisk" {
		return localdiskstore.NewFromConfig(config), nil
	}
	if storeType == "s3" {
		return objectstore.NewFromConfig(config), nil
	}
	if storeType == "testing" {
		return testingstore.New([]*archive.File{}), nil
	}
	return nil, fmt.Errorf("unknown store type %s", storeType)
}
