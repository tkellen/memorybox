package store

import (
	"encoding/hex"
	"fmt"
	hash "github.com/minio/sha256-simd"
	"github.com/tkellen/memorybox/internal/store/localdiskstore"
	"github.com/tkellen/memorybox/internal/store/objectstore"
	"github.com/tkellen/memorybox/internal/store/testingstore"
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

type hashFn func(source io.Reader) (string, int64, error)

// Sha256 computes a sha256 message digest for a provided io.Reader.
func Sha256(source io.Reader) (string, int64, error) {
	hash := hash.New()
	size, err := io.Copy(hash, source)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(hash.Sum(nil)) + "-sha256", size, nil
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
	if storeType == "testing" {
		return testingstore.New([]testingstore.Fixture{}), nil
	}
	return nil, fmt.Errorf("unknown store type %s", storeType)
}
