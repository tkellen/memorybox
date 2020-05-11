package operations_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/google/go-cmp/cmp"
	"github.com/mattetti/filebuffer"
	"github.com/tkellen/memorybox/pkg/archive"
	"github.com/tkellen/memorybox/pkg/operations"
	"github.com/tkellen/memorybox/pkg/store"
	"os"
	"testing"
)

func TestIndex(t *testing.T) {
	type testCase struct {
		store    *TestingStore
		sink     *filebuffer.Buffer
		rehash   bool
		expected []byte
		checkErr func(error) bool
	}
	datafile, _ := archive.NewSha256("fixture", filebuffer.New([]byte("test")))
	metafile := datafile.MetaFile()
	table := map[string]testCase{
		"fail on metafile missing datafile": {
			store:    NewTestingStore([]*archive.File{metafile}),
			rehash:   true,
			expected: nil,
			checkErr: store.IsCorrupted,
		},
		"fail on datafile missing metafile": {
			store:    NewTestingStore([]*archive.File{datafile}),
			rehash:   true,
			expected: nil,
			checkErr: store.IsCorrupted,
		},
		"fail on datafile corruption": {
			store: func() *TestingStore {
				datafile, _ := archive.NewSha256("fixture", filebuffer.New([]byte("test")))
				metafile := datafile.MetaFile()
				testStore := NewTestingStore([]*archive.File{datafile, metafile})
				// Simulate data corruption by changing the content of a file in
				// store so the hash doesn't match.
				testStore.Data.Store(datafile.Name(), []byte("TEST"))
				return testStore
			}(),
			rehash:   true,
			expected: nil,
			checkErr: store.IsCorrupted,
		},
		"fail on store metafile containing conflicting datafile reference": func() testCase {
			datafile, _ := archive.NewSha256("fixture", filebuffer.New([]byte("test")))
			metafile := datafile.MetaFile()
			testStore := NewTestingStore([]*archive.File{datafile, metafile})
			testStore.Data.Store(metafile.Name(), []byte("TEST"))
			return testCase{
				store:    testStore,
				rehash:   false,
				expected: nil,
				checkErr: store.IsCorrupted,
			}
		}(),
		"ignore datafile corruption when integrity checking is off": func() testCase {
			datafile, _ := archive.NewSha256("fixture", filebuffer.New([]byte("test")))
			metafile := datafile.MetaFile()
			testStore := NewTestingStore([]*archive.File{datafile, metafile})
			sink := filebuffer.New([]byte{})
			// get result of index before data corruption
			logger := discardLogger()
			logger.Stdout.SetOutput(sink)
			operations.Index(context.Background(), logger, testStore, 10, false)
			// simulate data corruption by changing the content of a file in
			// store so the hash doesn't match.
			testStore.Data.Store(datafile.Name(), []byte("TEST"))
			return testCase{
				store:    testStore,
				rehash:   false,
				expected: sink.Buff.Bytes(),
				checkErr: nil,
			}
		}(),
		"fail on store get datafile failure": func() testCase {
			badTime := errors.New("bad time")
			testStore := NewTestingStore(fixtureData())
			testStore.GetErrorWith = badTime
			return testCase{
				store: testStore,
				// enabling integrity checking makes the first store request be
				// for the contents of a datafile.
				rehash:   true,
				expected: nil,
				checkErr: func(err error) bool {
					return errors.Is(err, badTime)
				},
			}
		}(),
		"fail on store get metafile failure": func() testCase {
			badTime := errors.New("bad time")
			testStore := NewTestingStore(fixtureData())
			testStore.GetErrorWith = badTime
			return testCase{
				store: testStore,
				// disabling integrity checking makes the first store request be
				// for the contents of a metafile.
				rehash:   false,
				expected: nil,
				checkErr: func(err error) bool {
					return errors.Is(err, badTime)
				},
			}
		}(),
		"fail on store reading metafile failure": func() testCase {
			testStore := NewTestingStore(fixtureData())
			testStore.GetReturnsClosedReader = true
			return testCase{
				store: testStore,
				// Disabling integrity checking makes the first store request be
				// for the contents of a metafile (skipping validation of the
				// datafile)
				rehash:   false,
				expected: nil,
				checkErr: func(err error) bool {
					return errors.Is(err, os.ErrClosed)
				},
			}
		}(),
		"fail on store hashing failure": func() testCase {
			testStore := NewTestingStore(fixtureData())
			testStore.GetReturnsClosedReader = true
			return testCase{
				store: testStore,
				// Enabling integrity checking makes it so every datafile is
				// re-hashed to confirm it is still good. Without this, no hash
				// function would be called.
				rehash:   true,
				expected: nil,
				checkErr: func(err error) bool {
					return errors.Is(err, os.ErrClosed)
				},
			}
		}(),
		"fail on copy to sink": func() testCase {
			testStore := NewTestingStore(fixtureData())
			sink := filebuffer.New([]byte{})
			sink.Close()
			return testCase{
				store:    testStore,
				sink:     sink,
				rehash:   false,
				expected: nil,
				checkErr: func(err error) bool {
					return errors.Is(err, os.ErrClosed)
				},
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
			logger := discardLogger()
			logger.Stdout.SetOutput(sink)
			err := operations.Index(context.Background(), logger, test.store, 10, test.rehash)
			if err != nil && test.checkErr == nil {
				t.Fatal(err)
			}
			if err != nil && test.checkErr != nil && !test.checkErr(err) {
				t.Fatalf("did not expect error %s", err)
			}
			if err == nil && test.expected != nil {
				if diff := cmp.Diff(test.expected, sink.Buff.Bytes()); diff != "" {
					t.Fatal(diff)
				}
			}
		})
	}
}

func TestIndexUpdateTooLarge(t *testing.T) {
	fixtures := fixtureData()
	tooLarge := []byte(fmt.Sprintf(`{"memorybox":{"name":"%s"},"data":"%s"}`, fixtures[0].Name(), make([]byte, archive.MetaFileMaxSize*20, archive.MetaFileMaxSize*20)))
	err := operations.IndexUpdate(context.Background(), discardLogger(), NewTestingStore(fixtureData()), 10, bytes.NewReader(append(tooLarge, '\n')))
	if err == nil {
		t.Fatal("expected error on index item exceeding maximum allowable size")
	}
}
