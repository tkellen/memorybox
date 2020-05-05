package store_test

import (
	"context"
	"errors"
	"github.com/google/go-cmp/cmp"
	"github.com/mattetti/filebuffer"
	"github.com/tkellen/memorybox/internal/testingstore"
	"github.com/tkellen/memorybox/pkg/archive"
	"github.com/tkellen/memorybox/pkg/store"
	"io/ioutil"
	"log"
	"os"
	"testing"
)

func fixtureData() []*archive.File {
	dataFile, _ := archive.NewSha256("fixture", filebuffer.New([]byte("test")))
	metaFile := dataFile.MetaFile()
	return []*archive.File{dataFile, metaFile}
}

func TestIndex(t *testing.T) {
	silentLogger := log.New(ioutil.Discard, "", 0)
	type testCase struct {
		store             *testingstore.Store
		sink              *filebuffer.Buffer
		integrityChecking bool
		expected          []byte
		expectedErr       error
	}

	table := map[string]testCase{
		"ignore datafile corruption when integrity checking is off": func() testCase {
			datafile, _ := archive.NewSha256("fixture", filebuffer.New([]byte("test")))
			metafile := datafile.MetaFile()
			testStore := testingstore.New([]*archive.File{datafile, metafile})
			sink := filebuffer.New([]byte{})
			// get result of index before data corruption
			store.Index(context.Background(), testStore, 10, false, silentLogger, log.New(sink, "", 0))
			// simulate data corruption by changing the content of a file in
			// store so the hash doesn't match.
			testStore.Data.Store(datafile.Name(), []byte("TEST"))
			return testCase{
				store:             testStore,
				integrityChecking: false,
				expected:          sink.Buff.Bytes(),
				expectedErr:       nil,
			}
		}(),
		"fail on store get datafile failure": func() testCase {
			err := errors.New("bad time")
			testStore := testingstore.New(fixtureData())
			testStore.GetErrorWith = err
			return testCase{
				store: testStore,
				// enabling integrity checking makes the first store request be
				// for the contents of a datafile.
				integrityChecking: true,
				expected:          nil,
				expectedErr:       err,
			}
		}(),
		"fail on store get metafile failure": func() testCase {
			err := errors.New("bad time")
			testStore := testingstore.New(fixtureData())
			testStore.GetErrorWith = err
			return testCase{
				store: testStore,
				// disabling integrity checking makes the first store request be
				// for the contents of a metafile.
				integrityChecking: false,
				expected:          nil,
				expectedErr:       err,
			}
		}(),
		"fail on store reading metafile failure": func() testCase {
			testStore := testingstore.New(fixtureData())
			testStore.GetReturnsClosedReader = true
			return testCase{
				store: testStore,
				// Disabling integrity checking makes the first store request be
				// for the contents of a metafile (skipping validation of the
				// datafile)
				integrityChecking: false,
				expected:          nil,
				expectedErr:       os.ErrClosed,
			}
		}(),
		"fail on store hashing failure": func() testCase {
			testStore := testingstore.New(fixtureData())
			testStore.GetReturnsClosedReader = true
			return testCase{
				store: testStore,
				// Enabling integrity checking makes it so every datafile is
				// re-hashed to confirm it is still good. Without this, no hash
				// function would be called.
				integrityChecking: true,
				expected:          nil,
				expectedErr:       os.ErrClosed,
			}
		}(),
		"fail on copy to sink": func() testCase {
			testStore := testingstore.New(fixtureData())
			sink := filebuffer.New([]byte{})
			sink.Close()
			return testCase{
				store:             testStore,
				sink:              sink,
				integrityChecking: false,
				expected:          nil,
				expectedErr:       os.ErrClosed,
			}
		}(),
	}
	for name, test := range table {
		test := test
		t.Run(name, func(t *testing.T) {
			sink := filebuffer.New([]byte{})
			if test.sink != nil {
				sink = test.sink
			}
			err := store.Index(context.Background(), test.store, 10, test.integrityChecking, silentLogger, log.New(sink, "", 0))
			if err != nil && test.expectedErr == nil {
				t.Fatal(err)
			}
			if err != nil && test.expectedErr != nil && !errors.Is(err, test.expectedErr) {
				t.Fatalf("expected error: %s, got %s", test.expectedErr, err)
			}
			if err == nil && test.expected != nil {
				if diff := cmp.Diff(test.expected, sink.Buff.Bytes()); diff != "" {
					t.Fatal(diff)
				}
			}
		})
	}
}
