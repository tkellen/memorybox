package store_test

import (
	"context"
	"github.com/mattetti/filebuffer"
	"github.com/tkellen/memorybox/internal/testingstore"
	"github.com/tkellen/memorybox/pkg/archive"
	"github.com/tkellen/memorybox/pkg/store"
	"io/ioutil"
	"log"
	"strings"
	"testing"
)

func TestPutSuccess(t *testing.T) {
	silentLogger := log.New(ioutil.Discard, "", 0)
	type testCase struct {
		fixtures                 [][]byte
		concurrency              int
		expectedStoredFilesCount int
		expectedErr              error
	}
	table := map[string]testCase{
		"store two data files": func() testCase {
			fixtures := [][]byte{
				[]byte("foo-content"),
				[]byte("bar-content"),
			}
			return testCase{
				fixtures:                 fixtures,
				concurrency:              2,
				expectedStoredFilesCount: 4, // 2x fixtures, one meta file for each
			}
		}(),
		"store one meta file and one data file": func() testCase {
			fixtures := [][]byte{
				[]byte("foo-content"),
				[]byte(`{"memorybox":{"file":"fixture"},"data":{}}`),
			}
			return testCase{
				fixtures:                 fixtures,
				concurrency:              2,
				expectedStoredFilesCount: 3, // one content, two meta
			}
		}(),
		"store the same file multiple times": func() testCase {
			fixtures := [][]byte{
				[]byte("foo-content"),
				[]byte("foo-content"),
			}
			return testCase{
				fixtures:                 fixtures,
				concurrency:              2,
				expectedStoredFilesCount: 2, // one content, one meta
			}
		}(),
	}
	for name, test := range table {
		test := test
		t.Run(name, func(t *testing.T) {
			inputs, done := fixtureServer(t, test.fixtures)
			defer done()
			testStore := testingstore.New([]*archive.File{})
			ctx := context.Background()
			// Run put twice, it should be idempotent.
			for i := 0; i < 2; i++ {
				err := store.Put(ctx, testStore, inputs, []string{}, test.concurrency, silentLogger, silentLogger)
				if err != nil && test.expectedErr == nil {
					t.Fatal(err)
				}
				if err != nil && test.expectedErr != nil && !strings.Contains(err.Error(), test.expectedErr.Error()) {
					t.Fatalf("expected error: %s, got %s", test.expectedErr, err.Error())
				}
			}
			// Ensure all files were persisted.
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
			actualStoredFileCount := 0
			testStore.Data.Range(func(key, value interface{}) bool {
				actualStoredFileCount = actualStoredFileCount + 1
				return true
			})
			// Ensure no unexpected files were persisted.
			if actualStoredFileCount != test.expectedStoredFilesCount {
				t.Fatalf("expected %d files in store, saw %d", test.expectedStoredFilesCount, actualStoredFileCount)
			}
		})
	}
}

func TestPutFail(t *testing.T) {
	err := store.Put(
		context.Background(),
		testingstore.New([]*archive.File{}),
		[]string{"nope"},
		[]string{},
		2,
		log.New(ioutil.Discard, "", 0),
		log.New(ioutil.Discard, "", 0),
	)
	if err == nil {
		t.Fatal("expected error")
	}
}
