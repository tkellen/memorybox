// These are unit tests which validate nothing more than how the abstraction of
// the Store interface maps calls to the AWS SDK for S3. Honestly, the value of
// spending time writing these tests is about the equivalent to knitting a hat
// vs buying one. Whatever. Here we go.
package objectstore_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/tkellen/memorybox/pkg/objectstore"
	"io/ioutil"
	"reflect"
	"testing"
	"time"
)

type s3mock struct {
	getObjectWithContext        func(aws.Context, *s3.GetObjectInput, ...request.Option) (*s3.GetObjectOutput, error)
	deleteObjectWithContext     func(aws.Context, *s3.DeleteObjectInput, ...request.Option) (*s3.DeleteObjectOutput, error)
	listObjectsPagesWithContext func(aws.Context, *s3.ListObjectsInput, func(*s3.ListObjectsOutput, bool) bool, ...request.Option) error
	headObjectWithContext       func(aws.Context, *s3.HeadObjectInput, ...request.Option) (*s3.HeadObjectOutput, error)
}

func (s3 *s3mock) GetObjectWithContext(ctx aws.Context, input *s3.GetObjectInput, opts ...request.Option) (*s3.GetObjectOutput, error) {
	return s3.getObjectWithContext(ctx, input, opts...)
}
func (s3 *s3mock) HeadObjectWithContext(ctx aws.Context, input *s3.HeadObjectInput, opts ...request.Option) (*s3.HeadObjectOutput, error) {
	return s3.headObjectWithContext(ctx, input, opts...)
}
func (s3 *s3mock) ListObjectsPagesWithContext(ctx aws.Context, input *s3.ListObjectsInput, fn func(*s3.ListObjectsOutput, bool) bool, opts ...request.Option) error {
	return s3.listObjectsPagesWithContext(ctx, input, fn, opts...)
}
func (s3 *s3mock) DeleteObjectWithContext(ctx aws.Context, input *s3.DeleteObjectInput, opts ...request.Option) (*s3.DeleteObjectOutput, error) {
	return s3.deleteObjectWithContext(ctx, input, opts...)
}

type s3UploaderMock struct {
	uploadWithContext func(aws.Context, *s3manager.UploadInput, ...func(*s3manager.Uploader)) (*s3manager.UploadOutput, error)
}

func (u *s3UploaderMock) UploadWithContext(ctx aws.Context, input *s3manager.UploadInput, opts ...func(*s3manager.Uploader)) (*s3manager.UploadOutput, error) {
	return u.uploadWithContext(ctx, input, opts...)
}

func TestStore_String(t *testing.T) {
	bucket := "test"
	store := &objectstore.Store{Bucket: bucket}
	actual := store.String()
	expected := fmt.Sprintf("%s: %s", objectstore.Name, bucket)
	if expected != actual {
		t.Fatalf("expected %s, got %s", expected, actual)
	}
}

func TestStore_Get(t *testing.T) {
	called := false
	expectedBucket := "bucket"
	expectedFilename := "test"
	expectedContent := []byte("test")
	store := &objectstore.Store{
		Bucket: expectedBucket,
		S3: &s3mock{
			getObjectWithContext: func(ctx aws.Context, input *s3.GetObjectInput, opts ...request.Option) (*s3.GetObjectOutput, error) {
				called = true
				if expectedBucket != *input.Bucket {
					t.Fatalf("expected %s as bucket, got %s", expectedBucket, *input.Bucket)
				}
				if expectedFilename != *input.Key {
					t.Fatalf("expected %s as key, got %s", expectedFilename, *input.Key)
				}
				return &s3.GetObjectOutput{
					ContentLength: aws.Int64(int64(len(expectedContent))),
					LastModified:  aws.Time(time.Now()),
					Body:          ioutil.NopCloser(bytes.NewReader(expectedContent)),
					Metadata:      map[string]*string{},
				}, nil
			},
		},
	}
	store.Get(context.Background(), expectedFilename)
	if !called {
		t.Fatalf("expected call did not occur")
	}
}

func TestStore_Stat(t *testing.T) {
	called := false
	expectedBucket := "bucket"
	expectedFilename := "test"
	store := &objectstore.Store{
		Bucket: expectedBucket,
		S3: &s3mock{
			headObjectWithContext: func(ctx aws.Context, input *s3.HeadObjectInput, opts ...request.Option) (*s3.HeadObjectOutput, error) {
				called = true
				if expectedBucket != *input.Bucket {
					t.Fatalf("expected %s as bucket, got %s", expectedBucket, *input.Bucket)
				}
				if expectedFilename != *input.Key {
					t.Fatalf("expected %s as key, got %s", expectedFilename, *input.Key)
				}
				return &s3.HeadObjectOutput{
					ContentLength: aws.Int64(0),
					LastModified:  aws.Time(time.Time{}),
				}, nil
			},
		},
	}
	store.Stat(context.Background(), expectedFilename)
	if !called {
		t.Fatalf("expected call did not occur")
	}
}

func TestStore_Search(t *testing.T) {
	called := false
	expectedBucket := "bucket"
	expectedPrefix := "test"
	store := &objectstore.Store{
		Bucket: expectedBucket,
		S3: &s3mock{
			listObjectsPagesWithContext: func(ctx aws.Context, input *s3.ListObjectsInput, fn func(*s3.ListObjectsOutput, bool) bool, opts ...request.Option) error {
				called = true
				if expectedBucket != *input.Bucket {
					t.Fatalf("expected %s as bucket, got %s", expectedBucket, *input.Bucket)
				}
				if expectedPrefix != *input.Prefix {
					t.Fatalf("expected %s as key, got %s", expectedPrefix, *input.Prefix)
				}
				// this is pretty worthless as a test
				fn(&s3.ListObjectsOutput{Contents: []*s3.Object{
					{Key: aws.String("foo"), LastModified: &time.Time{}, Size: aws.Int64(3)},
					{Key: aws.String("bar"), LastModified: &time.Time{}, Size: aws.Int64(3)},
					{Key: aws.String("baz"), LastModified: &time.Time{}, Size: aws.Int64(3)},
				}}, true)
				return nil
			},
		},
	}
	store.Search(context.Background(), expectedPrefix)
	if !called {
		t.Fatalf("expected call did not occur")
	}
}

func TestStore_Delete(t *testing.T) {
	called := false
	expectedBucket := "bucket"
	expectedKey := "key"
	store := &objectstore.Store{
		Bucket: expectedBucket,
		S3: &s3mock{
			deleteObjectWithContext: func(ctx aws.Context, input *s3.DeleteObjectInput, opts ...request.Option) (*s3.DeleteObjectOutput, error) {
				called = true
				if expectedBucket != *input.Bucket {
					t.Fatalf("expected %s as bucket, got %s", expectedBucket, *input.Bucket)
				}
				if expectedKey != *input.Key {
					t.Fatalf("expected %s as key, got %s", expectedKey, *input.Key)
				}
				return &s3.DeleteObjectOutput{}, nil
			},
		},
	}
	store.Delete(context.Background(), expectedKey)
	if !called {
		t.Fatalf("expected call did not occur")
	}
}

func TestStore_SearchError(t *testing.T) {
	called := false
	expectedBucket := "bucket"
	expectedPrefix := "test"
	store := &objectstore.Store{
		Bucket: expectedBucket,
		S3: &s3mock{
			listObjectsPagesWithContext: func(ctx aws.Context, input *s3.ListObjectsInput, fn func(*s3.ListObjectsOutput, bool) bool, opts ...request.Option) error {
				called = true
				if expectedBucket != *input.Bucket {
					t.Fatalf("expected %s as bucket, got %s", expectedBucket, *input.Bucket)
				}
				if expectedPrefix != *input.Prefix {
					t.Fatalf("expected %s as key, got %s", expectedPrefix, *input.Prefix)
				}
				return nil
			},
		},
	}
	store.Search(context.Background(), expectedPrefix)
	if !called {
		t.Fatal("expected call did not occur")
	}
}

func TestStore_Put_Failure(t *testing.T) {
	called := false
	expectedBucket := "bucket"
	expectedReader := bytes.NewReader([]byte("test"))
	expectedFilename := "test"
	expectedError := errors.New("failed")
	store := &objectstore.Store{
		Bucket: expectedBucket,
		Uploader: &s3UploaderMock{
			uploadWithContext: func(_ aws.Context, input *s3manager.UploadInput, opts ...func(*s3manager.Uploader)) (*s3manager.UploadOutput, error) {
				called = true
				if expectedBucket != *input.Bucket {
					t.Fatalf("expected %s as bucket, got %s", expectedBucket, *input.Bucket)
				}
				if expectedFilename != *input.Key {
					t.Fatalf("expected %s as key, got %s", expectedFilename, *input.Key)
				}
				if expectedReader != input.Body {
					t.Fatalf("expected %v as reader, got %v", expectedReader, input.Body)
				}
				return nil, expectedError
			},
		},
	}
	err := store.Put(context.Background(), expectedReader, expectedFilename, time.Now())
	if !called {
		t.Fatalf("expected call did not occur")
	}
	if expectedError != err {
		t.Fatalf("expected error %s, got %s", expectedError, err)
	}
}

func TestStore_Concat(t *testing.T) {
	expected := [][]byte{[]byte("foo"), []byte("bar")}
	var input []string
	backend := map[string][]byte{}
	for _, content := range expected {
		backend[string(content)] = content
		input = append(input, string(content))
	}
	store := &objectstore.Store{
		Bucket: "test",
		S3: &s3mock{
			getObjectWithContext: func(_ aws.Context, input *s3.GetObjectInput, _ ...request.Option) (*s3.GetObjectOutput, error) {
				return &s3.GetObjectOutput{
					ContentLength: aws.Int64(int64(len(backend[*input.Key]))),
					LastModified:  aws.Time(time.Now()),
					Body:          ioutil.NopCloser(bytes.NewReader(backend[*input.Key])),
					Metadata: map[string]*string{
						"memorybox.LastModified": aws.String(time.Now().UTC().Format(time.RFC3339)),
					},
				}, nil
			},
		},
	}
	actual, err := store.Concat(context.Background(), 1, input)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(expected, actual) {
		t.Fatalf("expected %s, got %s", expected, actual)
	}
}

func TestStore_ConcatFail(t *testing.T) {
	expectedErr := errors.New("bad time")
	store := &objectstore.Store{
		Bucket: "test",
		S3: &s3mock{
			getObjectWithContext: func(ctx aws.Context, _ *s3.GetObjectInput, _ ...request.Option) (*s3.GetObjectOutput, error) {
				return nil, ctx.Err()
			},
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := store.Concat(ctx, 2, []string{"foo", "bar", "baz"})
	if err == expectedErr {
		t.Fatalf("expected error %s, got %s", err, expectedErr)
	}
}
