package memorybox_test

import (
	"context"
	"fmt"
	"github.com/mattetti/filebuffer"
	"github.com/tkellen/memorybox/internal/testingstore"
	"github.com/tkellen/memorybox/lib"
	"github.com/tkellen/memorybox/pkg/archive"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"path"
	"strings"
	"testing"
)

func fixtureServer(t *testing.T, inputs map[string][]byte) ([]string, func() error) {
	listen, listenErr := net.Listen("tcp", "127.0.0.1:0")
	if listenErr != nil {
		t.Fatal(listenErr)
	}
	var urls []string
	for key := range inputs {
		urls = append(urls, fmt.Sprintf("http://%s/%s", listen.Addr().String(), key))
	}
	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for key, content := range inputs {
				if key == path.Base(r.URL.Path) {
					w.WriteHeader(http.StatusOK)
					w.Write(content)
					return
				}
			}
			w.WriteHeader(http.StatusNotFound)
		}),
	}
	go server.Serve(listen)
	return urls, server.Close
}

func fixturesToArchives(t *testing.T, fixtures map[string][]byte) []*archive.File {
	var archiveFiles []*archive.File
	for _, content := range fixtures {
		fixture, err := archive.NewSha256("fixture", filebuffer.New(content))
		if err != nil {
			t.Fatalf("test setup: %s", err)
		}
		archiveFiles = append(archiveFiles, fixture)
	}
	return archiveFiles
}

func TestPutSuccess(t *testing.T) {
	silentLogger := log.New(ioutil.Discard, "", 0)
	type testCase struct {
		ctx                      context.Context
		store                    memorybox.Store
		fixtures                 map[string][]byte
		concurrency              int
		expectedStoredFilesCount int
		expectedErr              error
	}
	table := map[string]testCase{
		"store two data files": func() testCase {
			fixtures := map[string][]byte{
				"foo": []byte("foo-content"),
				"bar": []byte("bar-content"),
			}
			return testCase{
				ctx:                      context.Background(),
				store:                    testingstore.New([]*archive.File{}),
				fixtures:                 fixtures,
				concurrency:              2,
				expectedStoredFilesCount: 4, // 2x fixtures, one meta file for each
			}
		}(),
		"store one meta file and one data file": func() testCase {
			fixtures := map[string][]byte{
				"foo":  []byte("foo-content"),
				"json": []byte(`{"memorybox":{"file":"fixture"},"data":{}}`),
			}
			return testCase{
				ctx:                      context.Background(),
				store:                    testingstore.New([]*archive.File{}),
				fixtures:                 fixtures,
				concurrency:              2,
				expectedStoredFilesCount: 3, // one content, two meta
			}
		}(),
		"store the same file multiple times": func() testCase {
			fixtures := map[string][]byte{
				"foo-one": []byte("foo-content"),
				"foo-two": []byte("foo-content"),
			}
			return testCase{
				ctx:                      context.Background(),
				store:                    testingstore.New([]*archive.File{}),
				fixtures:                 fixtures,
				concurrency:              2,
				expectedStoredFilesCount: 2, // one content, one meta
			}
		}(),
	}
	for name, test := range table {
		test := test
		t.Run(name, func(t *testing.T) {
			inputs, done := fixtureServer(t, test.fixtures)
			defer done()
			// Run put twice, it should be idempotent.
			for i := 0; i < 2; i++ {
				err := memorybox.Put(test.ctx, test.store, inputs, test.concurrency, silentLogger, []string{})
				if err != nil && test.expectedErr == nil {
					t.Fatal(err)
				}
				if err != nil && test.expectedErr != nil && !strings.Contains(err.Error(), test.expectedErr.Error()) {
					t.Fatalf("expected error: %s, got %s", test.expectedErr, err.Error())
				}
			}
			// Ensure all files were persisted.
			for _, file := range fixturesToArchives(t, test.fixtures) {
				if !test.store.Exists(test.ctx, file.Name()) {
					t.Fatalf("expected %s to be in store", name)
				}
				// if the fixture wasn't a metafile, make sure one was made
				if !file.IsMetaFile() {
					metaFileName := archive.MetaFileNameFrom(file.Name())
					if !test.store.Exists(test.ctx, metaFileName) {
						t.Fatalf("expected %s to be in store", metaFileName)
					}
				}
			}
			actualStoredFileCount := 0
			test.store.(*testingstore.Store).Data.Range(func(key, value interface{}) bool {
				actualStoredFileCount = actualStoredFileCount + 1
				return true
			})
			// Ensure no unexpected files were persisted.
			if actualStoredFileCount != test.expectedStoredFilesCount {
				t.Fatalf("expected %d files in store, saw %d", test.expectedStoredFilesCount, actualStoredFileCount)
			}
		})
	}
}

func TestPutFail(t *testing.T) {
	err := memorybox.Put(
		context.Background(),
		testingstore.New([]*archive.File{}),
		[]string{"nope"},
		2,
		log.New(ioutil.Discard, "", 0),
		[]string{},
	)
	if err == nil {
		t.Fatal("expected error")
	}
}
