package memorybox_test

/*
import (
	"context"
	"errors"
	"github.com/google/go-cmp/cmp"
	"github.com/tkellen/memorybox/internal/archive"
	"github.com/tkellen/memorybox/lib"
	"github.com/tkellen/memorybox/pkg/index"
	"github.com/tkellen/memorybox/pkg/testingstore"
	"strings"
	"testing"
)

func TestIndex(t *testing.T) {
	type testCase struct {
		ctx           context.Context
		store         memorybox.Store
		io            *testIO
		expectedIndex map[string][]byte
		expectedErr   error
	}
	fixtures := []testingstore.Fixture{
		testingstore.NewFixture("something", false, memorybox.Sha256),
		testingstore.NewFixture("something", true, memorybox.Sha256),
	}
	fixtureIndex := map[string][]byte{
		archive.ToDataFileName(fixtures[1].Name): fixtures[1].Content,
	}
	testError := errors.New("bad time")
	table := map[string]testCase{
		"valid index of all metafiles keyed by the data file they describe": {
			ctx:           context.Background(),
			store:         testingstore.New(fixtures),
			expectedIndex: fixtureIndex,
			expectedErr:   nil,
		},
		"failure to fetch file from store": {
			ctx: context.Background(),
			store: func() memorybox.Store {
				store := testingstore.New(fixtures)
				store.GetErrorWith = testError
				return store
			}(),
			expectedIndex: nil,
			expectedErr:   testError,
		},
		"failure to search store for metafiles": {
			ctx: context.Background(),
			store: func() memorybox.Store {
				store := testingstore.New(fixtures)
				store.SearchErrorWith = testError
				return store
			}(),
			expectedIndex: nil,
			expectedErr:   testError,
		},
		"failure to read file from store": {
			ctx: context.Background(),
			store: func() memorybox.Store {
				store := testingstore.New(fixtures)
				store.GetReturnsTimeoutReader = true
				return store
			}(),
			expectedIndex: nil,
			expectedErr:   errors.New("timeout"),
		},
	}
	for name, test := range table {
		test := test
		t.Run(name, func(t *testing.T) {
			actualIndex, err := index.Run(test.ctx, test.store)
			if err != nil && test.expectedErr == nil {
				t.Fatal(err)
			}
			if err != nil && test.expectedErr != nil && !strings.Contains(err.Error(), test.expectedErr.Error()) {
				t.Fatalf("expected error: %s, got %s", test.expectedErr, err.Error())
			}
			if diff := cmp.Diff(test.expectedIndex, actualIndex); diff != "" {
				t.Fatal(diff)
			}
		})
	}
}
*/
