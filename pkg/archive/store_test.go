package archive_test

import (
	"bytes"
	"context"
	"fmt"
	"github.com/tkellen/memorybox/internal/test"
	"github.com/tkellen/memorybox/pkg/archive"
	"github.com/tkellen/memorybox/pkg/file"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"path"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

func fixtureServer(t *testing.T, inputs [][]byte) ([]string, func() error) {
	listen, listenErr := net.Listen("tcp", "127.0.0.1:0")
	if listenErr != nil {
		t.Fatal(listenErr)
	}
	var urls []string
	for _, content := range inputs {
		urls = append(urls, fmt.Sprintf("http://%s/%s", listen.Addr().String(), content))
	}
	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for _, content := range inputs {
				if string(content) == path.Base(r.URL.Path) {
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

func discardLogger() *archive.Logger {
	return &archive.Logger{
		Stdout:  log.New(ioutil.Discard, "", 0),
		Verbose: log.New(ioutil.Discard, "", 0),
		Stderr:  log.New(ioutil.Discard, "", 0),
	}
}

// MemStore is a in-memory implementation of Store to ease testing.
type MemStore struct {
	Data                   sync.Map
	GetErrorWith           error
	SearchErrorWith        error
	GetReturnsClosedReader bool
}

// NewMemStore returns a MemStore pre-filled with supplied fixtures.
func NewMemStore(fixtures file.List) *MemStore {
	store := &MemStore{
		Data: sync.Map{},
	}
	for _, fixture := range fixtures {
		store.Data.Store(fixture.Name, fixture)
	}
	return store
}

// String returns a human friendly representation of the MemStore.
func (s *MemStore) String() string {
	return fmt.Sprintf("MemStore")
}

// Put assigns the content of an io.Reader to a string keyed in-memory map using
// the hash as a key.
func (s *MemStore) Put(_ context.Context, reader io.Reader, name string, lastModified time.Time) error {
	data, err := ioutil.ReadAll(reader)
	if err != nil {
		return err
	}
	f := file.NewStub(name, int64(len(data)), lastModified)
	f.Body = bytes.NewReader(data)
	s.Data.Store(name, f)
	return nil
}

// Search finds matching items in storage by prefix.
func (s *MemStore) Search(_ context.Context, search string) (file.List, error) {
	if s.SearchErrorWith != nil {
		return nil, s.SearchErrorWith
	}
	var matches file.List
	s.Data.Range(func(key interface{}, value interface{}) bool {
		if strings.HasPrefix(key.(string), search) {
			matches = append(matches, value.(*file.File))
		}
		return true
	})
	sort.Sort(matches)
	return matches, nil
}

// Get finds an object in storage by name and returns an io.ReadCloser for it.
func (s *MemStore) Get(ctx context.Context, name string) (*file.File, error) {
	if s.GetErrorWith != nil {
		return nil, s.GetErrorWith
	}
	if data, ok := s.Data.Load(name); ok {
		return data.(*file.File), nil
	}
	return nil, fmt.Errorf("%w: not in store", os.ErrNotExist)
}

// Delete removes an object in archive.
func (s *MemStore) Delete(_ context.Context, request string) error {
	s.Data.Delete(request)
	return nil
}

// Concat an array of byte arrays ordered identically with the input files
// supplied. Note that this loads the entire dataset into memory.
func (s *MemStore) Concat(_ context.Context, _ int, files []string) ([][]byte, error) {
	sort.Strings(files)
	result := make([][]byte, len(files))
	for index, item := range files {
		if value, ok := s.Data.Load(item); ok {
			f := value.(*file.File)
			data, _ := ioutil.ReadAll(f)
			// make sure body of file can be read again.
			f.Body = ioutil.NopCloser(bytes.NewReader(data))
			result[index] = data
		} else {
			return nil, os.ErrNotExist
		}
	}
	return result, nil
}

// Exists determines if a requested object exists in the MemStore.
func (s *MemStore) Stat(_ context.Context, name string) (*file.File, error) {
	var result *file.File
	s.Data.Range(func(key interface{}, value interface{}) bool {
		if key.(string) == name {
			result = value.(*file.File)
			return false
		}
		return true
	})
	if result != nil {
		return result, nil
	}
	return nil, os.ErrNotExist
}

// Ensure MemStore satisfies same basic interactions as "real" stores.
func TestMemStore(t *testing.T) {
	test.StoreSuite(t, NewMemStore(file.List{}))
}
