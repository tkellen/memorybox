package main

import (
	"fmt"
	"github.com/minio/minio-go"
	"github.com/minio/minio-go/pkg/credentials"
	"io"
	"strings"
)

// ObjectStore is a Store implementation that uses s3-compatible object storage.
type ObjectStore struct {
	Bucket string
	Client *minio.Client
}

// NewObjectStore returns a reference to an ObjectStore instance writes to a
// s3 compatible object store bucket.
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

func (s *ObjectStore) Root() string {
	return s.Bucket
}

func (s *ObjectStore) Save(src io.Reader, dest string) error {
	opts := minio.PutObjectOptions{}
	if _, err := s.Client.PutObject(s.Bucket, dest, src, -1, opts); err != nil {
		return fmt.Errorf("object store failed: %v", err)
	}
	return nil
}

func (s *ObjectStore) Index(temp string, hash string) error {
	src := minio.NewSourceInfo(s.Bucket, temp, nil)
	dest, err := minio.NewDestinationInfo(s.Bucket, hash, nil, nil)
	if err != nil {
		return err
	}
	if err := s.Client.CopyObject(dest, src); err != nil {
		return err
	}
	if err := s.Client.RemoveObject(s.Bucket, temp); err != nil {
		return err
	}
	return nil
}
