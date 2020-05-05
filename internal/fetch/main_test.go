package fetch_test

import (
	"bytes"
	"context"
	"errors"
	"github.com/tkellen/memorybox/internal/fetch"
	"io/ioutil"
	"os"
	"testing"
)

func TestFetch(t *testing.T) {
	tempDir, tempDirErr := ioutil.TempDir("", "*")
	if tempDirErr != nil {
		t.Fatalf("test setup: %s", tempDirErr)
	}
	tempFileBytes := []byte("test")
	tempFile, tempFileErr := ioutil.TempFile(tempDir, "*")
	if tempFileErr != nil {
		t.Fatalf("test setup: %s", tempFileErr)
	}
	_, writeErr := tempFile.Write(tempFileBytes)
	if writeErr != nil {
		t.Fatalf("test setup: %s", writeErr)
	}
	tempFile.Close()
	defer os.RemoveAll(tempDir)
	table := map[string]struct {
		input         string
		expectedErr   error
		expectedBytes []byte
	}{
		"success from local file": {
			input:         tempFile.Name(),
			expectedBytes: tempFileBytes,
		},
		"fail on invalid file": {
			input:       "/nope/nope/nope/nope/nope",
			expectedErr: os.ErrNotExist,
		},
	}
	for name, test := range table {
		test := test
		t.Run(name, func(t *testing.T) {
			file, deleteWhenDone, err := fetch.One(context.Background(), test.input)
			if deleteWhenDone {
				defer os.Remove(file.Name())
			}
			if err != nil && test.expectedErr == nil {
				t.Fatal(err)
			}
			if err != nil && test.expectedErr != nil && !errors.Is(err, test.expectedErr) {
				t.Fatalf("expected error: %s, got %s", test.expectedErr, err)
			}
			if err == nil {
				actualBytes, readErr := ioutil.ReadAll(file)
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
