package operations_test

import (
	"context"
	"errors"
	"github.com/google/go-cmp/cmp"
	"github.com/mattetti/filebuffer"
	"github.com/tkellen/memorybox/pkg/archive"
	"github.com/tkellen/memorybox/pkg/operations"
	"io/ioutil"
	"os"
	"testing"
)

func TestMetaGet(t *testing.T) {
	type testCase struct {
		store         *TestingStore
		sink          *filebuffer.Buffer
		fixtures      []*archive.File
		request       string
		expectedBytes []byte
		expectedErr   error
	}
	table := map[string]testCase{
		"request existing metafile": func() testCase {
			dataFile, _ := archive.NewSha256("fixture", filebuffer.New([]byte("test")))
			metaFile := dataFile.MetaFile()
			metaContent, _ := ioutil.ReadAll(dataFile.MetaFile())
			return testCase{
				store:         NewTestingStore([]*archive.File{dataFile, metaFile}),
				request:       dataFile.Name(),
				expectedBytes: metaContent,
				expectedErr:   nil,
			}
		}(),
		"request existing metafile with failed filebuffer creation": func() testCase {
			dataFile, _ := archive.NewSha256("fixture", filebuffer.New([]byte("test")))
			metaFile := dataFile.MetaFile()
			metaContent, _ := ioutil.ReadAll(dataFile.MetaFile())
			store := NewTestingStore([]*archive.File{dataFile, metaFile})
			store.GetReturnsClosedReader = true
			return testCase{
				store:         store,
				request:       dataFile.Name(),
				expectedBytes: metaContent,
				expectedErr:   os.ErrClosed,
			}
		}(),
		"request existing metafile with failed copy to sink": func() testCase {
			dataFile, _ := archive.NewSha256("fixture", filebuffer.New([]byte("test")))
			metaFile := dataFile.MetaFile()
			metaContent, _ := ioutil.ReadAll(dataFile.MetaFile())
			sink := filebuffer.New([]byte{})
			sink.Close()
			return testCase{
				store:         NewTestingStore([]*archive.File{dataFile, metaFile}),
				sink:          sink,
				request:       dataFile.Name(),
				expectedBytes: metaContent,
				expectedErr:   os.ErrClosed,
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
			err := operations.MetaGet(context.Background(), logger, test.store, test.request)
			if err != nil && test.expectedErr == nil {
				t.Fatal(err)
			}
			if err != nil && test.expectedErr != nil && !errors.Is(err, test.expectedErr) {
				t.Fatalf("expected error: %s, got %s", test.expectedErr, err)
			}
			if err == nil && test.expectedBytes != nil {
				if diff := cmp.Diff(test.expectedBytes, sink.Buff.Bytes()); diff != "" {
					t.Fatal(diff)
				}
			}
		})
	}
}

func TestMetaSetAndDelete(t *testing.T) {
	ctx := context.Background()
	dataFile, _ := archive.NewSha256("fixture", filebuffer.New([]byte("test")))
	metaFile := dataFile.MetaFile()
	testStore := NewTestingStore([]*archive.File{dataFile, metaFile})
	request := dataFile.Name()
	expectedKeyAndValue := "test"
	// add meta key
	if err := operations.MetaSet(ctx, discardLogger(), testStore, request, expectedKeyAndValue, expectedKeyAndValue); err != nil {
		t.Fatal(err)
	}
	// get meta file again
	getFile := filebuffer.New([]byte{})
	getFileLogger := discardLogger()
	getFileLogger.Stdout.SetOutput(getFile)
	if err := operations.MetaGet(ctx, getFileLogger, testStore, request); err != nil {
		t.Fatal(err)
	}
	// check if value persisted
	metaSetCheck, metaSetCheckErr := archive.NewSha256("test", getFile)
	if metaSetCheckErr != nil {
		t.Fatal(metaSetCheckErr)
	}
	if expectedKeyAndValue != metaSetCheck.MetaGet(expectedKeyAndValue) {
		t.Fatalf("expected key %s to be set to %s, saw %s", expectedKeyAndValue, expectedKeyAndValue, metaSetCheck.MetaGet(expectedKeyAndValue))
	}
	// remove key
	if err := operations.MetaDelete(ctx, discardLogger(), testStore, request, expectedKeyAndValue); err != nil {
		t.Fatal(err)
	}
	// confirm key was removed by asking for it again
	setFile := filebuffer.New([]byte{})
	setFileLogger := discardLogger()
	setFileLogger.Stdout.SetOutput(setFile)
	if err := operations.MetaGet(ctx, setFileLogger, testStore, request); err != nil {
		t.Fatal(err)
	}
	metaDeleteCheck, metaDeleteCheckErr := archive.NewSha256("test", setFile)
	if metaDeleteCheckErr != nil {
		t.Fatal(metaDeleteCheckErr)
	}
	if metaDeleteCheck.MetaGet(expectedKeyAndValue) != nil {
		t.Fatalf("expected key %s to be deleted", expectedKeyAndValue)
	}
}

func TestMetaFailures(t *testing.T) {
	type testCase struct {
		store         *TestingStore
		request       string
		expectedBytes []byte
		expectedErr   error
	}
	table := map[string]testCase{
		"request missing metafile": {
			store:         NewTestingStore([]*archive.File{}),
			request:       "missing",
			expectedBytes: nil,
			expectedErr:   os.ErrNotExist,
		},
		"request with failed search": func() testCase {
			store := NewTestingStore([]*archive.File{})
			store.SearchErrorWith = errors.New("bad search")
			return testCase{
				store:         store,
				request:       "whatever",
				expectedBytes: nil,
				expectedErr:   store.SearchErrorWith,
			}
		}(),
		"request existing metafile with failed retrieval": func() testCase {
			dataFile, _ := archive.NewSha256("fixture", filebuffer.New([]byte("test")))
			metaFile := dataFile.MetaFile()
			store := NewTestingStore([]*archive.File{dataFile, metaFile})
			store.GetErrorWith = errors.New("bad get")
			return testCase{
				store:         store,
				request:       dataFile.Name(),
				expectedBytes: nil,
				expectedErr:   store.GetErrorWith,
			}
		}(),
	}
	for name, test := range table {
		test := test
		t.Run("Meta "+name, func(t *testing.T) {
			err := operations.MetaGet(context.Background(), discardLogger(), test.store, test.request)
			if err == nil {
				t.Fatal(err)
			}
			if !errors.Is(err, test.expectedErr) {
				t.Fatalf("expected error: %s, got %s", test.expectedErr, err)
			}
		})
		t.Run("MetaSet "+name, func(t *testing.T) {
			err := operations.MetaSet(context.Background(), discardLogger(), test.store, test.request, "test", "test")
			if err == nil {
				t.Fatal(err)
			}
			if !errors.Is(err, test.expectedErr) {
				t.Fatalf("expected error: %s, got %s", test.expectedErr, err)
			}
		})
		t.Run("MetaDelete "+name, func(t *testing.T) {
			err := operations.MetaDelete(context.Background(), discardLogger(), test.store, test.request, "test")
			if err == nil {
				t.Fatal(err)
			}
			if !errors.Is(err, test.expectedErr) {
				t.Fatalf("expected error: %s, got %s", test.expectedErr, err)
			}
		})
	}
}