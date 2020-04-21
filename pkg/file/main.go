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
	"os"
	"strings"
)

const MetaKey = "memorybox"
const MetaFilePrefix = MetaKey + "-meta-"

type File struct {
	meta            map[string]interface{}
	isMetaFile      bool
	name            string
	sys             *System
	backingFilePath string
	reader          io.ReadCloser
	source          string
}

// MetaFileName calculates what the metafile name for a supplied name should be.
func MetaFileName(source string) string {
	return MetaFilePrefix + source
}

func New() *File {
	return &File{
		meta: map[string]interface{}{},
		sys:  NewSystem(),
	}
}

func (f *File) Load(input interface{}) (*File, error) {
	// Start with the assumption this is not a metafile.
	f.isMetaFile = false
	// Save a string representation of the source of the file.
	if _, ok := input.(string); !ok {
		// If we can't cast to a string, use the name of the incoming type.
		f.source = fmt.Sprintf("%T", input)
	} else {
		f.source = fmt.Sprintf("%s", input)
	}
	// Get a io.ReadCloser for the data backing the supplied input.
	reader, filePath, err := f.sys.read(input)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	// Save a reference to the backing file this file points to. In the case of
	// inputs sourced from the local filesystem, this will exactly match the
	// input to this function. In the case of files sourced over the network or
	// from memory, this will point to a temporary file that was created to
	// allow the contents to be read more than once.
	f.backingFilePath = filePath
	// Hash the contents of the reader we have obtained.
	digest, size, digestErr := sha256(reader)
	if digestErr != nil {
		return nil, fmt.Errorf("hashing: %s", digestErr)
	}
	// Name file the same as the digest of its content.
	f.name = digest
	// If the size of our file is less than 1mb, load it into ram to see if is
	// a meta-file.
	if size < (1 * 1024 * 1024) {
		metaCheck, metaCheckErr := f.sys.Open(filePath)
		if metaCheckErr != nil {
			return nil, fmt.Errorf("metacheck: %s", metaCheckErr)
		}
		defer metaCheck.Close()
		// If the input is JSON encoded AND it has a MetaKey value, turn this
		// into a metafile.
		if err := json.NewDecoder(metaCheck).Decode(&f.meta); err == nil {
			if name, ok := f.meta[MetaKey].(string); ok {
				f.name = name
				f.isMetaFile = true
			}
		}
	}
	return f, nil
}

// Name returns the underlying name of the file.
func (f *File) Name() string {
	if f.isMetaFile {
		return f.MetaName()
	}
	return f.name
}

// MetaName calculates what the metafile name for a given file should be.
func (f *File) MetaName() string {
	return MetaFileName(f.name)
}

func (f *File) SetMeta(key string, value interface{}) {
	// Don't allow changing this key, we control it.
	if key == MetaKey {
		return
	}
	// If incoming value is a valid JSON string, store it as JSON, not a string.
	if valueAsString, ok := value.(string); ok {
		jsonValue := &json.RawMessage{}
		if err := json.Unmarshal([]byte(valueAsString), &jsonValue); err == nil {
			f.meta[key] = jsonValue
			return
		}
	}
	// Store value in exact form it arrived.
	f.meta[key] = value
}

func (f *File) DeleteMeta(key string) {
	// Don't allow changing this key, we control it.
	if key == MetaKey {
		return
	}
	delete(f.meta, key)
}

func (f *File) GetMeta(key string) interface{} {
	return f.meta[key]
}

func (f *File) IsMetaFile() bool {
	return f.isMetaFile
}

func (f *File) Source() string {
	return f.source
}

func (f *File) NewMetaFile() *File {
	if f.isMetaFile {
		return f
	}
	f.meta[MetaKey] = f.name
	return &File{
		isMetaFile: true,
		name:       f.name,
		meta:       f.meta,
	}
}

// Read calls read on the underlying reader of the file. If the file is a
// metafile the reader will contain a json encoded representation of `.meta` as
// of the first time Read is called.
func (f *File) Read(p []byte) (int, error) {
	// If we don't have a reader yet, create one.
	if f.reader == nil {
		if f.IsMetaFile() {
			// If this is a metafile, the reader should contain a json
			// serialization of the meta struct field.
			json, jsonErr := json.Marshal(f.meta)
			if jsonErr != nil {
				return 0, jsonErr
			}
			f.reader = ioutil.NopCloser(bytes.NewBuffer(json))
		} else {
			// Otherwise, the reader is a file on disk, open it.
			reader, err := f.sys.Open(f.backingFilePath)
			if err != nil {
				return 0, err
			}
			f.reader = reader
		}
	}
	return f.reader.Read(p)
}

func (f *File) Close() error {
	if f.sys != nil {
		err := os.RemoveAll(f.sys.TempDir)
		if err != nil {
			return err
		}
	}
	if f.reader == nil {
		return os.ErrInvalid
	}
	return f.reader.Close()
}

func (f *File) TestSetSystem(sys *System) {
	f.sys = sys
}

func (f *File) TestSetReader(reader io.ReadCloser) {
	f.reader = reader
}

// System defines a set of methods for network and disk io. This is an attempt
// to make the thinnest possible abstraction to support mocking to achieve 100%
// test coverage without introducing a runtime dependency on a mocking library.
type System struct {
	Get         func(url string) (*http.Response, error)
	Open        func(string) (*os.File, error)
	Stdin       io.ReadCloser
	TempFile    func(string, string) (*os.File, error)
	TempDirBase string
	TempDir     string
}

func NewSystem() *System {
	return &System{
		Get:         http.Get,
		Open:        os.Open,
		Stdin:       os.Stdin,
		TempFile:    ioutil.TempFile,
		TempDirBase: os.TempDir(),
		TempDir:     "",
	}
}

// read produces an io.ReadCloser from any supported source and ensures
// the backing data can be read multiple times.
func (sys *System) read(src interface{}) (io.ReadCloser, string, error) {
	// Ensure we have a temporary directory. This is here to allow System to be
	// instantiated without error conditions.
	if sys.TempDir == "" {
		tempDir, err := ioutil.TempDir(sys.TempDirBase, "*")
		if err != nil {
			return nil, "", err
		}
		sys.TempDir = tempDir
	}
	// If the input is an io.ReadCloser already, create a temporary file that
	// will be populated by reading it.
	if reader, ok := src.(io.ReadCloser); ok {
		return sys.teeTempReader(reader, sys.TempDir)
	}
	input, ok := src.(string)
	if !ok {
		return nil, "", fmt.Errorf("unsupported source type: %T", src)
	}
	// If the input string was determined to represent stdin, create a temporary
	// file that will be populated by reading it.
	if inputIsStdin(input) {
		return sys.teeTempReader(sys.Stdin, sys.TempDir)
	}
	// If the input string was determined to be a URL, attempt a http request to
	// get the contents and create a temporary file that will be populated with
	// the body of the request as it is read.
	if inputIsURL(input) {
		resp, err := sys.Get(input)
		if err != nil {
			return nil, "", err
		}
		if !(resp.StatusCode >= 200 && resp.StatusCode <= 299) {
			return nil, "", fmt.Errorf("http code: %d", resp.StatusCode)
		}
		return sys.teeTempReader(resp.Body, sys.TempDir)
	}
	// If we made it here, assume the input is on disk and just open it.
	reader, err := sys.Open(input)
	if err != nil {
		return nil, "", err
	}
	return reader, input, nil
}

// teeTempReader returns an io.readCloser that, when read, will populate a temp
// file in the supplied folder with the exact content that was read. See the
// read method to understand why this exists.
func (sys *System) teeTempReader(reader io.ReadCloser, tempDir string) (io.ReadCloser, string, error) {
	file, err := sys.TempFile(tempDir, "*")
	if err != nil {
		return nil, "", err
	}
	tee := io.TeeReader(reader, file)
	return ioutil.NopCloser(tee), file.Name(), nil
}

// inputIsStdin determines if a provided input points to data arriving over
// stdin. Per common convention, we recognize a single dash ("-") as meaning
// this.
func inputIsStdin(input string) bool {
	return input == "-"
}

// inputIsURL determines if we can find our input by making a http request.
func inputIsURL(input string) bool {
	return strings.HasPrefix(input, "http://") ||
		strings.HasPrefix(input, "https://")
}

// sha256 computes a sha256 message digest for a provided io.readCloser.
func sha256(source io.Reader) (string, int64, error) {
	hash := hash.New()
	size, err := io.Copy(hash, source)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(hash.Sum(nil)) + "-sha256", size, nil
}
