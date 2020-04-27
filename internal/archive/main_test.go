package archive_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/google/go-cmp/cmp"
	"github.com/tkellen/memorybox/internal/archive"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"testing"
)

// identityHash is a noop hashing function for testing that returns a string
// value of the input plus a suffix (assumes ASCII input).
func identityHash(source io.Reader) (string, int64, error) {
	bytes, err := ioutil.ReadAll(source)
	if err != nil {
		return "", 0, err
	}
	return string(bytes) + "-identity", int64(len(bytes)), nil
}

func TestFile_NewAndNewFromReader(t *testing.T) {
	tempFileBytes := []byte("test")
	tempFile, tempFileErr := ioutil.TempFile(os.TempDir(), "*")
	if tempFileErr != nil {
		t.Fatalf("test setup: %s", tempFileErr)
	}
	_, writeErr := tempFile.Write(tempFileBytes)
	if writeErr != nil {
		t.Fatalf("test setup: %s", writeErr)
	}
	tempFile.Close()
	defer os.Remove(tempFile.Name())
	table := map[string]struct {
		hashFn                 func(io.Reader) (string, int64, error)
		input                  interface{}
		expectedIsMetaDataFile bool
		expectedSource         string
		expectedErr            error
		expectedBytes          []byte
	}{
		"success from local file": {
			hashFn:                 identityHash,
			input:                  tempFile.Name(),
			expectedIsMetaDataFile: false,
			expectedSource:         tempFile.Name(),
			expectedBytes:          tempFileBytes,
		},
		"success from io.ReadCloser": {
			hashFn:                 identityHash,
			input:                  ioutil.NopCloser(bytes.NewBuffer([]byte("test"))),
			expectedIsMetaDataFile: false,
			expectedSource:         "ioutil.nopCloser",
			expectedBytes:          []byte("test"),
		},
		"fail on invalid file": {
			hashFn:                 identityHash,
			input:                  "/nope/nope/nope/nope/nope",
			expectedIsMetaDataFile: false,
			expectedErr:            os.ErrNotExist,
		},
		"fail on inability to compute content hash": {
			hashFn: func(_ io.Reader) (string, int64, error) {
				return "", 0, errors.New("bad time")
			},
			input:                  ioutil.NopCloser(bytes.NewBuffer([]byte("test"))),
			expectedIsMetaDataFile: false,
			expectedErr:            errors.New("bad time"),
		},
		"json formatted input with memorybox key is metafile": {
			hashFn:                 identityHash,
			input:                  ioutil.NopCloser(bytes.NewReader([]byte(`{"data":{},"memorybox":{"file":"test","size":0,"source":"ioutil.nopCloser"}}`))),
			expectedIsMetaDataFile: true,
			expectedSource:         "memorybox-meta-test",
			expectedBytes:          []byte(`{"data":{},"memorybox":{"file":"test","size":0,"source":"ioutil.nopCloser"}}`),
		},
		"json formatted input without memorybox key is not metafile": {
			hashFn:                 identityHash,
			input:                  ioutil.NopCloser(bytes.NewReader([]byte("{}"))),
			expectedIsMetaDataFile: false,
			expectedSource:         "ioutil.nopCloser",
			expectedBytes:          []byte("{}"),
		},
	}
	for name, test := range table {
		test := test
		t.Run(name, func(t *testing.T) {
			var f *archive.File
			var err error
			if inputIsString, ok := test.input.(string); ok {
				f, err = archive.New(test.hashFn, inputIsString)
			} else {
				f, err = archive.NewFromReader(test.hashFn, test.input.(io.ReadCloser))
			}
			if err != nil && test.expectedErr == nil {
				t.Fatal(err)
			}
			if err != nil && test.expectedErr != nil && !errors.Is(err, test.expectedErr) && !strings.Contains(err.Error(), test.expectedErr.Error()) {
				t.Fatalf("expected error: %s, got %s", test.expectedErr, err)
			}
			if err == nil {
				defer f.Close()
				actualSource := f.Source()
				if diff := cmp.Diff(test.expectedSource, actualSource); diff != "" {
					t.Fatal(diff)
				}
				if test.expectedIsMetaDataFile != f.IsMetaDataFile() {
					t.Fatalf("expected IsMetaDataFile to be %v, got %v", test.expectedIsMetaDataFile, f.IsMetaDataFile())
				}
				actualBytes, readErr := ioutil.ReadAll(f)
				if readErr != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(test.expectedBytes, actualBytes) {
					t.Fatalf("expected bytes %s, got %s", test.expectedBytes, actualBytes)
				}
			}
		})
	}
}

func TestNewSetsNameAsHash(t *testing.T) {
	table := map[string]struct {
		hashFn   func(io.Reader) (string, int64, error)
		input    io.ReadCloser
		expected string
	}{
		"hash is calculated for input test": {
			hashFn:   identityHash,
			input:    ioutil.NopCloser(bytes.NewReader([]byte("test"))),
			expected: "test-identity",
		},
		"hash is calculated for input wat": {
			hashFn:   identityHash,
			input:    ioutil.NopCloser(bytes.NewReader([]byte("wat"))),
			expected: "wat-identity",
		},
		"metafile name is calculated for metafile input": {
			hashFn:   identityHash,
			input:    ioutil.NopCloser(bytes.NewReader([]byte(`{"memorybox":{"file":"test","source":"testing"},"data":{}}`))),
			expected: archive.MetaFilePrefix + "test",
		},
	}
	for name, test := range table {
		test := test
		t.Run(name, func(t *testing.T) {
			f, err := archive.NewFromReader(test.hashFn, test.input)
			if err != nil {
				t.Fatalf("test setup: %s", err)
			}
			defer f.Close()
			if diff := cmp.Diff(test.expected, f.Name()); diff != "" {
				t.Fatalf(diff)
			}
		})
	}
}

func TestMetaFileName(t *testing.T) {
	filename := "test"
	expected := archive.MetaFilePrefix + filename
	actual := archive.MetaFileName(filename)
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Fatal(diff)
	}
}

func TestFile_Read(t *testing.T) {
	type testCase struct {
		file          *archive.File
		expectedBytes []byte
		expectedErr   error
	}
	table := map[string]testCase{
		"meta reader with valid json": func() testCase {
			f, err := archive.NewFromReader(identityHash, ioutil.NopCloser(bytes.NewReader([]byte("test"))))
			if err != nil {
				t.Fatalf("test setup: %s", err)
			}
			defer f.Close()
			f.MetaSet("test", "value")
			metaFile := archive.NewMetaFile(f)
			// get content, which will exhaust reader
			bytes, _ := ioutil.ReadAll(metaFile)
			// make new one from original
			metaFile = archive.NewMetaFile(f)
			return testCase{
				file:          metaFile,
				expectedBytes: bytes,
				expectedErr:   nil,
			}
		}(),
		"backing file disappeared": {
			file: func() *archive.File {
				tempFile, tempErr := ioutil.TempFile(os.TempDir(), "*")
				if tempErr != nil {
					t.Fatalf("test setup: %s", tempErr)
				}
				_, writeErr := tempFile.Write([]byte("test"))
				if writeErr != nil {
					t.Fatalf("test setup: %s", writeErr)
				}
				f, loadErr := archive.New(identityHash, tempFile.Name())
				if loadErr != nil {
					t.Fatalf("test setup: %s", loadErr)
				}
				tempFile.Close()
				removeErr := os.Remove(tempFile.Name())
				if removeErr != nil {
					t.Fatalf("test setup: %s", removeErr)
				}
				return f
			}(),
			expectedBytes: []byte{},
			expectedErr:   os.ErrNotExist,
		},
	}
	for name, test := range table {
		test := test
		t.Run(name, func(t *testing.T) {
			defer test.file.Close()
			expectedSize := len(test.expectedBytes)
			actualContent := make([]byte, expectedSize)
			size, err := test.file.Read(actualContent)
			if err != nil && test.expectedErr == nil {
				t.Fatal(err)
			}
			if err != nil && test.expectedErr != nil && !errors.Is(err, test.expectedErr) && !strings.Contains(err.Error(), test.expectedErr.Error()) {
				t.Fatalf("expected error: %s, got %s", test.expectedErr, err)
			}
			if !bytes.Equal(test.expectedBytes, actualContent) {
				t.Fatalf("expected %s, got %s", test.expectedBytes, actualContent)
			}
			if diff := cmp.Diff(expectedSize, size); diff != "" {
				t.Fatalf(diff)
			}
		})
	}
}

func TestFile_MetaSetGetDelete(t *testing.T) {
	type testCase struct {
		file         *archive.File
		key          string
		input        string
		expected     interface{}
		immutableKey bool
	}
	table := map[string]testCase{
		// the value of the metakey is the content hash of the file it describes
		"metakey is immutable for consumers": func() testCase {
			f, err := archive.NewFromReader(identityHash, ioutil.NopCloser(bytes.NewReader([]byte("test"))))
			if err != nil {
				t.Fatalf("test setup: %s", err)
			}
			f.Close()
			metaFile := archive.NewMetaFile(f)
			meta := metaFile.MetaGet(archive.MetaKey)
			return testCase{
				file:         metaFile,
				key:          archive.MetaKey,
				input:        "anything",
				expected:     meta,
				immutableKey: true,
			}
		}(),
		"string values can be set and retrieved": {
			key:      "test",
			input:    "value",
			expected: "value",
		},
		"string values containing integers are cast as such": {
			key:      "test",
			input:    "100",
			expected: json.RawMessage("100"),
		},
		"string values containing floating point numbers are cast as such": {
			key:      "test",
			input:    "100.1",
			expected: json.RawMessage("100.1"),
		},
		"string value true is cast as json boolean": {
			key:      "test",
			input:    "true",
			expected: json.RawMessage("true"),
		},
		"string value false is cast as json boolean": {
			key:      "test",
			input:    "false",
			expected: json.RawMessage("false"),
		},
		"valid json encoded strings are stored as json.RawMessages": {
			key:      "test",
			input:    `{"test":"value"}`,
			expected: json.RawMessage(`{"test":"value"}`),
		},
		"invalid json encoded strings are stored as plain strings": {
			key:      "test",
			input:    `{"test":"value}`,
			expected: `{"test":"value}`,
		},
	}
	for name, test := range table {
		test := test
		t.Run(name, func(t *testing.T) {
			f, err := archive.NewFromReader(identityHash, ioutil.NopCloser(bytes.NewReader([]byte("test"))))
			if err != nil {
				t.Fatalf("test setup: %s", err)
			}
			defer f.Close()
			if test.file != nil {
				//f.Close()
				f = test.file
			}
			f.MetaSet(test.key, test.input)
			actualAfterSet := f.MetaGet(test.key)
			if diff := cmp.Diff(test.expected, actualAfterSet); diff != "" {
				t.Fatalf(diff)
			}
			f.MetaDelete(test.key)
			actualAfterDelete := f.MetaGet(test.key)
			if test.immutableKey && actualAfterDelete != nil {
				t.Fatal("expected key to be immutable")
			}
			if !test.immutableKey && f.MetaGet(test.key) != nil {
				t.Fatal("expected delete to remove key")
			}
		})
	}
}
