package file

import (
	"encoding/hex"
	"fmt"
	hash "github.com/minio/sha256-simd"
	"io"
	"io/ioutil"
	"os"
	"time"
)

type readSeekCloser interface {
	io.ReadSeeker
	io.Closer
}

type HashFn func(io.Reader) (string, int64, error)

// File is an OS and storage system agnostic representation of a file.
type File struct {
	Name         string
	Source       string
	Size         int64
	LastModified time.Time
	Body         io.Reader
	Meta         *Meta
}

// NewStub produces a file that can be instantiated with details from a stat
// call.
func NewStub(name string, size int64, lastModified time.Time) *File {
	return &File{
		Name:         name,
		Size:         size,
		LastModified: lastModified,
	}
}

func NewSha256(source string, body readSeekCloser, lastModified time.Time) (*File, error) {
	return New(source, body, lastModified, Sha256)
}

// New creates a new instance of a file and names it by hashing the content of
// the supplied reader.
func New(source string, body readSeekCloser, lastModified time.Time, hash HashFn) (*File, error) {
	digest, size, hashErr := hash(body)
	if hashErr != nil {
		return nil, hashErr
	}
	// Prevent creating a file from a source containing metadata.
	if size < MetaFileMaxSize {
		body.Seek(0, io.SeekStart)
		if meta, err := ioutil.ReadAll(body); err != nil {
			return nil, err
		} else if ValidateMeta(meta) == nil {
			return nil, fmt.Errorf("%w: use sync to interact with metafiles directly", os.ErrInvalid)
		}
	}
	body.Seek(0, io.SeekStart)
	file := &File{
		Name:         digest,
		Source:       source,
		Size:         size,
		LastModified: lastModified,
		Body:         body,
	}
	file.Meta = NewMetaFromFile(file)
	return file, nil
}

// Close calls close on the underlying Body (if there is one and it is needed).
func (f *File) Close() error {
	if f.Body != nil {
		if asCloser, ok := f.Body.(io.ReadCloser); ok {
			return asCloser.Close()
		}
	}
	return nil
}

// Read calls read on the underlying Body (if there is one).
func (f *File) Read(p []byte) (int, error) {
	if f.Body == nil {
		return 0, io.ErrUnexpectedEOF
	}
	return f.Body.Read(p)
}

// CurrentWith calculates if an alternative file is considered to be "current"
// with this one. This is used by the sync system to determine if a file in one
// store should be copied to another.
func (f *File) CurrentWith(other *File) bool {
	if IsMetaFileName(f.Name) {
		diff := f.LastModified.Sub(other.LastModified)
		otherIsSameOrOlder := (time.Duration(0) * time.Second) <= diff
		return f.Size == other.Size && otherIsSameOrOlder
	} else {
		return f.Size == other.Size
	}
}

// Sha256 computes a sha256 message digest for a provided io.Reader.
func Sha256(source io.Reader) (string, int64, error) {
	hash := hash.New()
	size, err := io.Copy(hash, source)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(hash.Sum(nil)) + "-sha256", size, nil
}
