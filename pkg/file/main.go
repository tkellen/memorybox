package file

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	hash "github.com/minio/sha256-simd"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
)

// MetaKey is a unique key that appears in metadata files (which are just plain
// text files containing json). The value of this key is the filename of the
// object it describes.
const MetaKey = "memorybox"

// MetaFilePrefix controls naming for metadata files (which are named the
// same as the file they describe plus this prefix).
const MetaFilePrefix = MetaKey + "-meta-"

// MetaFileMaxSize defines the maximum size allowed for metadata files. This
// is only enforced to prevent memorybox from decoding potentially huge JSON
// blobs to see if they are memorybox metadata or just regular ol' json. This
// value can be increased if a real world use-case dictates it.
const MetaFileMaxSize = 1 * 1024 * 1024

// meta describes a structure to hold metadata about a file.
type meta map[string]interface{}

// File satisfies the io.ReadCloser interface and provides memorybox-specific
// additions for management of metadata / fetching content that is not already
// on the machine where memorybox is being run.
type File struct {
	sys      *sys
	name     string
	source   string
	filepath string
	reader   io.ReadCloser
	meta     meta
}

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
	Open func(string) (*os.File, error)
	// For real or mocked data arriving on stdin.
	Stdin io.ReadCloser
	// For real or mocked interactions with temporary files.
	TempFile    func(string, string) (*os.File, error)
	TempDirBase string
	TempDir     string
}

// new returns a bare file ready for initialization.
func new() *File {
	return &File{
		meta: map[string]interface{}{},
		sys: &sys{
			Get:         http.Get,
			Open:        os.Open,
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
func New(input string) (*File, error) {
	return new().init(input)
}

// NewFromReader uses the supplied input as the content of the file.
func NewFromReader(input io.ReadCloser) (*File, error) {
	return new().init(input)
}

// NewMetaFile creates a metadata "pair" for a source data file. When metadata
// files are read, they stream a json representation of their meta field.
func NewMetaFile(source *File) *File {
	// Metafiles have the same name as the data file they describe + a prefix.
	name := MetaFileName(source.name)
	f := new()
	f.source = name
	f.name = name
	// If the source file has had metadata set on it, bring it along too by
	// marshalling/unmarshalling it (deep map copy hack).
	if meta, err := json.Marshal(source.meta); err == nil {
		_ = json.Unmarshal(meta, &f.meta)
	}
	f.meta[MetaKey] = source.name
	return f
}

// MetaFileName calculates a metafile name for a data file.
func MetaFileName(source string) string {
	return MetaFilePrefix + source
}

// init prepares the internal state of a File for consumption.
func (f *File) init(input interface{}) (result *File, err error) {
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
	reader, filepath, readerErr := f.fetch(input)
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
	digest, size, digestErr := sha256(reader)
	if digestErr != nil {
		return nil, fmt.Errorf("hashing: %w", digestErr)
	}
	// Name file the same as the digest of its content.
	f.name = digest
	if size < MetaFileMaxSize {
		// Check to see if the content referred to by our input is metadata in
		// the memorybox format (aka: json).
		reader, openErr := f.sys.Open(f.filepath)
		if openErr != nil {
			return nil, fmt.Errorf("detecting metafile: %w", err)
		}
		defer reader.Close()
		var temp meta
		if err := json.NewDecoder(reader).Decode(&temp); err == nil {
			if name, ok := temp[MetaKey].(string); ok {
				// At this point we have confirmed that the data from our input
				// is memorybox metadata. That means the MetaKey contains the
				// name of the file being described. Rename the file to that.
				f.name = name
				// Set the metadata of this file to match the content.
				f.meta = temp
				// Clean up temporary files created so far.
				f.Close()
				// Swap this file for a metadata representation of it. This
				// could actually just be done inline here but it seems better
				// to use the publicly accessible method to do so.
				*f = *NewMetaFile(f)
			}
		}
	}
	return f, nil
}

// fetch produces an io.ReadCloser from any supported source,
func (f *File) fetch(input interface{}) (io.ReadCloser, string, error) {
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
// NewMetaFile with a data file or New with an input source whose data IS valid
// memorybox metadata.
func (f *File) IsMetaDataFile() bool {
	if _, ok := f.MetaGet(MetaKey).(string); ok {
		return true
	}
	return false
}

// MetaSet assigns metadata for a file. If the input is a string value that is
// valid json, it is converted to be stored as json.
func (f *File) MetaSet(key string, value string) {
	// Don't allow changing this key, it is managed internally.
	if key == MetaKey {
		return
	}
	jsonValue := json.RawMessage{}
	if err := json.Unmarshal([]byte(value), &jsonValue); err == nil {
		f.meta[key] = jsonValue
		return
	}
	f.meta[key] = value
}

// MetaDelete removes metadata from a file by key.
func (f *File) MetaDelete(key string) {
	// Don't allow changing this key, it is managed internally.
	if key == MetaKey {
		return
	}
	delete(f.meta, key)
}

// MetaGet fetches metadata from a file by key.
func (f *File) MetaGet(key string) interface{} {
	return f.meta[key]
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
	// instantiated and when the content represents is consumed.
	if f.reader == nil {
		if f.IsMetaDataFile() {
			// If this is a metafile, make the backing reader be a json byte
			// stream that replicates what is currently stored in the meta
			// field. Errors are ignored here on the faith that the interfaces
			// which allow modification of the meta field will not produce
			// unsafe values.
			json, _ := json.Marshal(f.meta)
			f.reader = ioutil.NopCloser(bytes.NewBuffer(json))
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

// sha256 computes a sha256 message digest for a provided io.ReadCloser.
func sha256(source io.Reader) (string, int64, error) {
	hash := hash.New()
	size, err := io.Copy(hash, source)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(hash.Sum(nil)) + "-sha256", size, nil
}
