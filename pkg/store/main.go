// Package store provides an implementation type and generalized instantiation
// mechanism for storage engines.
package store

import (
	"fmt"
	"github.com/tkellen/memorybox/pkg/store/localdisk"
	"github.com/tkellen/memorybox/pkg/store/object"
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

// New instantiates any supported store using data that likely originated from
// a specific target in a memorybox configuration file.
func New(config map[string]string) (Store, error) {
	if config["type"] == "localDisk" {
		return localdisk.NewFromConfig(config), nil
	}
	if config["type"] == "s3" {
		return object.NewFromConfig(config), nil
	}
	return nil, fmt.Errorf("unknown store type: %s", config["type"])
}
