package file_test

import (
	"bytes"
	"errors"
	"github.com/google/go-cmp/cmp"
	"github.com/mattetti/filebuffer"
	"github.com/tkellen/memorybox/pkg/file"
	"io"
	"os"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	type testCase struct {
		body         *filebuffer.Buffer
		expectedName string
		expectedErr  error
		hashFn       file.HashFn
	}
	content := []byte("test")
	mock := filebuffer.New(content)
	expectedName, _, _ := file.Sha256(mock)
	table := map[string]testCase{
		"name is set by hashing input content": {
			body:         filebuffer.New(content),
			expectedName: expectedName,
			expectedErr:  nil,
			hashFn:       file.Sha256,
		},
		"hashing failures are caught": func() testCase {
			return testCase{
				body: func() *filebuffer.Buffer {
					f := filebuffer.New(content)
					f.Close()
					return f
				}(),
				expectedName: "",
				expectedErr:  os.ErrClosed,
				hashFn:       file.Sha256,
			}
		}(),
		"failure to validate metadata is caught": func() testCase {
			return testCase{
				body: func() *filebuffer.Buffer {
					f := filebuffer.New([]byte(file.NewMetaFromFile(file.NewStub("test", 0, time.Now())).String()))
					f.Close()
					return f
				}(),
				expectedName: "",
				expectedErr:  os.ErrClosed,
				hashFn: func(_ io.Reader) (string, int64, error) {
					return "test", 0, nil
				},
			}
		}(),
		"metadata content is not allowed": {
			body:         filebuffer.New([]byte(file.NewMetaFromFile(file.NewStub("test", 0, time.Now())).String())),
			expectedName: "",
			expectedErr:  os.ErrInvalid,
			hashFn:       file.Sha256,
		},
	}
	for name, test := range table {
		test := test
		t.Run(name, func(t *testing.T) {
			f, err := file.New("test", test.body, time.Now(), test.hashFn)
			if err != nil && test.expectedErr == nil {
				t.Fatal(err)
			}
			if err != nil && test.expectedErr != nil && !errors.Is(err, test.expectedErr) {
				t.Fatalf("expected error: %s, got %s", test.expectedErr, err)
			}
			if err == nil {
				if diff := cmp.Diff(test.expectedName, f.Name); diff != "" {
					t.Fatalf(diff)
				}
			}
		})
	}
}

func TestFile_Read(t *testing.T) {
	type testCase struct {
		file          *file.File
		expectedBytes []byte
		expectedErr   error
	}
	table := map[string]testCase{
		"stub": {
			file:        file.NewStub("test", 0, time.Now()),
			expectedErr: io.ErrUnexpectedEOF,
		},
		"dataFile": func() testCase {
			bytes := []byte("test")
			file, err := file.NewSha256("test", filebuffer.New(bytes), time.Now())
			if err != nil {
				t.Fatalf("test setup: %s", err)
			}
			return testCase{
				file:          file,
				expectedBytes: bytes,
				expectedErr:   nil,
			}
		}(),
	}
	for name, test := range table {
		test := test
		t.Run(name, func(t *testing.T) {
			expectedSize := len(test.expectedBytes)
			actualContent := make([]byte, expectedSize)
			size, err := test.file.Read(actualContent)
			if err != test.expectedErr {
				t.Fatalf("expected error %s, got %s", test.expectedErr, err)
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

func TestFile_Close(t *testing.T) {
	stub := file.NewStub("test", 0, time.Now())
	if stub.Close() != nil {
		t.Fatal("expected closing file without backing io to cause no error")
	}
	file, err := file.NewSha256("test", filebuffer.New([]byte("test")), time.Now())
	if err != nil {
		t.Fatalf("test setup: %s", err)
	}
	if file.Close() != nil {
		t.Fatal("expected file to close without error")
	}
	if _, err := file.Read([]byte{}); err != os.ErrClosed {
		t.Fatalf("expected error %s, got %s", os.ErrClosed, err)
	}
}

func TestDataNameFrom(t *testing.T) {
	table := map[string]struct {
		input    string
		expected string
	}{
		"metaFile names become dataFile names": {
			input:    file.MetaFilePrefix + "test",
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
			actual := file.DataNameFrom(test.input)
			if diff := cmp.Diff(test.expected, actual); diff != "" {
				t.Fatal(diff)
			}
		})
	}
}
