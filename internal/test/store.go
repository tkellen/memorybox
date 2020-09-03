// Package test
package test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/tkellen/memorybox/pkg/archive"
	"github.com/tkellen/memorybox/pkg/file"
	"io/ioutil"
	"os"
	"reflect"
	"strings"
	"testing"
	"testing/iotest"
	"time"
)

func StoreSuite(t *testing.T, store archive.Store) {
	t.Run("put-stat-get-delete", func(t *testing.T) {
		storePutStatGetDelete(t, store)
	})
	t.Run("search-concat", func(t *testing.T) {
		storeSearchConcat(t, store)
	})
}

func storePutStatGetDelete(t *testing.T, store archive.Store) {
	ctx := context.Background()
	name := fmt.Sprint(time.Now().UnixNano())
	input := []byte("test")
	stamp := time.Now().Add(-(24 * time.Hour))
	// Test get failing on missing file.
	if f, err := store.Get(ctx, "test"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected file not to exist, got %#v", f)
	}
	// Test put failing when the supplied reader fails to be read.
	if err := store.Put(ctx, iotest.TimeoutReader(bytes.NewReader([]byte("test"))), "nope", time.Now()); err == nil {
		t.Fatalf("expected store to fail to put on invalid reader")
	}
	// Ensure failed put doesn't leave invalid file.
	if _, err := store.Get(ctx, "nope"); err == nil {
		t.Fatal("did not expect store to get file that failed to write")
	}
	// Test put succeeding.
	if err := store.Put(ctx, bytes.NewReader(input), name, stamp); err != nil {
		t.Fatalf("expected store to accept put, got %s", err)
	}
	// Test getting file that was just put.
	getFile, getErr := store.Get(ctx, name)
	if getErr != nil {
		t.Fatalf("expected store to get file by name, got %s", getErr)
	}
	output, err := ioutil.ReadAll(getFile)
	if err != nil {
		t.Fatal("unable to read data from returned file")
	}
	getFile.Close()
	if !bytes.Equal(input, output) {
		t.Fatalf("expected output to be %s, got %s", input, output)
	}
	// Test statting file that was just put.
	statFile, statErr := store.Stat(ctx, name)
	if statErr != nil {
		t.Fatalf("expected store to stat file by name, got %s", statErr)
	}
	// Test file that was "got" / "stat" has the right attributes.
	for _, f := range []*file.File{statFile, getFile} {
		if f.Name != name {
			t.Fatalf("expected name to be %s, got %s", f.Name, name)
		}
		if f.Size != int64(len(input)) {
			t.Fatalf("expected size to be %d, got %d", f.Size, len(input))
		}
		if f.LastModified.Sub(stamp) != 0 {
			t.Fatalf("expected lastModified to be %s, got %s", f.LastModified, stamp)
		}
	}
	// Test that a file can be removed.
	if err := store.Delete(ctx, name); err != nil {
		t.Fatalf("expected store to remove file by name, got %s", err)
	}
	// Ensure file was removed.
	if f, err := store.Get(ctx, "test"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected file not to exist, got %#v", f)
	}
}

func storeSearchConcat(t *testing.T, store archive.Store) {
	ctx := context.Background()
	expectedFiles := []string{"foo", "bar", "baz"}
	cleanup := func() {
		for _, file := range expectedFiles {
			_ = store.Delete(ctx, file)
		}
	}
	cleanup()
	defer cleanup()
	for _, file := range expectedFiles {
		if err := store.Put(ctx, bytes.NewReader([]byte(file)), file, time.Now()); err != nil {
			t.Fatalf("test setup: %s", err)
		}
	}
	table := map[string]struct {
		search          string
		expectedMatches []string
		expectedErr     error
	}{
		"multiple matches": {
			search:          "b",
			expectedMatches: []string{"bar", "baz"},
			expectedErr:     nil,
		},
		"one match": {
			search:          "f",
			expectedMatches: []string{"foo"},
			expectedErr:     nil,
		},
		"no matches": {
			search:          "nada",
			expectedMatches: []string{},
			expectedErr:     nil,
		},
	}
	for name, test := range table {
		test := test
		t.Run(name, func(t *testing.T) {
			actualMatches, err := store.Search(ctx, test.search)
			if err != nil && test.expectedErr == nil {
				t.Fatal(err)
			}
			if err != nil && test.expectedErr != nil && !errors.Is(err, test.expectedErr) && !strings.Contains(err.Error(), test.expectedErr.Error()) {
				t.Fatalf("expected error: %s, got %s", test.expectedErr, err)
			}
			if len(test.expectedMatches) != len(actualMatches) {
				t.Fatalf("expected %d matches, got %d, %#v", len(test.expectedMatches), len(actualMatches), actualMatches[0])
			}
			if err == nil {
				for index, match := range actualMatches {
					if match.Name != test.expectedMatches[index] {
						t.Fatalf("expected %s for match, got %s", test.expectedMatches[index], match.Name)
					}
				}
			}
		})
	}
	if _, err := store.Concat(ctx, 10, []string{"foo", "missing", "bar"}); err == nil {
		t.Fatal("expected error if any of the files in a concat array are missing")
	}
	concatBytes, err := store.Concat(ctx, 10, expectedFiles)
	if err != nil {
		t.Fatal(err)
	}
	expectedConcatBytes := make([][]byte, len(expectedFiles))
	for index, name := range expectedFiles {
		expectedConcatBytes[index] = []byte(name)
	}
	if !reflect.DeepEqual(expectedConcatBytes, concatBytes) {
		t.Fatalf("expected %s, got %s", expectedConcatBytes, concatBytes)
	}
}
