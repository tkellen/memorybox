package objectstore

import (
	"fmt"
	"github.com/minio/minio-go/v6"
	"github.com/minio/minio-go/v6/pkg/credentials"
	"io"
	"strings"
)

// Store implements memorybox.Store backed by s3-compatible object storage.
type Store struct {
	Bucket string
	Client s3
}

// s3 defines a mock-able interface that represents the subset of functionality
// needed to support using minio.Client as a back end for an object store.
type s3 interface {
	PutObject(string, string, io.Reader, int64, minio.PutObjectOptions) (int64, error)
	GetObject(string, string, minio.GetObjectOptions) (*minio.Object, error)
	StatObject(string, string, minio.StatObjectOptions) (minio.ObjectInfo, error)
}

// String returns a human friendly representation of the Store.
func (s *Store) String() string {
	return fmt.Sprintf("ObjectStore: %s", s.Bucket)
}

// New returns a reference to a Store instance.
func New(bucket string, client s3) *Store {
	return &Store{
		Bucket: strings.TrimPrefix(bucket, "s3://"),
		Client: client,
	}
}

// NewFromTarget instantiates a Store using configuration values that were
// likely sourced from a configuration file target.
// TODO: properly support aws with more settings
func NewFromTarget(config map[string]string) *Store {
	creds := credentials.NewEnvAWS()
	client, _ := minio.NewWithCredentials("s3.amazonaws.com", creds, true, "us-east-1")
	return New(config["home"], client)
}

// Put writes the content of an io.Reader to object storage.
func (s *Store) Put(source io.ReadCloser, hash string) error {
	defer source.Close()
	if _, err := s.Client.PutObject(s.Bucket, hash, source, -1, minio.PutObjectOptions{}); err != nil {
		return err
	}
	return nil
}

// Get finds an object in storage by name and returns an io.Reader for it.
func (s *Store) Get(key string) (io.ReadCloser, error) {
	return s.Client.GetObject(s.Bucket, key, minio.GetObjectOptions{})
}

// Exists determines if a given file exists in the object store already.
func (s *Store) Exists(key string) bool {
	_, err := s.Client.StatObject(s.Bucket, key, minio.StatObjectOptions{})
	return err == nil
}