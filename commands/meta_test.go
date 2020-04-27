package commands_test

import (
	"bytes"
	"errors"
	"github.com/acomagu/bufpipe"
	"github.com/google/go-cmp/cmp"
	"github.com/tkellen/memorybox/commands"
	"github.com/tkellen/memorybox/internal/archive"
	"github.com/tkellen/memorybox/internal/store"

	"io/ioutil"
	"strings"
	"testing"
)

type testIO struct {
	reader *bufpipe.PipeReader
	writer *bufpipe.PipeWriter
}

func TestMetaGet(t *testing.T) {
	type testCase struct {
		store         *store.TestingStore
		io            *testIO
		fixtures      []store.TestingStoreFixture
		request       string
		expectedBytes []byte
		expectedErr   error
	}
	fixtures := []store.TestingStoreFixture{
		store.NewTestingStoreFixture("something", false, commands.Sha256),
		store.NewTestingStoreFixture("something", true, commands.Sha256),
	}
	table := map[string]testCase{
		"request existing metafile": {
			store: store.NewTestingStore(fixtures),
			io: func() *testIO {
				reader, writer := bufpipe.New(nil)
				return &testIO{
					reader: reader,
					writer: writer,
				}
			}(),
			fixtures:      fixtures,
			request:       fixtures[0].Name,    // first file is data object
			expectedBytes: fixtures[1].Content, // second file is metafile
			expectedErr:   nil,
		},
		"request existing metafile with failed copy to sink": {
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
			request:       fixtures[0].Name,    // first file is data object
			expectedBytes: fixtures[1].Content, // second file is metafile
			expectedErr:   errors.New("closed pipe"),
		},
	}
	for name, test := range table {
		test := test
		t.Run(name, func(t *testing.T) {
			err := commands.MetaGet(test.store, test.request, test.io.writer)
			if err != nil && test.expectedErr == nil {
				t.Fatal(err)
			}
			test.io.writer.Close()
			if err != nil && test.expectedErr != nil && !strings.Contains(err.Error(), test.expectedErr.Error()) {
				t.Fatalf("expected error: %s, got %s", test.expectedErr, err.Error())
			}
			if err == nil && test.expectedBytes != nil {
				actualBytes, _ := ioutil.ReadAll(test.io.reader)
				if diff := cmp.Diff(test.expectedBytes, actualBytes); diff != "" {
					t.Fatal(diff)
				}
			}
		})
	}
}

func TestMetaSetAndDelete(t *testing.T) {
	fixtures := []store.TestingStoreFixture{
		store.NewTestingStoreFixture("something", false, commands.Sha256),
		store.NewTestingStoreFixture("something", true, commands.Sha256),
	}
	store := store.NewTestingStore(fixtures)
	request := fixtures[0].Name
	expectedKeyAndValue := "test"
	// add meta key
	if err := commands.MetaSet(store, request, expectedKeyAndValue, expectedKeyAndValue); err != nil {
		t.Fatal(err)
	}
	// confirm key was set by asking for the metafile again
	reader, writer := bufpipe.New(nil)
	if err := commands.MetaGet(store, request, writer); err != nil {
		t.Fatal(err)
	}
	writer.Close()
	metaSetCheck, metaSetCheckErr := archive.NewFromReader(commands.Sha256, reader)
	if metaSetCheckErr != nil {
		t.Fatal(metaSetCheckErr)
	}
	if expectedKeyAndValue != metaSetCheck.MetaGet(expectedKeyAndValue) {
		t.Fatal("expected key %[1] to be set to %[1], saw %[1]", metaSetCheck.MetaGet(expectedKeyAndValue))
	}
	// remove key
	if err := commands.MetaDelete(store, request, expectedKeyAndValue); err != nil {
		t.Fatal(err)
	}
	// confirm key was removed by asking for it again
	reader, writer = bufpipe.New(nil)
	if err := commands.MetaGet(store, request, writer); err != nil {
		t.Fatal(err)
	}
	writer.Close()
	metaDeleteCheck, metaDeleteCheckErr := archive.NewFromReader(commands.Sha256, reader)
	if metaDeleteCheckErr != nil {
		t.Fatal(metaDeleteCheckErr)
	}
	if metaDeleteCheck.MetaGet(expectedKeyAndValue) != nil {
		t.Fatalf("expected key %s to be deleted", expectedKeyAndValue)
	}
}

func TestMetaFailures(t *testing.T) {
	type testCase struct {
		store         *store.TestingStore
		fixtures      []store.TestingStoreFixture
		request       string
		expectedBytes []byte
		expectedErr   error
	}
	fixtures := []store.TestingStoreFixture{
		store.NewTestingStoreFixture("something", false, commands.Sha256),
		store.NewTestingStoreFixture("something", true, commands.Sha256),
	}
	table := map[string]testCase{
		"request missing metafile": {
			store:         store.NewTestingStore(fixtures),
			fixtures:      fixtures,
			request:       "missing",
			expectedBytes: nil,
			expectedErr:   errors.New("0 objects"),
		},
		"request with failed search": {
			store: func() *store.TestingStore {
				store := store.NewTestingStore(fixtures)
				store.SearchErrorWith = errors.New("bad search")
				return store
			}(),
			request:       fixtures[0].Name,
			expectedBytes: nil,
			expectedErr:   errors.New("bad search"),
		},
		"request existing metafile with failed retrieval": {
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
	}
	for name, test := range table {
		test := test
		t.Run("Meta "+name, func(t *testing.T) {
			err := commands.MetaGet(test.store, test.request, bytes.NewBuffer([]byte{}))
			if err == nil {
				t.Fatal(err)
			}
			if !strings.Contains(err.Error(), test.expectedErr.Error()) {
				t.Fatalf("expected error: %s, got %s", test.expectedErr, err.Error())
			}
		})
		t.Run("MetaSet "+name, func(t *testing.T) {
			err := commands.MetaSet(test.store, test.request, "test", "test")
			if err == nil {
				t.Fatal(err)
			}
			if !strings.Contains(err.Error(), test.expectedErr.Error()) {
				t.Fatalf("expected error: %s, got %s", test.expectedErr, err.Error())
			}
		})
		t.Run("MetaDelete "+name, func(t *testing.T) {
			err := commands.MetaDelete(test.store, test.request, "test")
			if err == nil {
				t.Fatal(err)
			}
			if !strings.Contains(err.Error(), test.expectedErr.Error()) {
				t.Fatalf("expected error: %s, got %s", test.expectedErr, err.Error())
			}
		})
	}
}
