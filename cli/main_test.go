// These are unit tests that validate how func (cmd *Command) Dispatch() behaves
// when called with configurations that are supported by the cli. The underlying
// memorybox.Store is a in-memory map whose keys exactly match the value of what
// is stored in them (aka, still content addressable, but not hashed).
package cli

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"strings"
	"testing"
	"testing/iotest"
)

func TestPutOneSuccess(t *testing.T) {
	input := "one"
	cmd := newTestCommand("put")
	cmd.Inputs = []string{input}
	if err := cmd.Dispatch(); err != nil {
		t.Fatalf("did not expect: %s", err)
	}
}

func TestPutManySuccess(t *testing.T) {
	cmd := newTestCommand("put")
	cmd.Inputs = []string{"one", "two"}
	if err := cmd.Dispatch(); err != nil {
		t.Fatalf("did not expect: %s", err)
	}
}

func TestPutConcurrencyLimiting(t *testing.T) {
	t.Log("TODO")
}

func TestPutExistsSuccess(t *testing.T) {
	var logSkipped bool
	cmd := newTestCommand("put")
	cmd.Inputs = []string{"one"}
	cmd.Logger = func(message string, args ...interface{}) {
		if strings.Contains(message, "skipped") {
			logSkipped = true
		}
	}
	if err := cmd.Dispatch(); err != nil {
		t.Fatalf("putting new item: %s", err)
	}
	if err := cmd.Dispatch(); err != nil {
		t.Fatalf("putting same item twice: %s", err)
	}
	if logSkipped != true {
		t.Fatal("expected logger to be called with skipped message")
	}
}

func TestPutFailureReadingFileToHash(t *testing.T) {
	expected := errors.New("read error")
	cmd := newTestCommand("put")
	cmd.Inputs = []string{"one"}
	cmd.Reader = func(input string) (io.ReadCloser, string, error) {
		return nil, "", expected
	}
	actual := cmd.Dispatch()
	if actual == nil || !strings.Contains(actual.Error(), expected.Error()) {
		t.Fatalf("expected error %s, got: %s", expected, actual)
	}
}

func TestPutFailureReadingFileToPersist(t *testing.T) {
	cmd := newTestCommand("put")
	cmd.Inputs = []string{"one"}
	cmd.Reader = func(input string) (io.ReadCloser, string, error) {
		return ioutil.NopCloser(
			iotest.TimeoutReader(strings.NewReader(input)),
		), input, nil
	}
	err := cmd.Dispatch()
	if err == nil {
		t.Fatal("expected error failing to read input")
	}
	if !strings.Contains(err.Error(), "timeout") {
		t.Fatalf("expected timout error: %s", err)
	}
}

func TestPutCleanupFailure(t *testing.T) {
	expected := errors.New("cleanup error")
	cmd := newTestCommand("put")
	cmd.Inputs = []string{"one"}
	cmd.Cleanup = func() error {
		return expected
	}
	actual := cmd.Dispatch()
	if actual == nil || !strings.Contains(actual.Error(), expected.Error()) {
		t.Fatalf("expected error %s, got: %s", expected, actual)
	}
}

func TestGetSuccess(t *testing.T) {
	var actual []byte
	expected := []byte("test")
	cmd := newTestCommand("get")
	cmd.Request = "test"
	cmd.Store = &testStore{
		Data: map[string][]byte{
			"test": expected,
		},
	}
	cmd.Writer = func(data io.ReadCloser) error {
		actual, _ = ioutil.ReadAll(data)
		return nil
	}
	if err := cmd.Dispatch(); err != nil {
		t.Fatalf("did not expect: %s", err)
	}
	if actual == nil || !bytes.Equal(expected, actual) {
		t.Fatalf("expected %s, got: %s", expected, actual)
	}
}

func TestGetMissingFailure(t *testing.T) {
	expected := errors.New("0 matches")
	cmd := newTestCommand("get")
	cmd.Request = "test"
	actual := cmd.Dispatch()
	if actual == nil || !strings.Contains(actual.Error(), expected.Error()) {
		t.Fatalf("expected error %s, got: %s", expected, actual)
	}
}

func TestGetMultipleMatchFailure(t *testing.T) {
	expected := errors.New("2 matches")
	cmd := newTestCommand("get")
	cmd.Request = "test"
	cmd.Store = &testStore{
		Data: map[string][]byte{
			"test-one": []byte("test-one"),
			"test-two": []byte("test-two"),
		},
	}
	actual := cmd.Dispatch()
	if actual == nil || !strings.Contains(actual.Error(), expected.Error()) {
		t.Fatalf("expected error %s, got: %s", expected, actual)
	}
}

func TestGetStoreSearchFailure(t *testing.T) {
	t.Log("TODO: trigger a failure on cmd.Store.Search")
}

func TestGetWriterFailure(t *testing.T) {
	expected := errors.New("write error")
	cmd := newTestCommand("get")
	cmd.Request = "test"
	cmd.Store = &testStore{
		Data: map[string][]byte{
			"test": []byte("test"),
		},
	}
	cmd.Writer = func(_ io.ReadCloser) error {
		return expected
	}
	actual := cmd.Dispatch()
	if actual == nil || !strings.Contains(actual.Error(), expected.Error()) {
		t.Fatalf("expected error %s, got: %s", expected, actual)
	}
}

func TestGetStoreFailure(t *testing.T) {
	t.Log("TODO: trigger a failure on cmd.Store.Get")
}

// Start Test Helpers

func newTestCommand(action string) *Command {
	return &Command{
		Action: action,
		Store: &testStore{
			Data: map[string][]byte{},
		},
		Logger:      func(string, ...interface{}) {},
		Concurrency: 1,
		Reader:      testReader,
		Writer:      testWriter,
		Cleanup:     testCleanup,
	}
}

func testCleanup() error {
	return nil
}

func testReader(input string) (io.ReadCloser, string, error) {
	return ioutil.NopCloser(strings.NewReader(input)), input, nil
}

func testWriter(input io.ReadCloser) error {
	input.Close()
	return nil
}

type testStore struct {
	Data map[string][]byte
}

func (s *testStore) String() string {
	return fmt.Sprintf("testStore")
}

func (s *testStore) Put(src io.ReadCloser, hash string) error {
	data, err := ioutil.ReadAll(src)
	if err != nil {
		return err
	}
	s.Data[hash] = data
	return nil
}

func (s *testStore) Search(search string) ([]string, error) {
	var matches []string
	for key := range s.Data {
		if strings.HasPrefix(key, search) {
			matches = append(matches, key)
		}
	}
	return matches, nil
}

// Get finds an object in storage by name and returns an io.ReadCloser for it.
func (s *testStore) Get(request string) (io.ReadCloser, error) {
	data := s.Data[request]
	if data != nil {
		return ioutil.NopCloser(bytes.NewReader(data)), nil
	}
	return nil, fmt.Errorf("not found")
}

// Exists determines if a requested object exists in the Store.
func (s *testStore) Exists(request string) bool {
	return s.Data[request] != nil
}
