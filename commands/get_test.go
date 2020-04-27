package commands_test

import (
	"errors"
	"github.com/acomagu/bufpipe"
	"github.com/google/go-cmp/cmp"
	"github.com/tkellen/memorybox/commands"
	"github.com/tkellen/memorybox/internal/store"
	"io/ioutil"
	"strings"
	"testing"
)

func TestGet(t *testing.T) {
	type testIO struct {
		reader *bufpipe.PipeReader
		writer *bufpipe.PipeWriter
	}
	type testCase struct {
		store         *store.TestingStore
		io            *testIO
		fixtures      []store.TestingStoreFixture
		request       string
		expectedBytes []byte
		expectedErr   error
	}
	fixtures := []store.TestingStoreFixture{
		store.NewTestingStoreFixture("foo-content", false, commands.Sha256),
		store.NewTestingStoreFixture("bar-content", false, commands.Sha256),
	}
	table := map[string]testCase{
		"get existing file": {
			store:         store.NewTestingStore(fixtures),
			fixtures:      fixtures,
			request:       fixtures[0].Name,
			expectedBytes: fixtures[0].Content,
			expectedErr:   nil,
		},
		"get missing file": {
			store:         store.NewTestingStore(fixtures),
			fixtures:      fixtures,
			request:       "missing",
			expectedBytes: nil,
			expectedErr:   errors.New("0 objects"),
		},
		"get with failed search": {
			store: func() *store.TestingStore {
				store := store.NewTestingStore(fixtures)
				store.SearchErrorWith = errors.New("bad search")
				return store
			}(),
			fixtures:      fixtures,
			request:       fixtures[0].Name,
			expectedBytes: nil,
			expectedErr:   errors.New("bad search"),
		},
		"get existing file with failed retrieval": {
			store: func() *store.TestingStore {
				store := store.NewTestingStore(fixtures)
				store.GetErrorWith = errors.New("bad get")
				return store
			}(),
			fixtures:      fixtures,
			request:       fixtures[0].Name,
			expectedBytes: nil,
			expectedErr:   errors.New("bad get"),
		},
		"get existing file with failed copy to sink": {
			store: store.NewTestingStore(fixtures),
			io: func() *testIO {
				reader, writer := bufpipe.New(nil)
				reader.Close()
				return &testIO{
					reader: reader,
					writer: writer,
				}
			}(),
			fixtures:      fixtures,
			request:       fixtures[0].Name,
			expectedBytes: nil,
			expectedErr:   errors.New("closed pipe"),
		},
	}
	for name, test := range table {
		test := test
		t.Run(name, func(t *testing.T) {
			reader, writer := bufpipe.New(nil)
			if test.io != nil {
				reader = test.io.reader
				writer = test.io.writer
			}
			err := commands.Get(test.store, test.request, writer)
			if err != nil && test.expectedErr == nil {
				t.Fatal(err)
			}
			writer.Close()
			if err != nil && test.expectedErr != nil && !strings.Contains(err.Error(), test.expectedErr.Error()) {
				t.Fatalf("expected error: %s, got %s", test.expectedErr, err.Error())
			}
			if err == nil && test.expectedBytes != nil {
				actualBytes, _ := ioutil.ReadAll(reader)
				if diff := cmp.Diff(test.expectedBytes, actualBytes); diff != "" {
					t.Fatal(diff)
				}
			}
		})
	}
}
