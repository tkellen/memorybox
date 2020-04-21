package file

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
)

const MetaKey = "memorybox"
const MetaFilePrefix = MetaKey + "-meta-"

type File struct {
	meta            map[string]interface{}
	isMetaFile      bool
	name            string
	io              *system
	backingFilePath string
	reader          io.ReadCloser
	source          string
}

// MetaFileName calculates what the metafile name for a supplied name should be.
func MetaFileName(source string) string {
	return MetaFilePrefix + source
}

func New(input interface{}) (*File, error) {
	sys, sysErr := newSystem()
	if sysErr != nil {
		return nil, sysErr
	}
	file := &File{
		meta:   map[string]interface{}{},
		io:     sys,
		source: fmt.Sprintf("%s", input),
	}
	// Go get backing data.
	reader, tempFile, err := file.io.read(input)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	// Hash the contents of the reader we have obtained.
	digest, _, digestErr := sha256(reader)
	if digestErr != nil {
		return nil, fmt.Errorf("hashing: %s", digestErr)
	}
	// Name file the same as the digest of its content.
	file.name = digest
	file.backingFilePath = tempFile
	// See if input is JSON encoded (maybe it is a meta file)
	metaCheck, metaCheckErr := file.io.Open(tempFile)
	if metaCheckErr != nil {
		return nil, metaCheckErr
	}
	defer metaCheck.Close()
	// If the input is JSON encoded AND it has a MetaKey value, we indeed
	// have a metafile.
	if err := json.NewDecoder(metaCheck).Decode(&file.meta); err == nil {
		if name, ok := file.meta[MetaKey].(string); ok {
			file.name = name
			file.isMetaFile = true
		}
	}
	return file, nil
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
			reader, err := f.io.Open(f.backingFilePath)
			if err != nil {
				return 0, err
			}
			f.reader = reader
		}
	}
	return f.reader.Read(p)
}

func (f *File) Close() error {
	if f.io != nil {
		err := os.RemoveAll(f.io.TempDir)
		if err != nil {
			return err
		}
	}
	if f.reader == nil {
		return os.ErrInvalid
	}
	return f.reader.Close()
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
	return &File{
		isMetaFile: true,
		name:       f.name,
		meta: map[string]interface{}{
			MetaKey: f.name,
		},
	}
}
