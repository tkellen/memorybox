package main

import (
	"fmt"
	"github.com/minio/minio-go"
	"github.com/minio/minio-go/pkg/credentials"
	"io"
	"strings"
)

// ObjectStore implements Store backed by s3-compatible object storage.
type ObjectStore struct {
	Bucket string
	Client *minio.Client
}

// NewObjectStore returns a reference to an ObjectStore instance.
func NewObjectStore(bucket string) (*ObjectStore, error) {
	creds := credentials.NewEnvAWS()
	client, err := minio.NewWithCredentials("s3.amazonaws.com", creds, true, "us-east-1")
	if err != nil {
		return nil, fmt.Errorf("no client: %s", err)
	}
	return &ObjectStore{
		Bucket: strings.TrimPrefix(bucket, "s3://"),
		Client: client,
	}, nil
}

// Save writes the content of an io.Reader to object storage.
func (s *ObjectStore) Save(src io.Reader, key string) error {
	if _, err := s.Client.PutObject(s.Bucket, key, src, -1, minio.PutObjectOptions{}); err != nil {
		return fmt.Errorf("object store failed to put: %s", err)
	}
	return nil
}

// Exists determines if a given file exists in the object store already.
func (s *ObjectStore) Exists(key string) bool {
	_, err := s.Client.StatObject(s.Bucket, key, minio.StatObjectOptions{})
	return err == nil
}
