package file_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/google/go-cmp/cmp"
	. "github.com/tkellen/memorybox/pkg/file"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"testing"
)

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
		input                  interface{}
		expectedIsMetaDataFile bool
		expectedSource         string
		expectedErr            error
		expectedBytes          []byte
	}{
		"success from local file": {
			input:          tempFile.Name(),
			expectedSource: tempFile.Name(),
			expectedBytes:  tempFileBytes,
		},
		"success from io.ReadCloser": {
			input:          ioutil.NopCloser(bytes.NewBuffer([]byte("test"))),
			expectedSource: "ioutil.nopCloser",
			expectedBytes:  []byte("test"),
		},
		"fail on inability to make http request for input url": {
			input:          "http://that.is.not.a.valid.url",
			expectedSource: "http://that.is.not.a.valid.url",
			expectedErr:    errors.New("no such host"),
		},
		"fail on invalid file": {
			input:       "/nope/nope/nope/nope/nope",
			expectedErr: os.ErrNotExist,
		},
		"json formatted input with memorybox key is metafile": {
			input:                  ioutil.NopCloser(bytes.NewReader([]byte("{\"memorybox\":\"test\"}"))),
			expectedIsMetaDataFile: true,
			expectedSource:         "memorybox-meta-test",
			expectedBytes:          []byte("{\"memorybox\":\"test\"}"),
		},
		"json formatted input without memorybox key is not metafile": {
			input:          ioutil.NopCloser(bytes.NewReader([]byte("{}"))),
			expectedSource: "ioutil.nopCloser",
			expectedBytes:  []byte("{}"),
		},
	}
	for name, test := range table {
		test := test
		t.Run(name, func(t *testing.T) {
			var f *File
			var err error
			if inputIsString, ok := test.input.(string); ok {
				f, err = New(inputIsString)
			} else {
				f, err = NewFromReader(test.input.(io.ReadCloser))
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

func TestNewSetsName(t *testing.T) {
	table := map[string]struct {
		input    io.ReadCloser
		expected string
	}{
		"sha256 digest name is calculated for input test": {
			input:    ioutil.NopCloser(bytes.NewReader([]byte("test"))),
			expected: "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08-sha256",
		},
		"sha256 digest name is calculated for input wat": {
			input:    ioutil.NopCloser(bytes.NewReader([]byte("wat"))),
			expected: "f00a787f7492a95e165b470702f4fe9373583fbdc025b2c8bdf0262cc48fcff4-sha256",
		},
		"metafile name is calculated for metafile input": {
			input:    ioutil.NopCloser(bytes.NewReader([]byte("{\"memorybox\":\"test\"}"))),
			expected: MetaFilePrefix + "test",
		},
	}
	for name, test := range table {
		test := test
		t.Run(name, func(t *testing.T) {
			f, err := NewFromReader(test.input)
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
	expected := MetaFilePrefix + filename
	actual := MetaFileName(filename)
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Fatal(diff)
	}
}

func TestFile_MetaSetGetDelete(t *testing.T) {
	table := map[string]struct {
		file         *File
		key          string
		input        string
		expected     interface{}
		immutableKey bool
	}{
		// the value of the metakey is the content hash of the file it describes
		"metakey is immutable for consumers": {
			file: func() *File {
				f, err := NewFromReader(ioutil.NopCloser(bytes.NewReader([]byte("test"))))
				if err != nil {
					t.Fatalf("test setup: %s", err)
				}
				f.Close()
				return NewMetaFile(f)
			}(),
			key:          MetaKey,
			input:        "anything",
			expected:     "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08-sha256",
			immutableKey: true,
		},
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
			input:    "{\"test\":\"value\"}",
			expected: json.RawMessage("{\"test\":\"value\"}"),
		},
		"invalid json encoded strings are stored as plain strings": {
			key:      "test",
			input:    "{\"test\":\"value}",
			expected: "{\"test\":\"value}",
		},
	}
	for name, test := range table {
		test := test
		t.Run(name, func(t *testing.T) {
			f, err := NewFromReader(ioutil.NopCloser(bytes.NewReader([]byte("test"))))
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
			if test.immutableKey && actualAfterDelete != actualAfterSet {
				t.Fatal("expected key to be immutable")
			}
			if !test.immutableKey && f.MetaGet(test.key) != nil {
				t.Fatal("expected delete to remove key")
			}
		})
	}
}

func TestFile_Read(t *testing.T) {
	table := map[string]struct {
		file          *File
		expectedBytes []byte
		expectedErr   error
	}{
		"meta reader with valid json": {
			file: func() *File {
				f, err := NewFromReader(ioutil.NopCloser(bytes.NewReader([]byte("test"))))
				if err != nil {
					t.Fatalf("test setup: %s", err)
				}
				defer f.Close()
				f.MetaSet("test", "value")
				return NewMetaFile(f)
			}(),
			expectedBytes: []byte("{\"memorybox\":\"9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08-sha256\",\"test\":\"value\"}"),
			expectedErr:   nil,
		},
		"backing file disappeared": {
			file: func() *File {
				tempFile, tempErr := ioutil.TempFile(os.TempDir(), "*")
				if tempErr != nil {
					t.Fatalf("test setup: %s", tempErr)
				}
				_, writeErr := tempFile.Write([]byte("test"))
				if writeErr != nil {
					t.Fatalf("test setup: %s", writeErr)
				}
				f, loadErr := New(tempFile.Name())
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
			if diff := cmp.Diff(test.expectedBytes, actualContent); diff != "" {
				t.Fatal(diff)
			}
			if diff := cmp.Diff(expectedSize, size); diff != "" {
				t.Fatalf(diff)
			}
		})
	}
}
