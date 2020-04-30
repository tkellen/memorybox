package store

import (
	"bytes"
	"context"
	"errors"
	"github.com/tkellen/filebuffer"
	"github.com/tkellen/memorybox/internal/testingstore"
	"github.com/tkellen/memorybox/pkg/archive"
	"io/ioutil"
	"log"
	"testing"
)

func TestIndexInternal(t *testing.T) {
	silentLogger := log.New(ioutil.Discard, "", 0)
	type testCase struct {
		store             *testingstore.Store
		integrityChecking bool
		sink              *filebuffer.Buffer
		expected          []byte
		expectedErr       error
	}
	datafile, _ := archive.NewSha256("fixture", filebuffer.New([]byte("test")))
	metafile := datafile.MetaFile()
	table := map[string]testCase{
		"fail on metafile missing datafile": {
			store:             testingstore.New([]*archive.File{metafile}),
			integrityChecking: true,
			expected:          nil,
			expectedErr:       errCorrupted,
		},
		"fail on datafile missing metafile": {
			store:             testingstore.New([]*archive.File{datafile}),
			integrityChecking: true,
			expected:          nil,
			expectedErr:       errCorrupted,
		},
		"fail on datafile corruption": {
			store: func() *testingstore.Store {
				datafile, _ := archive.NewSha256("fixture", filebuffer.New([]byte("test")))
				metafile := datafile.MetaFile()
				testStore := testingstore.New([]*archive.File{datafile, metafile})
				// Simulate data corruption by changing the content of a file in
				// store so the hash doesn't match.
				testStore.Data.Store(datafile.Name(), []byte("TEST"))
				return testStore
			}(),
			integrityChecking: true,
			expected:          nil,
			expectedErr:       errCorrupted,
		},
		"fail on store metafile containing conflicting datafile reference": func() testCase {
			datafile, _ := archive.NewSha256("fixture", filebuffer.New([]byte("test")))
			metafile := datafile.MetaFile()
			testStore := testingstore.New([]*archive.File{datafile, metafile})
			testStore.Data.Store(metafile.Name(), []byte("TEST"))
			return testCase{
				store:             testStore,
				integrityChecking: false,
				expected:          nil,
				expectedErr:       errCorrupted,
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
			err := Index(context.Background(), test.store, 10, silentLogger, test.integrityChecking, sink)
			if err != nil && test.expectedErr == nil {
				t.Fatal(err)
			}
			if err != nil && test.expectedErr != nil && !errors.Is(err, test.expectedErr) {
				t.Fatalf("expected error: %s, got %s", test.expectedErr, err)
			}
			if err == nil && test.expected != nil {
				if !bytes.Equal(test.expected, sink.Buff.Bytes()) {
					t.Fatalf("expected %s, got %s", test.expected, sink.Buff.Bytes())
				}
			}
		})
	}
}
