package objectstore

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/tkellen/memorybox/pkg/file"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
	"io"
	"io/ioutil"
	"sort"
	"time"
)

// Store implements archive.Store backed by s3-compatible object archive.
type Store struct {
	Bucket   string
	S3       s3Backend
	Uploader s3Uploader
	Session  *session.Session
}

// Name is used in the memorybox configuration file to determine which type of
// store to instantiate.
const Name = "objectStore"

// S3 does not allow changing the last modified time of a file. This makes the
// process of determining up-to-dated-ness when syncing to or from an object
// store work the same as from local disk.
const timeKey = "memorybox.LastModified"

type s3Backend interface {
	GetObjectWithContext(aws.Context, *s3.GetObjectInput, ...request.Option) (*s3.GetObjectOutput, error)
	DeleteObjectWithContext(aws.Context, *s3.DeleteObjectInput, ...request.Option) (*s3.DeleteObjectOutput, error)
	ListObjectsPagesWithContext(aws.Context, *s3.ListObjectsInput, func(*s3.ListObjectsOutput, bool) bool, ...request.Option) error
	HeadObjectWithContext(aws.Context, *s3.HeadObjectInput, ...request.Option) (*s3.HeadObjectOutput, error)
}

type s3Uploader interface {
	UploadWithContext(aws.Context, *s3manager.UploadInput, ...func(*s3manager.Uploader)) (*s3manager.UploadOutput, error)
}

// String returns a human friendly representation of the Store.
func (s *Store) String() string {
	return fmt.Sprintf("%s: %s", Name, s.Bucket)
}

// New returns a reference to a Store instance.
func New(bucket string, sess *session.Session) *Store {
	return &Store{
		Bucket: bucket,
		S3:     s3.New(sess),
		Uploader: s3manager.NewUploader(sess, func(u *s3manager.Uploader) {
			u.BufferProvider = s3manager.NewBufferedReadSeekerWriteToPool(25 * 1024 * 1024)
		}),
		Session: sess,
	}
}

// NewFromConfig produces a new instance of a store.
func NewFromConfig(config map[string]string) *Store {
	var sess *session.Session
	if profile, ok := config["profile"]; ok {
		sess, _ = session.NewSessionWithOptions(session.Options{
			Profile:           profile,
			SharedConfigState: session.SharedConfigEnable,
		})
	} else {
		sess, _ = session.NewSession(&aws.Config{
			Credentials: credentials.NewStaticCredentials(
				config["access_key_id"],
				config["secret_access_key"],
				"",
			),
			Endpoint: aws.String(config["endpoint"]),
			Region:   aws.String("us-east-1"),
		})
	}
	return New(config["bucket"], sess)
}

// Put writes the content of an io.Reader to the backing object storage bucket.
// It saves the actual lastModified time supplied as metadata because most s3
// implementations do not allow modifying it.
func (s *Store) Put(ctx context.Context, reader io.Reader, name string, lastModified time.Time) error {
	_, err := s.Uploader.UploadWithContext(ctx, &s3manager.UploadInput{
		Bucket: aws.String(s.Bucket),
		Key:    aws.String(name),
		Body:   reader,
		Metadata: map[string]*string{
			timeKey: aws.String(lastModified.UTC().Format(time.RFC3339)),
		},
	})
	return err
}

func (s *Store) lastModified(meta map[string]*string, fallback time.Time) time.Time {
	if betterTime, ok := meta[timeKey]; ok {
		result, err := time.Parse(time.RFC3339, *betterTime)
		if err != nil {
			return result
		}
	}
	return fallback
}

// Get finds an object and its metadata in storage by name.
func (s *Store) Get(ctx context.Context, name string) (*file.File, error) {
	resp, err := s.S3.GetObjectWithContext(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.Bucket),
		Key:    aws.String(name),
	})
	if err != nil {
		return nil, err
	}
	return &file.File{
		Name:         name,
		Size:         *resp.ContentLength,
		LastModified: s.lastModified(resp.Metadata, *resp.LastModified),
		Body:         resp.Body,
	}, nil
}

// Delete removes an object from archive.
func (s *Store) Delete(ctx context.Context, key string) error {
	_, err := s.S3.DeleteObjectWithContext(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.Bucket),
		Key:    aws.String(key),
	})
	return err
}

// Search finds an object in storage by prefix and returns an array of matches
func (s *Store) Search(ctx context.Context, prefix string) (file.List, error) {
	var matches file.List
	// Not using v2 because digitalocean doesn't support it.
	// https://developers.digitalocean.com/documentation/spaces/#list-bucket-contents
	if err := s.S3.ListObjectsPagesWithContext(ctx, &s3.ListObjectsInput{
		Bucket:  aws.String(s.Bucket),
		Prefix:  aws.String(prefix),
		MaxKeys: aws.Int64(1000),
	}, func(resp *s3.ListObjectsOutput, _ bool) bool {
		for _, item := range resp.Contents {
			matches = append(matches, &file.File{
				Name: *item.Key,
				Size: *item.Size,
				// TODO: find a way to get metadata for many objects fast.
				LastModified: *item.LastModified,
			})
		}
		return true
	}); err != nil {
		return nil, err
	}
	sort.Sort(matches)
	return matches, nil
}

// Concat an array of byte arrays ordered identically with the input files
// supplied. Note that this loads the entire dataset into memory.
func (s *Store) Concat(ctx context.Context, concurrency int, files []string) ([][]byte, error) {
	result := make([][]byte, len(files))
	sem := semaphore.NewWeighted(int64(concurrency))
	eg, egCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		for index, item := range files {
			index, item := index, item // https://golang.org/doc/faq#closures_and_goroutines
			if err := sem.Acquire(egCtx, 1); err != nil {
				return err
			}
			eg.Go(func() error {
				defer sem.Release(1)
				resp, err := s.Get(egCtx, item)
				if err != nil {
					return nil
				}
				result[index], err = ioutil.ReadAll(resp.Body)
				if err != nil {
					return nil
				}
				resp.Close()
				return nil
			})
		}
		return nil
	})
	if err := eg.Wait(); err != nil {
		return nil, err
	}
	return result, nil
}

// Stat gets details about an object in the store.
func (s *Store) Stat(ctx context.Context, name string) (*file.File, error) {
	stat, err := s.S3.HeadObjectWithContext(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.Bucket),
		Key:    aws.String(name),
	})
	if err != nil {
		return nil, err
	}
	// TODO: find a way to get metadata for many objects fast.
	return file.NewStub(name, *stat.ContentLength, *stat.LastModified), nil
}
