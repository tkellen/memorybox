package operations_test

import (
	"context"
	"errors"
	"fmt"
	"github.com/mattetti/filebuffer"
	"github.com/tkellen/memorybox/pkg/archive"
	"github.com/tkellen/memorybox/pkg/operations"
	"io"
	"os"
	"testing"
)

func TestImport(t *testing.T) {
	type testCase struct {
		store                    *TestingStore
		fixtures                 [][]byte
		shutdownServer           func() error
		imports                  *filebuffer.Buffer
		expectedStoredFilesCount int
		expectedErr              error
	}
	table := map[string]testCase{
		"files are uploaded": func() testCase {
			fixtures := [][]byte{
				[]byte("foo-content"),
				[]byte("bar-content"),
			}
			sources, done := fixtureServer(t, fixtures)
			imports := filebuffer.New(
				[]byte(fmt.Sprintf("%s\t{}\n%s\t{}\n", sources[0], sources[1])),
			)
			return testCase{
				store:                    NewTestingStore([]*archive.File{}),
				fixtures:                 fixtures,
				imports:                  imports,
				shutdownServer:           done,
				expectedStoredFilesCount: 4, // two content, two meta
				expectedErr:              nil,
			}
		}(),
		"duplicate files are skipped": func() testCase {
			fixtures := [][]byte{
				[]byte("foo-content"),
				[]byte("foo-content"),
			}
			sources, done := fixtureServer(t, fixtures)
			imports := filebuffer.New(
				[]byte(fmt.Sprintf("%s\t{}\n%s\t{}\n", sources[0], sources[1])),
			)
			return testCase{
				store:                    NewTestingStore([]*archive.File{}),
				fixtures:                 fixtures,
				imports:                  imports,
				shutdownServer:           done,
				expectedStoredFilesCount: 2, // one content, one meta
				expectedErr:              nil,
			}
		}(),
		"lines with no metadata are allowed": func() testCase {
			fixtures := [][]byte{
				[]byte("foo-content"),
				[]byte("test-content"),
			}
			sources, done := fixtureServer(t, fixtures)
			imports := filebuffer.New(
				[]byte(fmt.Sprintf("%s\t{}\n%s\n", sources[0], sources[1])),
			)
			// trigger coverage on existing source file being skipped
			dataFile, _ := archive.NewSha256(sources[1], filebuffer.New(fixtures[1]))
			metaFile := dataFile.MetaFile()
			return testCase{
				store:                    NewTestingStore([]*archive.File{dataFile, metaFile}),
				fixtures:                 fixtures,
				imports:                  imports,
				shutdownServer:           done,
				expectedStoredFilesCount: 4, // two content, two meta
				expectedErr:              nil,
			}
		}(),
		"duplicate lines with differing metadata fail": func() testCase {
			fixtures := [][]byte{
				[]byte("foo-content"),
			}
			sources, done := fixtureServer(t, fixtures)
			imports := filebuffer.New(
				[]byte(fmt.Sprintf("%s\t{}\n%s\t{\"test\":\"data\"}\n", sources[0], sources[0])),
			)
			return testCase{
				store:          NewTestingStore([]*archive.File{}),
				fixtures:       fixtures,
				imports:        imports,
				shutdownServer: done,
				expectedErr:    os.ErrInvalid,
			}
		}(),
	}
	for name, test := range table {
		test := test
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			defer test.shutdownServer()
			err := operations.Import(ctx, discardLogger(), test.store, 10, test.imports)
			if err != nil && test.expectedErr == nil {
				t.Fatal(err)
			}
			if err != nil && test.expectedErr != nil && !errors.Is(err, test.expectedErr) {
				t.Fatalf("expected error: %s, got %s", test.expectedErr, err)
			}
			if err == nil && test.expectedErr == nil {
				// Multiple runs should be idempotent
				test.imports.Seek(0, io.SeekStart)
				err := operations.Import(ctx, discardLogger(), test.store, 10, test.imports)
				if err != nil {
					t.Fatalf("unexpected error on repeated run: %s", err)
				}
				// Ensure right number of files were persisted.
				actualStoredFileCount := 0
				test.store.Data.Range(func(key, value interface{}) bool {
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
					if !test.store.Exists(ctx, fixture.Name()) {
						t.Fatalf("expected %s to be in store", name)
					}
					// if the fixture wasn't a metafile, make sure one was made
					if !fixture.IsMetaFile() {
						metaFileName := archive.MetaFileNameFrom(fixture.Name())
						if !test.store.Exists(ctx, metaFileName) {
							t.Fatalf("expected %s to be in store", metaFileName)
						}
					}
				}
			}
		})
	}
}
