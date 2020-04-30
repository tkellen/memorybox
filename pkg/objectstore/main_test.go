// These are unit tests which validate nothing more than how the abstraction of
// the Store interface maps calls to *minio.Client.
//
// I could actually *run* minio during testing. Here is how it is done:
//
//   os.Setenv("MINIO_ACCESS_KEY", "access-key")
//   os.Setenv("MINIO_SECRET_KEY", "secret-key")
//   cmd.Main([]string{"--address :9001", "server", "/tmp/server"})
//
// ...but there is no supported way to wait for it to finish starting up, nor
// any way to cleanly shut it down.
//
// Honestly, the value of spending time writing these tests is, by analogy,
// equivalent to knitting a hat vs buying one. Whatever. Here we go.
package objectstore_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/minio/minio-go/v6"
	"github.com/tkellen/memorybox/pkg/objectstore"
	"io"
	"io/ioutil"
	"testing"
)

type s3mock struct {
	putObject   func(context.Context, string, string, io.Reader, int64, minio.PutObjectOptions) (int64, error)
	getObject   func(context.Context, string, string, minio.GetObjectOptions) (*minio.Object, error)
	listObjects func(string, string, bool, <-chan struct{}) <-chan minio.ObjectInfo
	statObject  func(context.Context, string, string, minio.StatObjectOptions) (minio.ObjectInfo, error)
}

func (s3 *s3mock) PutObjectWithContext(ctx context.Context, bucket string, key string, reader io.Reader, size int64, opts minio.PutObjectOptions) (int64, error) {
	return s3.putObject(ctx, bucket, key, reader, size, opts)
}
func (s3 *s3mock) GetObjectWithContext(ctx context.Context, bucket string, key string, opts minio.GetObjectOptions) (*minio.Object, error) {
	return s3.getObject(ctx, bucket, key, opts)
}
func (s3 *s3mock) StatObjectWithContext(ctx context.Context, bucket string, key string, opts minio.StatObjectOptions) (minio.ObjectInfo, error) {
	return s3.statObject(ctx, bucket, key, opts)
}
func (s3 *s3mock) ListObjects(bucket string, prefix string, recursive bool, doneCh <-chan struct{}) <-chan minio.ObjectInfo {
	return s3.listObjects(bucket, prefix, recursive, doneCh)
}

func TestNewFromConfig(t *testing.T) {
	expected := "bucket-name"
	actual := objectstore.NewFromConfig(map[string]string{
		"home": expected,
	})
	if expected != actual.Bucket {
		t.Fatalf("expected bucket of %s, got %s", expected, actual.Bucket)
	}
}

func TestStore_String(t *testing.T) {
	bucket := "test"
	store := objectstore.New("test", nil)
	actual := store.String()
	expected := fmt.Sprintf("ObjectStore: %s", bucket)
	if expected != actual {
		t.Fatalf("expected %s, got %s", expected, actual)
	}
}

func TestStore_Put_Success(t *testing.T) {
	called := false
	expectedBucket := "bucket"
	expectedReader := bytes.NewReader([]byte("test"))
	expectedFilename := "test"
	objectstore.New(expectedBucket, &s3mock{
		putObject: func(_ context.Context, bucket string, key string, reader io.Reader, size int64, options minio.PutObjectOptions) (int64, error) {
			called = true
			if expectedBucket != bucket {
				t.Fatalf("expected %s as bucket, got %s", expectedBucket, bucket)
			}
			if expectedFilename != key {
				t.Fatalf("expected %s as key, got %s", expectedFilename, key)
			}
			if expectedReader != reader {
				t.Fatalf("expected %v as reader, got %v", expectedReader, reader)
			}
			bytes, _ := ioutil.ReadAll(expectedReader)
			return int64(len(bytes)), nil
		},
	}).Put(context.Background(), expectedReader, expectedFilename)
	if !called {
		t.Fatalf("expected call did not occur")
	}
}

func TestStore_Put_Failure(t *testing.T) {
	called := false
	expectedBucket := "bucket"
	expectedReader := bytes.NewReader([]byte("test"))
	expectedFilename := "test"
	expectedError := errors.New("failed")
	err := objectstore.New(expectedBucket, &s3mock{
		putObject: func(_ context.Context, bucket string, key string, reader io.Reader, size int64, options minio.PutObjectOptions) (int64, error) {
			called = true
			if expectedBucket != bucket {
				t.Fatalf("expected %s as bucket, got %s", expectedBucket, bucket)
			}
			if expectedFilename != key {
				t.Fatalf("expected %s as key, got %s", expectedFilename, key)
			}
			if expectedReader != reader {
				t.Fatalf("expected %v as reader, got %v", expectedReader, reader)
			}
			return 0, expectedError
		},
	}).Put(context.Background(), expectedReader, expectedFilename)
	if !called {
		t.Fatalf("expected call did not occur")
	}
	if expectedError != err {
		t.Fatalf("expected error %s, got %s", expectedError, err)
	}
}

func TestStore_Get(t *testing.T) {
	called := false
	expectedBucket := "bucket"
	expectedFilename := "test"
	objectstore.New(expectedBucket, &s3mock{
		getObject: func(_ context.Context, bucket string, key string, options minio.GetObjectOptions) (*minio.Object, error) {
			called = true
			if expectedBucket != bucket {
				t.Fatalf("expected %s as bucket, got %s", expectedBucket, bucket)
			}
			if expectedFilename != key {
				t.Fatalf("expected %s as key, got %s", expectedFilename, key)
			}
			return &minio.Object{}, nil
		},
	}).Get(context.Background(), expectedFilename)
	if !called {
		t.Fatalf("expected call did not occur")
	}
}

func TestStore_Exists(t *testing.T) {
	called := false
	expectedBucket := "bucket"
	expectedFilename := "test"
	objectstore.New(expectedBucket, &s3mock{
		statObject: func(_ context.Context, bucket string, key string, options minio.StatObjectOptions) (minio.ObjectInfo, error) {
			called = true
			if expectedBucket != bucket {
				t.Fatalf("expected %s as bucket, got %s", expectedBucket, bucket)
			}
			if expectedFilename != key {
				t.Fatalf("expected %s as key, got %s", expectedFilename, key)
			}
			return minio.ObjectInfo{}, nil
		},
	}).Exists(context.Background(), expectedFilename)
	if !called {
		t.Fatalf("expected call did not occur")
	}
}

func TestStore_Search(t *testing.T) {
	called := false
	expectedBucket := "bucket"
	expectedPrefix := "test"
	objectstore.New(expectedBucket, &s3mock{
		listObjects: func(bucket string, prefix string, recursive bool, done <-chan struct{}) <-chan minio.ObjectInfo {
			called = true
			results := make(chan minio.ObjectInfo)
			if expectedBucket != bucket {
				t.Fatalf("expected %s as bucket, got %s", expectedBucket, bucket)
			}
			if expectedPrefix != prefix {
				t.Fatalf("expected %s as key, got %s", expectedPrefix, prefix)
			}
			go func() {
				results <- minio.ObjectInfo{}
				close(results)
			}()
			return results
		},
	}).Search(context.Background(), expectedPrefix)
	if !called {
		t.Fatalf("expected call did not occur")
	}
}

func TestStore_SearchContextCancel(t *testing.T) {
	called := false
	expectedBucket := "bucket"
	expectedPrefix := "test"
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := objectstore.New(expectedBucket, &s3mock{
		listObjects: func(bucket string, prefix string, recursive bool, done <-chan struct{}) <-chan minio.ObjectInfo {
			called = true
			results := make(chan minio.ObjectInfo)
			if expectedBucket != bucket {
				t.Fatalf("expected %s as bucket, got %s", expectedBucket, bucket)
			}
			if expectedPrefix != prefix {
				t.Fatalf("expected %s as key, got %s", expectedPrefix, prefix)
			}
			go func() {
				results <- minio.ObjectInfo{}
				close(results)
			}()
			return results
		},
	}).Search(ctx, expectedPrefix)
	if !called {
		t.Fatalf("expected call did not occur")
	}
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ctx.Err()) {
		t.Fatalf("expected error %s, got %s", ctx.Err(), err)
	}
}
