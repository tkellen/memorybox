package memorybox_test

import (
	"errors"
	"github.com/acomagu/bufpipe"
	"github.com/google/go-cmp/cmp"
	"github.com/tkellen/memorybox/lib"
	"github.com/tkellen/memorybox/pkg/testingstore"
	"io/ioutil"
	"path"
	"strings"
	"testing"
)

func fixtures(tempDir string, fixtures []testingstore.Fixture) ([]string, error) {
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

func TestGet(t *testing.T) {
	type testIO struct {
		reader *bufpipe.PipeReader
		writer *bufpipe.PipeWriter
	}
	type testCase struct {
		store         *testingstore.Store
		io            *testIO
		fixtures      []testingstore.Fixture
		request       string
		expectedBytes []byte
		expectedErr   error
	}
	fixtures := []testingstore.Fixture{
		testingstore.NewFixture("foo-content", false, memorybox.Sha256),
		testingstore.NewFixture("bar-content", false, memorybox.Sha256),
	}
	table := map[string]testCase{
		"get existing file": {
			store:         testingstore.New(fixtures),
			fixtures:      fixtures,
			request:       fixtures[0].Name,
			expectedBytes: fixtures[0].Content,
			expectedErr:   nil,
		},
		"get missing file": {
			store:         testingstore.New(fixtures),
			fixtures:      fixtures,
			request:       "missing",
			expectedBytes: nil,
			expectedErr:   errors.New("0 objects"),
		},
		"get with failed search": {
			store: func() *testingstore.Store {
				store := testingstore.New(fixtures)
				store.SearchErrorWith = errors.New("bad search")
				return store
			}(),
			fixtures:      fixtures,
			request:       fixtures[0].Name,
			expectedBytes: nil,
			expectedErr:   errors.New("bad search"),
		},
		"get existing file with failed retrieval": {
			store: func() *testingstore.Store {
				store := testingstore.New(fixtures)
				store.GetErrorWith = errors.New("bad get")
				return store
			}(),
			fixtures:      fixtures,
			request:       fixtures[0].Name,
			expectedBytes: nil,
			expectedErr:   errors.New("bad get"),
		},
		"get existing file with failed copy to sink": {
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
			err := memorybox.Get(test.store, test.request, writer)
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
