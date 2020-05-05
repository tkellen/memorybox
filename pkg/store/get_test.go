package store_test

import (
	"context"
	"errors"
	"github.com/google/go-cmp/cmp"
	"github.com/mattetti/filebuffer"
	"github.com/tkellen/memorybox/internal/testingstore"
	"github.com/tkellen/memorybox/pkg/archive"
	"github.com/tkellen/memorybox/pkg/store"
	"log"
	"os"
	"testing"
)

func TestGet(t *testing.T) {
	type testCase struct {
		store         *testingstore.Store
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
			store:         testingstore.New(fixtures),
			fixtures:      fixtures,
			request:       fixtures[0].Name(),
			expectedBytes: contents[0],
			expectedErr:   nil,
		},
		"get missing file": {
			store:         testingstore.New(fixtures),
			fixtures:      fixtures,
			request:       "missing",
			expectedBytes: nil,
			expectedErr:   os.ErrNotExist,
		},
		"get with failed search": func() testCase {
			store := testingstore.New(fixtures)
			store.SearchErrorWith = errors.New("bad search")
			return testCase{
				store:         store,
				fixtures:      fixtures,
				request:       fixtures[0].Name(),
				expectedBytes: nil,
				expectedErr:   store.SearchErrorWith,
			}
		}(),
		"get existing file with failed retrieval": func() testCase {
			store := testingstore.New(fixtures)
			store.GetErrorWith = errors.New("bad get")
			return testCase{
				store:         store,
				fixtures:      fixtures,
				request:       fixtures[0].Name(),
				expectedBytes: nil,
				expectedErr:   store.GetErrorWith,
			}
		}(),
		"get existing file with failed copy to sink": func() testCase {
			store := testingstore.New(fixtures)
			store.GetReturnsClosedReader = true
			return testCase{
				store:         store,
				fixtures:      fixtures,
				request:       fixtures[0].Name(),
				expectedBytes: nil,
				expectedErr:   os.ErrClosed,
			}
		}(),
	}
	for name, test := range table {
		test := test
		t.Run(name, func(t *testing.T) {
			sink := filebuffer.New([]byte{})
			err := store.Get(context.Background(), test.store, test.request, log.New(sink, "", 0))
			if err != nil && test.expectedErr == nil {
				t.Fatal(err)
			}
			if err != nil && test.expectedErr != nil && !errors.Is(err, test.expectedErr) {
				t.Fatalf("expected error: %s, got %s", test.expectedErr, err.Error())
			}
			if err == nil && test.expectedBytes != nil {
				if diff := cmp.Diff(test.expectedBytes, sink.Buff.Bytes()); diff != "" {
					t.Fatal(diff)
				}
			}
		})
	}
}
