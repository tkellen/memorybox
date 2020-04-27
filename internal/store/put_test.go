package store_test

import (
	"github.com/tkellen/memorybox/internal/archive"
	"github.com/tkellen/memorybox/internal/store"
	"github.com/tkellen/memorybox/internal/store/testingstore"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"testing"
)

func TestPutManySuccess(t *testing.T) {
	silentLogger := log.New(ioutil.Discard, "", 0)
	type testCase struct {
		store                    store.Store
		fixtures                 []testingstore.Fixture
		concurrency              int
		expectedStoredFilesCount int
		expectedErr              error
	}
	hashFn := store.Sha256
	table := map[string]testCase{
		"store two data files": {
			store: &testingstore.Store{Data: map[string][]byte{}},
			fixtures: []testingstore.Fixture{
				testingstore.NewFixture("foo-content", false, hashFn),
				testingstore.NewFixture("bar-content", false, hashFn),
			},
			expectedStoredFilesCount: 4, // 2x fixtures, one meta file for each
			concurrency:              2,
		},
		"store one meta file and one data file": {
			store: &testingstore.Store{Data: map[string][]byte{}},
			fixtures: []testingstore.Fixture{
				testingstore.NewFixture("foo-content", false, hashFn),
				testingstore.NewFixture("{\"key\":\"value\"}", true, hashFn),
			},
			expectedStoredFilesCount: 3, // one content, two meta
			concurrency:              2,
		},
		"store the same file multiple times": {
			store: &testingstore.Store{Data: map[string][]byte{}},
			fixtures: []testingstore.Fixture{
				testingstore.NewFixture("foo-content", false, hashFn),
				testingstore.NewFixture("foo-content", false, hashFn),
			},
			expectedStoredFilesCount: 2, // one content, one meta
			concurrency:              2,
		},
	}
	for name, test := range table {
		test := test
		t.Run(name, func(t *testing.T) {
			// Generate temporary files to serve as input.
			tempDir, tempErr := ioutil.TempDir("", "*")
			if tempErr != nil {
				t.Fatalf("test setup: %s", tempErr)
			}
			defer os.RemoveAll(tempDir)
			inputs, err := fixtures(tempDir, test.fixtures)
			if err != nil {
				t.Fatalf("test setup: %s", err)
			}
			// Run put twice, it should be idempotent.
			for i := 0; i < 2; i++ {
				err := store.PutMany(test.store, hashFn, inputs, test.concurrency, silentLogger, []string{})
				if err != nil && test.expectedErr == nil {
					t.Fatal(err)
				}
				if err != nil && test.expectedErr != nil && !strings.Contains(err.Error(), test.expectedErr.Error()) {
					t.Fatalf("expected error: %s, got %s", test.expectedErr, err.Error())
				}
			}
			// Ensure all files were persisted.
			for _, fixture := range test.fixtures {
				if !test.store.Exists(fixture.Name) {
					t.Fatalf("expected %s to be in store", fixture.Name)
				}
				// if the fixture wasn't a metafile, make sure one was made
				if strings.HasPrefix(fixture.Name, archive.MetaFilePrefix) {
					metaFileName := archive.MetaFileName(fixture.Name)
					if test.store.Exists(metaFileName) {
						t.Fatalf("expected %s to be in store", metaFileName)
					}
				}
			}
			// Ensure no unexpected files were persisted.
			actualStoredFileCount := len(test.store.(*testingstore.Store).Data)
			if actualStoredFileCount != test.expectedStoredFilesCount {
				t.Fatalf("expected %d files in store, saw %d", test.expectedStoredFilesCount, actualStoredFileCount)
			}
		})
	}
}

func TestPutManyFail(t *testing.T) {
	silentLogger := log.New(ioutil.Discard, "", 0)
	err := store.PutMany(&testingstore.Store{}, store.Sha256, []string{"nope"}, 2, silentLogger, []string{})
	if err == nil {
		t.Fatal("expected error")
	}
}
