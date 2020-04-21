// I would prefer these be unit tests but there appears to be no coherent way in
// golang to mock *os.File, nor any clear way to remove the responsibility of
// performing real network/disk io in the implementation of this library. As a
// result, this test suite is mostly integration tests of the golang standard
// library for network/disk io (except in cases where failures needed to be
// simulated). More details here:
// https://github.com/golang/go/issues/14106
// https://github.com/golang/go/issues/21592
package file_test

import (
	"bytes"
	"errors"
	"github.com/google/go-cmp/cmp"
	. "github.com/tkellen/memorybox/pkg/file"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"testing"
)

func TestFile_LoadSetSourceValue(t *testing.T) {
	table := map[string]struct {
		input    interface{}
		expected string
	}{
		"string input becomes exact source value": {
			input:    "http://google.com",
			expected: "http://google.com",
		},
		"non-string input becomes its type": {
			input:    ioutil.NopCloser(bytes.NewReader([]byte("test"))),
			expected: "ioutil.nopCloser",
		},
	}
	for name, test := range table {
		t.Run(name, func(t *testing.T) {
			f := New()
			_, _ = f.Load(test.input)
			defer f.Close()
			actual := f.Source()
			if diff := cmp.Diff(test.expected, actual); diff != "" {
				t.Fatal(diff)
			}
		})
	}
}

func TestFile_LoadDetectMetafile(t *testing.T) {
	table := map[string]struct {
		input    io.ReadCloser
		expected bool
	}{
		"json formatted input with memorybox key is metafile": {
			input:    ioutil.NopCloser(bytes.NewReader([]byte("{\"memorybox\":\"\"}"))),
			expected: true,
		},
		"json formatted input without memorybox key is not metafile": {
			input:    ioutil.NopCloser(bytes.NewReader([]byte("{}"))),
			expected: false,
		},
	}
	for name, test := range table {
		t.Run(name, func(t *testing.T) {
			f := New()
			_, _ = f.Load(test.input)
			defer f.Close()
			if test.expected != f.IsMetaFile() {
				t.Fatalf("expected IsMetaFile to be %v", test.expected)
			}
		})
	}
}

func TestFile_LoadSetName(t *testing.T) {
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
		t.Run(name, func(t *testing.T) {
			f := New()
			_, _ = f.Load(test.input)
			defer f.Close()
			if diff := cmp.Diff(test.expected, f.Name()); diff != "" {
				t.Fatalf(diff)
			}
		})
	}
}

func TestFile_Load(t *testing.T) {
	expectedBytes := []byte("test")
	tempFile, tempFileErr := ioutil.TempFile(os.TempDir(), "*")
	if tempFileErr != nil {
		t.Fatalf("test setup: %s", tempFileErr)
	}
	defer os.Remove(tempFile.Name())
	table := map[string]struct {
		input       interface{}
		file        *File
		expectedErr error
	}{
		"success from stdin": {
			input: "-",
			file: func() *File {
				sys := NewSystem()
				sys.Stdin = ioutil.NopCloser(bytes.NewReader(expectedBytes))
				f := New()
				f.TestSetSystem(sys)
				return f
			}(),
			expectedErr: nil,
		},
		"success from url": {
			input: "http://totally.legit",
			file: func() *File {
				sys := NewSystem()
				// Mock every http request to contain our expected bytes.
				sys.Get = func(url string) (resp *http.Response, err error) {
					return &http.Response{
						Status:        "OK",
						StatusCode:    200,
						Proto:         "HTTP/1.1",
						ProtoMajor:    1,
						ProtoMinor:    1,
						Body:          ioutil.NopCloser(bytes.NewReader(expectedBytes)),
						ContentLength: int64(len(expectedBytes)),
					}, nil
				}
				f := New()
				f.TestSetSystem(sys)
				return f
			}(),
			expectedErr: nil,
		},
		"success from local file": {
			input: tempFile.Name(),
			file: func() *File {
				_, writeErr := tempFile.Write(expectedBytes)
				if writeErr != nil {
					t.Fatalf("test setup: %s", writeErr)
				}
				tempFile.Close()
				return New()
			}(),
			expectedErr: nil,
		},
		"success from io.ReadCloser": {
			input:       ioutil.NopCloser(bytes.NewBuffer(expectedBytes)),
			file:        New(),
			expectedErr: nil,
		},
		"fail on inability to make http request for input url": {
			input:       "http://that.is.not.a.valid.url",
			file:        New(),
			expectedErr: errors.New("no such host"),
		},
		"fail on non-200 http response from url input": {
			input: "http://totally.legit",
			file: func() *File {
				sys := NewSystem()
				// Mock every http request to fail with a 400 error code.
				sys.Get = func(url string) (resp *http.Response, err error) {
					return &http.Response{
						Status:        "BadRequest",
						StatusCode:    400,
						Proto:         "HTTP/1.1",
						ProtoMajor:    1,
						ProtoMinor:    1,
						Body:          ioutil.NopCloser(bytes.NewReader(expectedBytes)),
						ContentLength: int64(len(expectedBytes)),
					}, nil
				}
				f := New()
				f.TestSetSystem(sys)
				return f
			}(),
			expectedErr: errors.New("http code: 400"),
		},
		"fail on invalid file": {
			input:       "/nope/nope/nope/nope/nope",
			file:        New(),
			expectedErr: errors.New("no such file"),
		},
		"fail on unsupported input source": {
			input:       func() {},
			file:        New(),
			expectedErr: errors.New("unsupported source"),
		},
		"fail on inability to create temporary directory": {
			input: "-",
			file: func() *File {
				sys := NewSystem()
				sys.TempDirBase = "/tmp/nope/bad"
				f := New()
				f.TestSetSystem(sys)
				return f
			}(),
			expectedErr: errors.New("/tmp/nope/bad"),
		},
		"fail on inability to buffer streaming input to disk": {
			input: "-",
			file: func() *File {
				sys := NewSystem()
				sys.TempDir = "/cannot/buffer/here"
				f := New()
				f.TestSetSystem(sys)
				return f
			}(),
			expectedErr: errors.New("/cannot/buffer/here"),
		},
		"fail on inability to compute content hash": {
			input: ioutil.NopCloser(bytes.NewBuffer(expectedBytes)),
			file: func() *File {
				count := 0
				sys := NewSystem()
				// When input arrives as a stream, it is tee'd to a temporary
				// file that is populated during hashing. By closing this file
				// early hashing is forced to fail.
				sys.TempFile = func(dir string, pattern string) (*os.File, error) {
					file, err := ioutil.TempFile(dir, pattern)
					if err != nil {
						return nil, err
					}
					if count == 0 {
						file.Close()
					}
					count = count + 1
					return file, nil
				}
				f := New()
				f.TestSetSystem(sys)
				return f
			}(),
			expectedErr: errors.New("hashing"),
		},
		"fail on inability to open file to check if it is a metafile": {
			input: ioutil.NopCloser(bytes.NewBuffer(expectedBytes)),
			file: func() *File {
				sys := NewSystem()
				sys.Open = func(name string) (*os.File, error) {
					return nil, errors.New("bad time")
				}
				f := New()
				f.TestSetSystem(sys)
				return f
			}(),
			expectedErr: errors.New("metacheck: bad time"),
		},
	}
	for name, test := range table {
		test := test
		t.Run(name, func(t *testing.T) {
			_, err := test.file.Load(test.input)
			defer test.file.Close()
			if err != nil && test.expectedErr == nil {
				t.Fatal(err)
			}
			log.Printf("%s: %s", name, err)
			if err != nil && test.expectedErr != nil && !strings.Contains(err.Error(), test.expectedErr.Error()) {
				t.Fatalf("expected error: %s, got %s", test.expectedErr, err)
			}
			if err == nil {
				actualBytes, readErr := ioutil.ReadAll(test.file)
				if readErr != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(expectedBytes, actualBytes) {
					t.Fatalf("expected bytes %s, got %s", expectedBytes, actualBytes)
				}
			}
		})
	}
}

func TestNew(t *testing.T) {
	f := New()
	f.SetMeta("test", "value")
	value := f.GetMeta("test").(string)
	if value != "value" {
		t.Fatal("expected file initialized enough to control metadata")
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

func TestFile_NewMetaFile(t *testing.T) {
	alreadyMeta := New().NewMetaFile()
	convertedAlreadyMeta := alreadyMeta.NewMetaFile()
	if alreadyMeta != convertedAlreadyMeta {
		t.Fatal("expected converting a metafile to a metafile to be a noop")
	}
	dataFile := New()
	convertedDataFile := dataFile.NewMetaFile()
	expectedConvertedName := MetaFileName(dataFile.Name())
	if !convertedDataFile.IsMetaFile() {
		t.Fatal("expected datafile to become metafile")
	}
	if expectedConvertedName != convertedDataFile.Name() {
		t.Fatalf("expected name to be %s, got %s", expectedConvertedName, convertedDataFile.Name())
	}
}

func TestFile_Read(t *testing.T) {
	testContent := []byte("test")
	testSize := len(testContent)
	table := map[string]struct {
		file            *File
		expectedContent []byte
		expectedSize    int
		expectedErr     error
	}{
		"existing reader": {
			file: func() *File {
				f := New()
				f.TestSetReader(ioutil.NopCloser(bytes.NewBuffer(testContent)))
				return f
			}(),
			expectedContent: testContent,
			expectedSize:    testSize,
			expectedErr:     nil,
		},
		"meta reader with valid json": {
			file: func() *File {
				f := New()
				f.SetMeta("test", "value")
				return f.NewMetaFile()
			}(),
			expectedContent: []byte("{\"memorybox\":\"\",\"test\":\"value\"}"),
			expectedSize:    31,
			expectedErr:     nil,
		},
		"meta reader with invalid json": {
			file: func() *File {
				f := New()
				f.SetMeta("test", func() {})
				return f.NewMetaFile()
			}(),
			expectedContent: []byte{},
			expectedSize:    0,
			expectedErr:     errors.New("unsupported type"),
		},
		"backing file disappeared": {
			file: func() *File {
				f := New()
				tempFile, tempErr := ioutil.TempFile(os.TempDir(), "*")
				if tempErr != nil {
					t.Fatalf("test setup: %s", tempErr)
				}
				_, writeErr := tempFile.Write(testContent)
				if writeErr != nil {
					t.Fatalf("test setup: %s", writeErr)
				}
				_, loadErr := f.Load(tempFile.Name())
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
			expectedContent: []byte{},
			expectedSize:    0,
			expectedErr:     errors.New("no such file"),
		},
	}
	for name, test := range table {
		test := test
		t.Run(name, func(t *testing.T) {
			defer test.file.Close()
			actualContent := make([]byte, test.expectedSize)
			size, err := test.file.Read(actualContent)
			if err != nil && test.expectedErr == nil {
				t.Fatal(err)
			}
			if err != nil && test.expectedErr != nil && !strings.Contains(err.Error(), test.expectedErr.Error()) {
				t.Fatalf("expected error: %s, got %s", test.expectedErr, err)
			}
			if diff := cmp.Diff(test.expectedContent, actualContent); diff != "" {
				t.Fatal(diff)
			}
			if diff := cmp.Diff(test.expectedSize, size); diff != "" {
				t.Fatalf(diff)
			}
		})
	}
}
