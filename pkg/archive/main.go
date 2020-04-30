package archive

import (
	"encoding/hex"
	"encoding/json"
	"github.com/mattetti/filebuffer"
	hash "github.com/minio/sha256-simd"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"io"
	"io/ioutil"
	"strings"
)

// File satisfies the io.ReadCloser interface and provides memorybox-specific
// additions for management of metadata.
type File struct {
	// Meta holds metadata about this file, some managed by users, some managed
	// by memorybox itself.
	meta []byte
	// Name is the name of the file as computed by sending entire content of the
	// file through a hashing function.
	name string
	// Source is a string representation of where the File "came from" and is
	// used primarily for logging.
	source string
	// Size caches the size of the file. Used to populate metadata about the
	// file.
	size int64
	// The backing data for this file is stored here.
	data io.ReadSeeker
	// It means just what you think.
	isMetaFile bool
	// When this file is in "metafile" mode, the first Read call populates the
	// internal reader with the current contents of meta. This tracks if that
	// event has happened yet.
	hasBeenRead bool
}

// MetaKey is the key in metadata json files under which memorybox controls the
// content automatically.
const MetaKey = "memorybox"

// MetaFileNameKey refers to the location where memorybox stores the name of the
// datafile that a metafile describes.
const MetaFileNameKey = "memorybox.file"

// MetaFileSourceKey refers to the location where memorybox stores a string
// value that represents the original source a user supplied when putting a
// datafile into the store.
const MetaFileSourceKey = "memorybox.source"

// MetaFileSizeKey refers to the filesize of the datafile a metafile describes.
const MetaFileSizeKey = "memorybox.size"

// MetaFilePrefix controls naming for metadata files (which are named the
// same as the file they describe plus this prefix).
const MetaFilePrefix = MetaKey + "-meta-"

// MetaFileMaxSize defines the maximum size allowed for metadata files. This
// is only enforced to prevent memorybox from decoding potentially huge JSON
// blobs to see if they are memorybox metadata vs just regular ol' json. This
// value can be increased if a real world use-case dictates it.
const MetaFileMaxSize = 1 * 1024 * 1024

// Hasher describes a method that will take a reader and compute a hash of its
// contents, returning the result as a string and the size of the data that was
// read.
type Hasher func(source io.Reader) (string, int64, error)

// Sha256 computes a sha256 message digest for a provided io.Reader.
func Sha256(source io.Reader) (string, int64, error) {
	hash := hash.New()
	size, err := io.Copy(hash, source)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(hash.Sum(nil)) + "-sha256", size, nil
}

// MetaFileNameFrom calculates a metafile name for a data file.
func MetaFileNameFrom(source string) string {
	if !IsMetaFileName(source) {
		return MetaFilePrefix + source
	}
	return source
}

// DataFileNameFrom calculates a datafile name from a metafile name.
func DataFileNameFrom(source string) string {
	return strings.TrimPrefix(source, MetaFilePrefix)
}

// IsMetaFileName determines if a given source string is named like a metafile.
func IsMetaFileName(source string) bool {
	return strings.HasPrefix(source, MetaFilePrefix)
}

// IsMetaData determines if a given set of bytes contains json that matches
// heuristics this package considers "metadata" (that is, a json object with a
// "memorybox" key).
func IsMetaData(bytes []byte) bool {
	return gjson.ValidBytes(bytes) && gjson.GetBytes(bytes, MetaKey).Exists()
}

// new creates a bare file.
func new(source string) *File {
	return &File{
		meta:   []byte(`{}`),
		source: source,
	}
}

// New creates a new File instance.
func New(source string, data io.ReadSeeker, hash Hasher) (*File, error) {
	f := new(source)
	// Hash the contents of the file to determine its name.
	digest, size, err := hash(data)
	if err != nil {
		return nil, err
	}
	// Ensure the file can be read again.
	data.Seek(0, io.SeekStart)
	// Save the file so it can be read later.
	f.data = data
	// Name file the same as the digest of its content.
	f.name = digest
	// Capture the size of the file so it can be recorded in metadata.
	f.size = size
	// If the file is below a size threshold, check the content to see if it is
	// formatted as a memorybox metadata file.
	if size < MetaFileMaxSize {
		bytes, err := ioutil.ReadAll(data)
		if err != nil {
			return nil, err
		}
		// Ensure the reader can be read again.
		data.Seek(0, 0)
		if IsMetaData(bytes) {
			// The metadata contains the name of the file it describes. Use it.
			f.name = gjson.GetBytes(bytes, MetaFileNameKey).String()
			// Set the metadata of this file to match the content.
			f.meta = bytes
			// Mark this a a metafile so converting it doesn't overwrite
			f.isMetaFile = true
		}
	}
	return f, nil
}

// NewSha256 creates a file using the Sha256 hashing algorithm.
func NewSha256(source string, data io.ReadSeeker) (*File, error) {
	return New(source, data, Sha256)
}

// MetaFile creates a metadata "pair" for a source data file. When metadata
// files are read, they stream a json representation of their meta field.
func (f *File) MetaFile() *File {
	// Metafiles have the same name as the data file they describe + a prefix.
	name := MetaFileNameFrom(f.name)
	metaFile := new(f.source)
	metaFile.name = name
	metaFile.isMetaFile = true
	// If the source file had metadata set on it, bring a copy.
	metaFile.meta = f.meta
	if !f.IsMetaFile() {
		// Assign values for memory-box-managed keys.
		metaFile.meta, _ = sjson.SetBytes(metaFile.meta, MetaFileNameKey, f.name)
		metaFile.meta, _ = sjson.SetBytes(metaFile.meta, MetaFileSourceKey, f.source)
		metaFile.meta, _ = sjson.SetBytes(metaFile.meta, MetaFileSizeKey, f.size)
	}
	return metaFile
}

// Name returns the underlying name of the file.
func (f *File) Name() string {
	return f.name
}

// Source returns the underlying source value.
func (f *File) Source() string {
	return f.source
}

// IsMetaFile does just what you think it does.
func (f *File) IsMetaFile() bool {
	return f.isMetaFile
}

// Read calls read on the underlying reader of the file.
func (f *File) Read(p []byte) (int, error) {
	if !f.hasBeenRead && f.IsMetaFile() {
		// When this is a "metafile" the content can change between the time
		// the file is instantiated and when it is consumed. At the point of
		// first reading, this ensures we read from meta as it existed when
		// reading started.
		f.data = filebuffer.New(f.meta)
	}
	f.hasBeenRead = true
	return f.data.Read(p)
}

// MetaSet assigns metadata for a file. If the input is a string value that is
// valid json, it is converted to be stored as json.
func (f *File) MetaSet(key string, value string) {
	// Managed internally.
	if strings.Contains(key, MetaKey) {
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
	// Managed internally.
	if strings.Contains(key, MetaKey) {
		return
	}
	f.meta, _ = sjson.DeleteBytes(f.meta, "data."+key)
}

// MetaGet fetches metadata from a file by key.
func (f *File) MetaGet(key string) interface{} {
	var value gjson.Result
	var result json.RawMessage
	if strings.Contains(key, MetaKey) {
		value = gjson.GetBytes(f.meta, key)
	} else {
		value = gjson.GetBytes(f.meta, "data."+key)
	}
	if !value.Exists() {
		return nil
	}
	if err := json.Unmarshal([]byte(value.String()), &result); err == nil {
		return result
	}
	return value.String()
}