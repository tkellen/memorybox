package store_test

import (
	"context"
	"errors"
	"fmt"
	"github.com/mattetti/filebuffer"
	"github.com/tkellen/memorybox/internal/testingstore"
	"github.com/tkellen/memorybox/pkg/archive"
	"github.com/tkellen/memorybox/pkg/store"
	"io/ioutil"
	"log"
	"os"
	"testing"
)

func TestImport(t *testing.T) {
	silentLogger := log.New(ioutil.Discard, "", 0)
	type testCase struct {
		fixtures                 [][]byte
		expectedStoredFilesCount int
		expectedErr              error
	}
	table := map[string]testCase{
		"files are uploaded": {
			fixtures: [][]byte{
				[]byte("foo-content"),
				[]byte("bar-content"),
			},
			expectedStoredFilesCount: 4, // two content, two meta
			expectedErr:              nil,
		},
	}
	for name, test := range table {
		test := test
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			testStore := testingstore.New([]*archive.File{})
			sources, done := fixtureServer(t, test.fixtures)
			defer done()
			tempFile, tempErr := ioutil.TempFile("", "*")
			if tempErr != nil {
				t.Fatalf("test setup: %s", tempErr)
			}
			for _, source := range sources {
				tempFile.WriteString(fmt.Sprintf("%s\t{}\n", source))
			}
			tempFile.Close()
			defer os.Remove(tempFile.Name())
			err := store.Import(ctx, testStore, []string{tempFile.Name()}, 10, silentLogger, silentLogger)
			if err != nil && test.expectedErr == nil {
				t.Fatal(err)
			}
			if err != nil && test.expectedErr != nil && !errors.Is(err, test.expectedErr) {
				t.Fatalf("expected error: %s, got %s", test.expectedErr, err)
			}
			if err == nil {
				// Ensure right number of files were persisted.
				actualStoredFileCount := 0
				testStore.Data.Range(func(key, value interface{}) bool {
					actualStoredFileCount = actualStoredFileCount + 1
					return true
				})
				// Ensure no unexpected files were persisted.
				if actualStoredFileCount != test.expectedStoredFilesCount {
					t.Fatalf("expected %d files in store, saw %d", test.expectedStoredFilesCount, actualStoredFileCount)
				}
				// Ensure right files/metadata was persisted.
				for _, content := range test.fixtures {
					fixture, err := archive.NewSha256("fixture", filebuffer.New(content))
					if err != nil {
						t.Fatalf("test setup: %s", err)
					}
					if !testStore.Exists(ctx, fixture.Name()) {
						t.Fatalf("expected %s to be in store", name)
					}
					// if the fixture wasn't a metafile, make sure one was made
					if !fixture.IsMetaFile() {
						metaFileName := archive.MetaFileNameFrom(fixture.Name())
						if !testStore.Exists(ctx, metaFileName) {
							t.Fatalf("expected %s to be in store", metaFileName)
						}
					}
				}
			}
		})
	}
}
