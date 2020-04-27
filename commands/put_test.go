package commands_test

import (
	"github.com/tkellen/memorybox/commands"
	"github.com/tkellen/memorybox/internal/archive"
	"github.com/tkellen/memorybox/internal/store"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"testing"
)

func noopLogger(format string, v ...interface{}) {}
func fixtures(tempDir string, fixtures []store.TestingStoreFixture) ([]string, error) {
	files := make([]string, len(fixtures))
	for index, fixture := range fixtures {
		filepath := path.Join(tempDir, fixture.Name)
		err := ioutil.WriteFile(filepath, fixture.Content, 0644)
		if err != nil {
			return nil, err
		}
		files[index] = filepath
	}
	return files, nil
}

func TestPutSuccess(t *testing.T) {
	type testCase struct {
		store                    *store.TestingStore
		fixtures                 []store.TestingStoreFixture
		concurrency              int
		expectedStoredFilesCount int
		expectedErr              error
	}
	hashFn := commands.Sha256
	table := map[string]testCase{
		"store two data files": {
			store: &store.TestingStore{Data: map[string][]byte{}},
			fixtures: []store.TestingStoreFixture{
				store.NewTestingStoreFixture("foo-content", false, hashFn),
				store.NewTestingStoreFixture("bar-content", false, hashFn),
			},
			expectedStoredFilesCount: 4, // 2x fixtures, one meta file for each
			concurrency:              2,
		},
		"store one meta file and one data file": {
			store: &store.TestingStore{Data: map[string][]byte{}},
			fixtures: []store.TestingStoreFixture{
				store.NewTestingStoreFixture("foo-content", false, hashFn),
				store.NewTestingStoreFixture("{\"key\":\"value\"}", true, hashFn),
			},
			expectedStoredFilesCount: 3, // one content, two meta
			concurrency:              2,
		},
		"store the same file multiple times": {
			store: &store.TestingStore{Data: map[string][]byte{}},
			fixtures: []store.TestingStoreFixture{
				store.NewTestingStoreFixture("foo-content", false, hashFn),
				store.NewTestingStoreFixture("foo-content", false, hashFn),
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
				err := commands.Put(test.store, hashFn, inputs, test.concurrency, noopLogger, []string{})
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
			actualStoredFileCount := len(test.store.Data)
			if actualStoredFileCount != test.expectedStoredFilesCount {
				t.Fatalf("expected %d files in store, saw %d", test.expectedStoredFilesCount, actualStoredFileCount)
			}
		})
	}
}

func TestPutFail(t *testing.T) {
	err := commands.Put(&store.TestingStore{}, commands.Sha256, []string{"nope"}, 2, noopLogger, []string{})
	if err == nil {
		t.Fatal("expected error")
	}
}
