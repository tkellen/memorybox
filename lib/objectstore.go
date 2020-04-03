package memorybox

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

// String returns a human-friendly representation of the store.
func (s *ObjectStore) String() string {
	return fmt.Sprintf("ObjectStore: %s", s.Bucket)
}

// NewObjectStore returns a reference to an ObjectStore instance.
func NewObjectStore(bucket string) (*ObjectStore, error) {
	creds := credentials.NewEnvAWS()
	client, err := minio.NewWithCredentials("s3.amazonaws.com", creds, true, "us-east-1")
	if err != nil {
		return nil, fmt.Errorf("client: %s", err)
	}
	return &ObjectStore{
		Bucket: strings.TrimPrefix(bucket, "s3://"),
		Client: client,
	}, nil
}

// Put writes the content of an io.Reader to object storage.
func (s *ObjectStore) Put(src io.Reader, key string) error {
	if _, err := s.Client.PutObject(s.Bucket, key, src, -1, minio.PutObjectOptions{}); err != nil {
		return err
	}
	return nil
}

// Search finds an object in storage by prefix and returns an array of matches
func (s *ObjectStore) Search(search string) ([]string, error) {
	var matches []string
	done := make(chan struct{})
	defer close(done)
	objects := s.Client.ListObjectsV2(s.Bucket, search, true, done)
	for object := range objects {
		if object.Err == nil {
			matches = append(matches, object.Key)
		}
	}
	return matches, nil
}

// Get finds an object in storage by name and returns an io.Reader for it.
func (s *ObjectStore) Get(key string) (io.Reader, error) {
	object, err := s.Client.GetObject(s.Bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	return object, nil
}

// Exists determines if a given file exists in the object store already.
func (s *ObjectStore) Exists(key string) bool {
	_, err := s.Client.StatObject(s.Bucket, key, minio.StatObjectOptions{})
	return err == nil
}
