package memorybox_test

import (
	"context"
	"github.com/tkellen/memorybox/internal/archive"
	"github.com/tkellen/memorybox/lib"
	"github.com/tkellen/memorybox/pkg/testingstore"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"testing"
)

func TestPutManySuccess(t *testing.T) {
	silentLogger := log.New(ioutil.Discard, "", 0)
	type testCase struct {
		ctx                      context.Context
		store                    memorybox.Store
		fixtures                 []testingstore.Fixture
		concurrency              int
		expectedStoredFilesCount int
		expectedErr              error
	}
	hashFn := memorybox.Sha256
	table := map[string]testCase{
		"store two data files": func() testCase {
			fixtures := []testingstore.Fixture{
				testingstore.NewFixture("foo-content", false, hashFn),
				testingstore.NewFixture("bar-content", false, hashFn),
			}
			return testCase{
				ctx:                      context.Background(),
				store:                    testingstore.New(fixtures),
				fixtures:                 fixtures,
				concurrency:              2,
				expectedStoredFilesCount: 4, // 2x fixtures, one meta file for each
			}
		}(),
		"store one meta file and one data file": func() testCase {
			fixtures := []testingstore.Fixture{
				testingstore.NewFixture("foo-content", false, hashFn),
				testingstore.NewFixture(`{"key":"value"}`, true, hashFn),
			}
			return testCase{
				ctx:                      context.Background(),
				store:                    testingstore.New(fixtures),
				fixtures:                 fixtures,
				concurrency:              2,
				expectedStoredFilesCount: 3, // one content, two meta
			}
		}(),
		"store the same file multiple times": func() testCase {
			fixtures := []testingstore.Fixture{
				testingstore.NewFixture("foo-content", false, hashFn),
				testingstore.NewFixture("foo-content", false, hashFn),
			}
			return testCase{
				ctx:                      context.Background(),
				store:                    testingstore.New(fixtures),
				fixtures:                 fixtures,
				concurrency:              2,
				expectedStoredFilesCount: 2, // one content, one meta
			}
		}(),
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
				err := memorybox.PutMany(test.ctx, test.store, hashFn, inputs, test.concurrency, silentLogger, []string{})
				if err != nil && test.expectedErr == nil {
					t.Fatal(err)
				}
				if err != nil && test.expectedErr != nil && !strings.Contains(err.Error(), test.expectedErr.Error()) {
					t.Fatalf("expected error: %s, got %s", test.expectedErr, err.Error())
				}
			}
			// Ensure all files were persisted.
			for _, fixture := range test.fixtures {
				if !test.store.Exists(test.ctx, fixture.Name) {
					t.Fatalf("expected %s to be in store", fixture.Name)
				}
				// if the fixture wasn't a metafile, make sure one was made
				if !archive.IsMetaFileName(fixture.Name) {
					metaFileName := archive.ToMetaFileName(fixture.Name)
					if !test.store.Exists(test.ctx, metaFileName) {
						t.Fatalf("expected %s to be in store", metaFileName)
					}
				}
			}
			actualStoredFileCount := 0
			test.store.(*testingstore.Store).Data.Range(func(key, value interface{}) bool {
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

func TestPutManyFail(t *testing.T) {
	silentLogger := log.New(ioutil.Discard, "", 0)
	err := memorybox.PutMany(context.Background(), testingstore.New([]testingstore.Fixture{}), memorybox.Sha256, []string{"nope"}, 2, silentLogger, []string{})
	if err == nil {
		t.Fatal("expected error")
	}
}
