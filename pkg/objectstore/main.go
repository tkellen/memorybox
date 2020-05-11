package objectstore

import (
	"context"
	"fmt"
	"github.com/minio/minio-go/v6"
	"io"
	"strings"
)

// Store implements store.Store backed by s3-compatible object storage.
type Store struct {
	Bucket string
	Client s3
}

// s3 defines a mock-able interface that represents the subset of functionality
// needed to support using minio.Client as a back end for an object store.
type s3 interface {
	PutObjectWithContext(context.Context, string, string, io.Reader, int64, minio.PutObjectOptions) (int64, error)
	GetObjectWithContext(context.Context, string, string, minio.GetObjectOptions) (*minio.Object, error)
	ListObjects(string, string, bool, <-chan struct{}) <-chan minio.ObjectInfo
	StatObjectWithContext(context.Context, string, string, minio.StatObjectOptions) (minio.ObjectInfo, error)
	RemoveObject(string, string) error
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

// NewFromConfig instantiates a Store using configuration values from a config
// file.
func NewFromConfig(config map[string]string) *Store {
	client, _ := minio.New(config["endpoint"], config["access_key_id"], config["secret_access_key"], true)
	return New(config["bucket"], client)
}

// Put writes the content of an io.Reader to object storage.
func (s *Store) Put(ctx context.Context, source io.Reader, hash string) error {
	if _, err := s.Client.PutObjectWithContext(ctx, s.Bucket, hash, source, -1, minio.PutObjectOptions{}); err != nil {
		return err
	}
	return nil
}

// Get finds an object in storage by name and returns an io.ReadCloser for it.
func (s *Store) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	return s.Client.GetObjectWithContext(ctx, s.Bucket, key, minio.GetObjectOptions{})
}

// Delete removes an object from storage.
func (s *Store) Delete(_ context.Context, key string) error {
	return s.Client.RemoveObject(s.Bucket, key)
}

// Search finds an object in storage by prefix and returns an array of matches
func (s *Store) Search(ctx context.Context, search string) ([]string, error) {
	var matches []string
	var err error
	done := make(chan struct{})
	defer close(done)
	for object := range s.Client.ListObjects(s.Bucket, search, true, done) {
		err = ctx.Err()
		matches = append(matches, object.Key)
		if object.Err != nil {
			err = object.Err
		}
		if err != nil {
			matches = nil
			break
		}
	}
	return matches, err
}

// Exists determines if a given file exists in the object store already.
func (s *Store) Exists(ctx context.Context, key string) bool {
	_, err := s.Client.StatObjectWithContext(ctx, s.Bucket, key, minio.StatObjectOptions{})
	return err == nil
}
