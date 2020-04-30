package memorybox_test

import (
	"context"
	"errors"
	"github.com/acomagu/bufpipe"
	"github.com/google/go-cmp/cmp"
	"github.com/mattetti/filebuffer"
	"github.com/tkellen/memorybox/internal/testingstore"
	"github.com/tkellen/memorybox/lib"
	"github.com/tkellen/memorybox/pkg/archive"
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
		ctx           context.Context
		store         *testingstore.Store
		io            *testIO
		fixtures      []*archive.File
		request       string
		expectedBytes []byte
		expectedErr   error
	}
	contents := [][]byte{
		[]byte("foo-content"),
		[]byte("bar-content"),
	}
	var fixtures []*archive.File
	for _, content := range contents {
		fixture, err := archive.NewSha256("fixture", filebuffer.New(content))
		if err != nil {
			t.Fatalf("test setup: %s", err)
		}
		fixtures = append(fixtures, fixture)
	}
	table := map[string]testCase{
		"get existing file": {
			ctx:           context.Background(),
			store:         testingstore.New(fixtures),
			fixtures:      fixtures,
			request:       fixtures[0].Name(),
			expectedBytes: contents[0],
			expectedErr:   nil,
		},
		"get missing file": {
			ctx:           context.Background(),
			store:         testingstore.New(fixtures),
			fixtures:      fixtures,
			request:       "missing",
			expectedBytes: nil,
			expectedErr:   errors.New("0 objects"),
		},
		"get with failed search": {
			ctx: context.Background(),
			store: func() *testingstore.Store {
				store := testingstore.New(fixtures)
				store.SearchErrorWith = errors.New("bad search")
				return store
			}(),
			fixtures:      fixtures,
			request:       fixtures[0].Name(),
			expectedBytes: nil,
			expectedErr:   errors.New("bad search"),
		},
		"get existing file with failed retrieval": {
			ctx: context.Background(),
			store: func() *testingstore.Store {
				store := testingstore.New(fixtures)
				store.GetErrorWith = errors.New("bad get")
				return store
			}(),
			fixtures:      fixtures,
			request:       fixtures[0].Name(),
			expectedBytes: nil,
			expectedErr:   errors.New("bad get"),
		},
		"get existing file with failed copy to sink": {
			ctx:   context.Background(),
			store: testingstore.New(fixtures),
			io: func() *testIO {
				reader, writer := bufpipe.New(nil)
				reader.Close()
				return &testIO{
					reader: reader,
					writer: writer,
				}
			}(),
			fixtures:      fixtures,
			request:       fixtures[0].Name(),
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
			err := memorybox.Get(test.ctx, test.store, test.request, writer)
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
