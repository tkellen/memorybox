package archive_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/go-cmp/cmp"
	"github.com/mattetti/filebuffer"
	"github.com/tkellen/memorybox/pkg/archive"
	"io"
	"io/ioutil"
	"os"
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

func TestSha256(t *testing.T) {
	input := []byte("test")
	expected := "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08-sha256"
	expectedSize := int64(len(input))
	actual, actualSize, goodErr := archive.Sha256(bytes.NewReader(input))
	if goodErr != nil {
		t.Fatal(goodErr)
	}
	if expected != actual {
		t.Fatalf("expected %s, got %s", expected, actual)
	}
	if expectedSize != actualSize {
		t.Fatalf("expected size %d, got %d", expectedSize, actualSize)
	}
	file := filebuffer.New([]byte("test"))
	file.Close()
	_, _, err := archive.Sha256(file)
	if err == nil {
		t.Fatal("expected error on bad reader")
	}
}

func TestMetaFileNameFrom(t *testing.T) {
	table := map[string]struct {
		input    string
		expected string
	}{
		"filenames become metafile names": {
			input:    "test",
			expected: archive.MetaFilePrefix + "test",
		},
		"metafile names remain the same": {
			input:    archive.MetaFilePrefix + "test",
			expected: archive.MetaFilePrefix + "test",
		},
	}
	for name, test := range table {
		test := test
		t.Run(name, func(t *testing.T) {
			actual := archive.MetaFileNameFrom(test.input)
			if diff := cmp.Diff(test.expected, actual); diff != "" {
				t.Fatal(diff)
			}
		})
	}
}

func TestDataFileNameFrom(t *testing.T) {
	table := map[string]struct {
		input    string
		expected string
	}{
		"metafile names become datafile names": {
			input:    archive.MetaFilePrefix + "test",
			expected: "test",
		},
		"regular filenames are unchanged": {
			input:    "test",
			expected: "test",
		},
	}
	for name, test := range table {
		test := test
		t.Run(name, func(t *testing.T) {
			actual := archive.DataFileNameFrom(test.input)
			if diff := cmp.Diff(test.expected, actual); diff != "" {
				t.Fatal(diff)
			}
		})
	}
}

func TestIsMetaFileName(t *testing.T) {
	table := map[string]struct {
		input    string
		expected bool
	}{
		"metafile names return true": {
			input:    archive.MetaFilePrefix + "test",
			expected: true,
		},
		"other things return false": {
			input:    "test",
			expected: false,
		},
	}
	for name, test := range table {
		test := test
		t.Run(name, func(t *testing.T) {
			actual := archive.IsMetaFileName(test.input)
			if diff := cmp.Diff(test.expected, actual); diff != "" {
				t.Fatal(diff)
			}
		})
	}
}

func TestIsMetaData(t *testing.T) {
	table := map[string]struct {
		input    []byte
		expected bool
	}{
		"not being valid json means it is not metadata": {
			input:    []byte(`[:not-metadata:]`),
			expected: false,
		},
		"being valid json is not enough to be considered metadata": {
			input:    []byte(`{}`),
			expected: false,
		},
		"valid json with a metadata key is metadata": {
			input:    []byte(`{"memorybox":{}}`),
			expected: true,
		},
	}
	for name, test := range table {
		test := test
		t.Run(name, func(t *testing.T) {
			actual := archive.IsMetaData(test.input)
			if test.expected != actual {
				t.Fatalf("expected %v, got %v", test.expected, actual)
			}
		})
	}
}

func TestNew(t *testing.T) {
	largeJsonContent := []byte(fmt.Sprintf(`{"memorybox":{"name":"test"},"data":"%s"}`, make([]byte, archive.MetaFileMaxSize, archive.MetaFileMaxSize)))
	table := map[string]struct {
		hashFn       archive.Hasher
		data         io.ReadSeeker
		expectedName string
		isMetaFile   bool
		expectedErr  error
	}{
		"datafile is detected and name is set by hashing input content": {
			hashFn:       identityHash,
			data:         filebuffer.New([]byte("test")),
			expectedName: "test-identity",
			isMetaFile:   false,
			expectedErr:  nil,
		},
		fmt.Sprintf("metafile is detected and name is set by reading %s if content is memorybox json", archive.MetaFileNameKey): {
			hashFn:       identityHash,
			data:         filebuffer.New([]byte(`{"memorybox":{"file":"wacky"}}`)),
			expectedName: "wacky",
			isMetaFile:   true,
			expectedErr:  nil,
		},
		"files larger than archive.MetaFileMaxSize are not detected as metadata": {
			hashFn: archive.Sha256,
			data:   filebuffer.New(largeJsonContent),
			expectedName: func() string {
				name, _, _ := archive.Sha256(bytes.NewReader(largeJsonContent))
				return name
			}(),
			isMetaFile:  false,
			expectedErr: nil,
		},
		"hashing failures are caught": {
			hashFn: identityHash,
			data: func() *filebuffer.Buffer {
				file := filebuffer.New([]byte("test"))
				file.Close()
				return file
			}(),
			expectedName: "",
			isMetaFile:   false,
			expectedErr:  os.ErrClosed,
		},
		"failing to read file during meta check is caught": {
			// make the hash operation do nothing so it doesn't fail. this
			// allows the failure to occur in the meta check step
			hashFn: func(reader io.Reader) (string, int64, error) {
				return "test", 0, nil
			},
			data: func() *filebuffer.Buffer {
				file := filebuffer.New([]byte("test"))
				file.Close()
				return file
			}(),
			expectedName: "",
			isMetaFile:   false,
			expectedErr:  os.ErrClosed,
		},
	}
	for name, test := range table {
		test := test
		t.Run(name, func(t *testing.T) {
			f, err := archive.New("test", test.data, test.hashFn)
			if err != nil && test.expectedErr == nil {
				t.Fatal(err)
			}
			if err != nil && test.expectedErr != nil && !errors.Is(err, test.expectedErr) {
				t.Fatalf("expected error: %s, got %s", test.expectedErr, err)
			}
			if err == nil {
				if diff := cmp.Diff(test.expectedName, f.Name()); diff != "" {
					t.Fatalf(diff)
				}
				if test.isMetaFile != f.IsMetaFile() {
					t.Fatalf("expected isMetaFile to return %v, got %v", test.isMetaFile, f.IsMetaFile())
				}
			}
		})
	}
}

func TestNewSha256(t *testing.T) {
	data := []byte("test")
	expectedName, _, _ := archive.Sha256(filebuffer.New(data))
	file, err := archive.NewSha256("test", filebuffer.New(data))
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(expectedName, file.Name()); diff != "" {
		t.Fatalf(diff)
	}
}

func TestFile_Read(t *testing.T) {
	type testCase struct {
		file          *archive.File
		expectedBytes []byte
	}
	table := map[string]testCase{
		"datafile": func() testCase {
			bytes := []byte("test")
			file, err := archive.New("test", filebuffer.New(bytes), identityHash)
			if err != nil {
				t.Fatalf("test setup: %s", err)
			}
			return testCase{
				file:          file,
				expectedBytes: bytes,
			}
		}(),
		"metafile": func() testCase {
			bytes := []byte(`{"memorybox":{"file":"test"}}`)
			file, err := archive.New("test", filebuffer.New(bytes), identityHash)
			if err != nil {
				t.Fatalf("test setup: %s", err)
			}
			return testCase{
				file:          file,
				expectedBytes: bytes,
			}
		}(),
	}
	for name, test := range table {
		test := test
		t.Run(name, func(t *testing.T) {
			expectedSize := len(test.expectedBytes)
			actualContent := make([]byte, expectedSize)
			size, err := test.file.Read(actualContent)
			if err != nil {
				t.Fatal(err)
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
		key          string
		input        string
		expected     interface{}
		immutableKey bool
	}
	table := map[string]testCase{
		"metakey is immutable for consumers": {
			key:          archive.MetaFileNameKey,
			input:        "anything",
			expected:     "test-identity",
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
			f, err := archive.New("test", filebuffer.New([]byte(`{"memorybox":{"file":"test-identity"}}`)), identityHash)
			if err != nil {
				t.Fatalf("test setup: %s", err)
			}
			f.MetaSet(test.key, test.input)
			actualAfterSet := f.MetaGet(test.key)
			if diff := cmp.Diff(test.expected, actualAfterSet); diff != "" {
				t.Fatalf(diff)
			}
			f.MetaDelete(test.key)
			actualAfterDelete := f.MetaGet(test.key)
			if test.immutableKey && actualAfterDelete != test.expected {
				t.Fatal("expected key to be immutable")
			}
			if !test.immutableKey && f.MetaGet(test.key) != nil {
				t.Fatal("expected delete to remove key")
			}
		})
	}
}