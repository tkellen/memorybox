package file

import (
	"encoding/json"
	"fmt"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"strings"
	"time"
)

// MetaFileMaxSize defines the maximum size allowed for metadata files. This
// is only enforced to prevent memorybox from decoding potentially huge JSON
// blobs to see if they are memorybox metadata vs just regular ol' json. This
// value can be increased if a real world use-case dictates it.
const MetaFileMaxSize = 256 * 1024

// MetaFilePrefix controls naming for metadata files (which are named the
// same as the file they describe plus this prefix).
const MetaFilePrefix = "memorybox-meta-"

// MetaKey is the key in metadata json files under which memorybox controls the
// content automatically.
const MetaKey = "memorybox"

// MetaKeyFileName refers to the location where memorybox stores the name of the
// datafile that a metafile describes.
const MetaKeyFileName = MetaKey + ".file"

// MetaKeySource refers to the location where memorybox stores a string value
// that represents the original source a user supplied when putting a datafile
// into the store.
const MetaKeySource = MetaKey + ".source"

// MetaKeyImport refers to the location where memorybox stores details about
// when a file was imported.
const MetaKeyImport = MetaKey + ".import"

// MetaKeyImportFrom refers to the location where memorybox stores details about
// what grouping of files a given file was imported with.
const MetaKeyImportFrom = MetaKeyImport + ".from"

// Meta holds JSON encoded metadata.
type Meta []byte

// NewMetaFromFile produces memorybox formatted metadata from a supplied file.
func NewMetaFromFile(file *File) *Meta {
	data, _ := sjson.SetBytes([]byte{}, "memorybox", map[string]interface{}{
		"source": file.Source,
		"file":   file.Name,
		"import": map[string]interface{}{
			"at": time.Now().UTC().Format(time.RFC3339),
		},
	})
	meta := Meta(data)
	return &meta
}

// IsMetaFileName determines if a given source string is named like a metafile.
func IsMetaFileName(source string) bool {
	return strings.HasPrefix(source, MetaFilePrefix)
}

// MetaNameFrom calculates a metafile name for a data file.
func MetaNameFrom(source string) string {
	if !IsMetaFileName(source) {
		return MetaFilePrefix + source
	}
	return source
}

// DataNameFrom calculates a datafile name from a metafile name.
func DataNameFrom(source string) string {
	return strings.TrimPrefix(source, MetaFilePrefix)
}

// ValidateMeta determines if a given set of bytes matches the metaFile format.
func ValidateMeta(bytes []byte) error {
	if len(bytes) > MetaFileMaxSize {
		return fmt.Errorf("too large by %d bytes", len(bytes)-MetaFileMaxSize)
	}
	if !gjson.ValidBytes(bytes) {
		return fmt.Errorf("not json encoded")
	}
	if !gjson.GetBytes(bytes, MetaKey).Exists() {
		return fmt.Errorf("missing %s", MetaKey)
	}
	return nil
}

// String converts the underlying byte array string form.
func (m *Meta) String() string { return fmt.Sprintf("%s", *m) }

// DataFileName extracts the datafile this metadata describes.
func (m Meta) DataFileName() string {
	return gjson.GetBytes(m, MetaKeyFileName).String()
}

// Source extracts the original source of the datafile this metadata describes.
func (m Meta) Source() string {
	return gjson.GetBytes(m, MetaKeySource).String()
}

// Get retrieves a value from the json-encoded byte array.
func (m *Meta) Get(key string) interface{} {
	var value gjson.Result
	var result json.RawMessage
	value = gjson.GetBytes(*m, key)
	if !value.Exists() {
		return nil
	}
	if err := json.Unmarshal([]byte(value.String()), &result); err == nil {
		return result
	}
	return value.String()
}

// Set persists a value in the json-encoded byte array.
func (m *Meta) Set(key string, value string) {
	if key == "" {
		return
	}
	jsonValue := json.RawMessage{}
	if err := json.Unmarshal([]byte(value), &jsonValue); err == nil {
		*m, _ = sjson.SetBytes(*m, key, jsonValue)
		return
	}
	*m, _ = sjson.SetBytes(*m, key, value)
}

// Delete removes a value from the json-encoded byte array.
func (m *Meta) Delete(key string) {
	*m, _ = sjson.DeleteBytes(*m, key)
}

// Merge takes an object and assigns every key into the meta field except
// managed ones.
func (m *Meta) Merge(data string) error {
	jsonData, ok := gjson.Parse(data).Value().(map[string]interface{})
	if !ok {
		return fmt.Errorf("%s is not valid json", data)
	}
	for key, value := range jsonData {
		if strings.HasPrefix(key, MetaKey) {
			continue
		}
		*m, _ = sjson.SetBytes(*m, key, value)
	}
	return nil
}
