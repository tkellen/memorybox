package file

import (
	"bytes"
	"errors"
	"github.com/google/go-cmp/cmp"
	"io/ioutil"
	"strings"
	"testing"
)

/*
func TestNew(t *testing.T) {
	expectedContent := []byte("test")
	expectedHash := "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08-sha256"
	table := map[string]struct{
		input io.ReadCloser
		expected *File
		expectedErr error
	}{
		"file is named by hash of content": {
			input: ioutil.NopCloser(bytes.NewBuffer(expectedContent)),
			expected: &File{
				meta: map[string]interface{}{},
				name: expectedHash,
				source: "{test}",
			},
			expectedErr: nil,
		},
	}
	for name, test := range table {
		t.Run(name, func(t *testing.T) {
			actual, err := New(test.input)
			actual.Close()
			actual.backingFilePath = ""
			test.expected.io = actual.io
			if err != nil && test.expectedErr == nil {
				t.Fatalf("unexpected error: %s", err)
			}
			if !reflect.DeepEqual(test.expected, actual) {
				t.Fatalf("expected %#v, got %#v", test.expected, actual)
			}
		})
	}
}*/

func TestMetaFileName(t *testing.T) {
	filename := "test"
	expected := MetaFilePrefix + filename
	actual := MetaFileName(filename)
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Fatal(diff)
	}
}

func TestFile_Name(t *testing.T) {
	filename := "test"
	expected := filename
	actual := (&File{name: filename}).Name()
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Fatal(diff)
	}

	expected = MetaFilePrefix + filename
	actual = (&File{name: filename, isMetaFile: true}).Name()
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Fatal(diff)
	}
}

func TestFile_Source(t *testing.T) {
	source := "test"
	expected := source
	actual := (&File{source: source}).Source()
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Fatal(diff)
	}
}

func TestFile_NewMetaFile(t *testing.T) {
	alreadyMeta := &File{isMetaFile: true}
	convertedAlreadyMeta := alreadyMeta.NewMetaFile()
	if alreadyMeta != convertedAlreadyMeta {
		t.Fatal("expected converting a metafile to a metafile to be a noop")
	}
	dataFile := &File{name: "test"}
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
			file: &File{
				reader: ioutil.NopCloser(bytes.NewBuffer(testContent)),
			},
			expectedContent: testContent,
			expectedSize:    testSize,
			expectedErr:     nil,
		},
		"meta reader": {
			file: &File{
				isMetaFile: true,
				meta: map[string]interface{}{
					"test": "value",
				},
			},
			expectedContent: []byte("{\"test\":\"value\"}"),
			expectedSize:    16,
			expectedErr:     nil,
		},
		"meta reader bad content": {
			file: &File{
				isMetaFile: true,
				meta: map[string]interface{}{
					"test": func() {},
				},
			},
			expectedContent: []byte{},
			expectedSize:    0,
			expectedErr:     errors.New("unsupported type"),
		},
		"good backing file reader": {
			file: func() *File {
				sys, err := newSystem()
				if err != nil {
					t.Fatalf("test setup: %s", err)
				}
				tempFile, tempErr := ioutil.TempFile(sys.TempDir, "*")
				if tempErr != nil {
					t.Fatalf("test setup: %s", tempErr)
				}
				_, writeErr := tempFile.Write(testContent)
				if writeErr != nil {
					t.Fatalf("test setup: %s", writeErr)
				}
				tempFile.Close()
				return &File{
					io:              sys,
					backingFilePath: tempFile.Name(),
				}
			}(),
			expectedContent: testContent,
			expectedSize:    testSize,
			expectedErr:     nil,
		},
		"bad backing file reader": {
			file: func() *File {
				sys, err := newSystem()
				if err != nil {
					t.Fatalf("test setup: %s", err)
				}
				return &File{
					io:              sys,
					backingFilePath: "/nope/nope/nope",
				}
			}(),
			expectedContent: []byte{},
			expectedSize:    0,
			expectedErr:     errors.New("/nope/nope/nope"),
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
