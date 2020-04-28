package archive

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
)

type hashFn func(io.Reader) (string, int64, error)

// File satisfies the io.ReadCloser interface and provides memorybox-specific
// additions for management of metadata / fetching content that is not already
// on the machine where memorybox is being run.
type File struct {
	// Meta holds metadata about this file, some managed by users, some managed
	// by memorybox itself.
	meta []byte
	// hashFn computes the content-hash of the supplied reader and returns a
	// string valued result along with the size of the input.
	hashFn hashFn
	// Name is the name of the file as computed by sending entire content of the
	// file through hashFn.
	name string
	// Source is a string representation of where the File "came from" and is
	// used primarily for logging.
	source string
	// Size holds the size of the file. Used primarily to populate metadata
	// about the file.
	size int64
	// Filepath indicates where the data this File represents can be found.
	// In the case of data arriving from the network or stdin, this will be a
	// path to a temporary file where the data is buffered for consumers to
	// read (the original data stream is consumed to calculate the filename
	// when the instance is created).
	filepath string
	// See Read method.
	reader io.ReadCloser
	sys    *sys
}

// MetaKey is a unique key that appears in metadata files (which are plain text
// files containing json).
const MetaKey = "memorybox"

// MetaFilePrefix controls naming for metadata files (which are named the
// same as the file they describe plus this prefix).
const MetaFilePrefix = MetaKey + "-meta-"

// MetaFileMaxSize defines the maximum size allowed for metadata files. This
// is only enforced to prevent memorybox from decoding potentially huge JSON
// blobs to see if they are memorybox metadata vs just regular ol' json. This
// value can be increased if a real world use-case dictates it.
const MetaFileMaxSize = 1 * 1024 * 1024

// sys defines a set of methods for network and disk io. This is an attempt to
// make the thinnest possible abstraction to support achieving 100% test
// coverage without a runtime dependency on a mocking library.
// Note:
// There appears to be no coherent way in golang to mock *os.File, nor a virtual
// filesystem implementation available in the standard library. For more info,
// read these links:
// https://github.com/golang/go/issues/14106
// https://github.com/golang/go/issues/21592
type sys struct {
	// For real or mocked HTTP get requests.
	Get func(url string) (*http.Response, error)
	// For real or mocked opening of files.
	Open     func(string) (*os.File, error)
	ReadFile func(string) ([]byte, error)
	// For real or mocked data arriving on stdin.
	Stdin io.ReadCloser
	// For real or mocked interactions with temporary files.
	TempFile    func(string, string) (*os.File, error)
	TempDirBase string
	TempDir     string
}

// new returns a bare file ready for initialization.
func new(ctx context.Context, hashFn func(io.Reader) (string, int64, error)) *File {
	return &File{
		meta:   []byte(`{"data":{}}`),
		hashFn: hashFn,
		sys: &sys{
			Get: func(url string) (*http.Response, error) {
				client := retryablehttp.NewClient()
				client.Logger = log.New(ioutil.Discard, "", 0)
				request, err := retryablehttp.NewRequest("GET", url, nil)
				if err != nil {
					return nil, err
				}
				return client.Do(request.WithContext(ctx))
			},
			Open:        os.Open,
			ReadFile:    ioutil.ReadFile,
			Stdin:       os.Stdin,
			TempFile:    ioutil.TempFile,
			TempDirBase: os.TempDir(),
			TempDir:     "",
		},
	}
}

// New creates a new File instance and populates its fields by fetching/reading
// the input data supplied to this function. New accepts values that denote
// local files (path/to/file), stdin (-), or valid urls.
func New(ctx context.Context, hashFn func(io.Reader) (string, int64, error), input string) (*File, error) {
	return new(ctx, hashFn).init(ctx, input)
}

// NewFromReader uses the supplied input as the content of the file.
func NewFromReader(ctx context.Context, hashFn func(io.Reader) (string, int64, error), input io.ReadCloser) (*File, error) {
	return new(ctx, hashFn).init(ctx, input)
}

// NewMetaFile creates a metadata "pair" for a source data file. When metadata
// files are read, they stream a json representation of their meta field.
func NewMetaFile(source *File) *File {
	// Metafiles have the same name as the data file they describe + a prefix.
	name := ToMetaFileName(source.name)
	// No network activity is required for these so the context is invented.
	f := new(context.Background(), source.hashFn)
	f.source = name
	f.name = name
	// If the source file had metadata set on it, bring it along too.
	f.meta = source.meta
	// Some workflows can call this function on a file whose contents are a
	// metafile already. If this field already has a value, it is one of them.
	if !gjson.GetBytes(f.meta, MetaKey).Exists() {
		f.meta, _ = sjson.SetBytes(f.meta, MetaKey+".file", source.name)
		f.meta, _ = sjson.SetBytes(f.meta, MetaKey+".source", source.source)
		f.meta, _ = sjson.SetBytes(f.meta, MetaKey+".size", source.size)
	}
	return f
}

// MetaFileName calculates a metafile name for a data file.
func MetaFileName(source string) string {
	return MetaFilePrefix + source
}

// init prepares the internal state of a File for consumption.
func (f *File) init(ctx context.Context, input interface{}) (result *File, err error) {
	// Ensure temporary files are cleaned up even if this fails.
	defer func() {
		if err != nil {
			f.Close()
		}
	}()
	// Save a string representation of the source of the file. This is useful
	// for consumers who may wish to know the origin of the file.
	if _, ok := input.(string); !ok {
		// Use the name of the incoming type if the input is not a string.
		f.source = fmt.Sprintf("%T", input)
	} else {
		f.source = fmt.Sprintf("%s", input)
	}
	// Get a reader for the data backing the supplied input.
	reader, filepath, readerErr := f.fetch(ctx, input)
	if readerErr != nil {
		err = readerErr
		return nil, fmt.Errorf("fetch: %w", readerErr)
	}
	defer reader.Close()
	// Save a reference to the file the input points to. In the case of inputs
	// that reference files from from the local machine, this will exactly match
	// the input to this function. In the case of inputs sourced over a network
	// or from memory, this will point to a temporary file that was created to
	// allow the contents to be read again by consumers.
	f.filepath = filepath
	// Read the entire input and hash the contents to determine the file's name.
	digest, size, digestErr := f.hashFn(reader)
	if digestErr != nil {
		return nil, fmt.Errorf("hashing: %w", digestErr)
	}
	// Name file the same as the digest of its content.
	f.name = digest
	// Capture the size of the file so we can record it in our metadata when a
	// metafile is generated from this instance.
	f.size = size
	if size < MetaFileMaxSize {
		// Check to see if the content referred to by our input is metadata in
		// the memorybox format (aka: json).
		bytes, readErr := f.sys.ReadFile(f.filepath)
		if readErr != nil {
			return nil, fmt.Errorf("detecting metafile: %w", readErr)
		}
		if gjson.ValidBytes(bytes) && gjson.GetBytes(bytes, MetaKey).Exists() {
			// At this point we have confirmed that the data from our
			// input has memorybox metadata in it. That means the MetaKey
			// contains the name of the file being described. Rename the
			// file to match.
			f.name = gjson.GetBytes(bytes, MetaKey+".file").String()
			// Set the metadata of this file to match the content.
			f.meta = bytes
			// Clean up temporary files created so far.
			f.Close()
			// Swap this file for a metadata representation of it. This
			// could actually just be done inline here but it seems
			// better to use the publicly accessible method to do so.
			*f = *NewMetaFile(f)
		}
	}
	return f, nil
}

// fetch produces an io.ReadCloser from any supported source,
func (f *File) fetch(ctx context.Context, input interface{}) (io.ReadCloser, string, error) {
	// If the input is an io.ReadCloser already, create a temporary file that
	// will be populated by reading it.
	if reader, ok := input.(io.ReadCloser); ok {
		return f.teeTempFileReader(reader)
	}
	// The only other supported type for an input source is a string.
	src, _ := input.(string)
	// If the input string is determined to represent stdin, create a temporary
	// file that will be populated by reading it. Per common convention, a dash
	// ("-") is used for this.
	if src == "-" {
		return f.teeTempFileReader(f.sys.Stdin)
	}
	// If the input string is determined to be a URL, attempt a http request to
	// get the contents and create a temporary file that will be populated with
	// the body of the request as it is read.
	if u, err := url.Parse(src); err == nil && u.Scheme != "" && u.Host != "" {
		resp, err := f.sys.Get(src)
		if err != nil {
			return nil, "", err
		}
		if !(resp.StatusCode >= 200 && resp.StatusCode <= 299) {
			return nil, "", fmt.Errorf("http code: %d", resp.StatusCode)
		}
		return f.teeTempFileReader(resp.Body)
	}
	// If here, assume the input is on disk and open the file it points to.
	reader, err := f.sys.Open(src)
	if err != nil {
		return nil, "", err
	}
	return reader, src, nil
}

// teeTempFileReader returns an io.ReadCloser that, when read, will populate a
// temp file with the exact content that was read. See the fetch method for more
// details about why this exists.
func (f *File) teeTempFileReader(reader io.ReadCloser) (io.ReadCloser, string, error) {
	if f.sys.TempDir == "" {
		tempDir, err := ioutil.TempDir(f.sys.TempDirBase, "*")
		if err != nil {
			return nil, "", err
		}
		f.sys.TempDir = tempDir
	}
	file, err := f.sys.TempFile(f.sys.TempDir, "*")
	if err != nil {
		return nil, "", err
	}
	tee := io.TeeReader(reader, file)
	return ioutil.NopCloser(tee), file.Name(), nil
}

// Name returns the underlying name of the file.
func (f *File) Name() string {
	return f.name
}

// IsMetaDataFile checks to see if the immutable "MetaKey" has been set in the
// File's meta field. This can only be set when the file is created by calling
// NewMetaFile with a data file or New with an input source whose data is valid
// memorybox metadata.
func (f *File) IsMetaDataFile() bool {
	return gjson.GetBytes(f.meta, MetaKey).Exists()
}

// Source returns a string representation of the input that was used when the
// File was created.
func (f *File) Source() string {
	return f.source
}

// Read calls read on the underlying reader of the file.
func (f *File) Read(p []byte) (int, error) {
	// A reader is created the first time Read is called because its content may
	// be driven by internal state that can change between the time a File is
	// instantiated and when the content it represents is consumed.
	if f.reader == nil {
		if f.IsMetaDataFile() {
			f.reader = ioutil.NopCloser(bytes.NewReader(f.meta))
		} else {
			// Otherwise, the reader is simply a file on disk. Open it.
			reader, err := f.sys.Open(f.filepath)
			if err != nil {
				return 0, err
			}
			f.reader = reader
		}
	}
	return f.reader.Read(p)
}

// Close cleans up any temporary files that were created to support this File
// instance. If there is a reader associated with this file, it also calls close
// on that.
func (f *File) Close() error {
	if f.sys != nil {
		err := os.RemoveAll(f.sys.TempDir)
		if err != nil {
			return err
		}
	}
	if f.reader == nil {
		return nil
	}
	return f.reader.Close()
}

// MetaSet assigns metadata for a file. If the input is a string value that is
// valid json, it is converted to be stored as json.
func (f *File) MetaSet(key string, value string) {
	// Don't allow changing this key, it is managed internally.
	if key == MetaKey {
		return
	}
	path := "data"
	if key != "" {
		path = path + "." + key
	}
	jsonValue := json.RawMessage{}
	if err := json.Unmarshal([]byte(value), &jsonValue); err == nil {
		f.meta, _ = sjson.SetBytes(f.meta, path, jsonValue)
		return
	}
	f.meta, _ = sjson.SetBytes(f.meta, path, value)
}

// MetaDelete removes metadata from a file by key.
func (f *File) MetaDelete(key string) {
	// Don't allow changing this key, it is managed internally.
	if key == MetaKey {
		return
	}
	f.meta, _ = sjson.DeleteBytes(f.meta, "data."+key)
}

// MetaGet fetches metadata from a file by key.
func (f *File) MetaGet(key string) interface{} {
	var value gjson.Result
	var result json.RawMessage
	if key == MetaKey {
		value = gjson.GetBytes(f.meta, MetaKey)
	}
	value = gjson.GetBytes(f.meta, "data."+key)
	if !value.Exists() {
		return nil
	}
	if err := json.Unmarshal([]byte(value.String()), &result); err == nil {
		return result
	}
	return value.String()
}
