// These are unit tests that validate how func (cmd *Command) Dispatch() behaves
// when called with configurations that are supported by the cli.
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

func newTestCommand(action string) *Command {
	cmd := New()
	cmd.Action = action
	cmd.Concurrency = 1
	// in-memory map whose keys exactly match the value of what is stored in
	// them (aka, still content addressable, but not hashed).
	cmd.Store = &testStore{
		Data: map[string][]byte{},
	}
	cmd.reader = func(input string, _ string) (io.ReadCloser, string, error) {
		return ioutil.NopCloser(strings.NewReader(input)), input, nil
	}
	cmd.writer = func(input io.ReadCloser) error {
		input.Close()
		return nil
	}
	cmd.cleanup = func(_ string) error {
		return nil
	}
	return cmd
}

func TestUnrecognizedCommand(t *testing.T) {
	cmd := newTestCommand("wat")
	expected := "unrecognized command"
	actual := cmd.Dispatch()
	if actual == nil || !strings.Contains(actual.Error(), expected) {
		t.Fatalf("expected error %s, got: %s", expected, actual)
	}
}

func TestPutOneSuccess(t *testing.T) {
	input := "one"
	cmd := newTestCommand("put")
	cmd.Request = []string{input}
	if err := cmd.Dispatch(); err != nil {
		t.Fatalf("did not expect: %s", err)
	}
}

func TestPutManySuccess(t *testing.T) {
	cmd := newTestCommand("put")
	cmd.Request = []string{"one", "two"}
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
	cmd.Request = []string{"one"}
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
	cmd.Request = []string{"one"}
	cmd.reader = func(input string, _ string) (io.ReadCloser, string, error) {
		return nil, "", expected
	}
	actual := cmd.Dispatch()
	if actual == nil || !strings.Contains(actual.Error(), expected.Error()) {
		t.Fatalf("expected error %s, got: %s", expected, actual)
	}
}

func TestPutFailureReadingFileToPersist(t *testing.T) {
	cmd := newTestCommand("put")
	cmd.Request = []string{"one"}
	cmd.reader = func(input string, _ string) (io.ReadCloser, string, error) {
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
	cmd.Request = []string{"one"}
	cmd.cleanup = func(_ string) error {
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
	cmd.Request = []string{"test"}
	cmd.Store = &testStore{
		Data: map[string][]byte{
			"test": expected,
		},
	}
	cmd.writer = func(data io.ReadCloser) error {
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
	cmd.Request = []string{"test"}
	actual := cmd.Dispatch()
	if actual == nil || !strings.Contains(actual.Error(), expected.Error()) {
		t.Fatalf("expected error %s, got: %s", expected, actual)
	}
}

func TestGetMultipleMatchFailure(t *testing.T) {
	expected := errors.New("2 matches")
	cmd := newTestCommand("get")
	cmd.Request = []string{"test"}
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
	expected := errors.New("failed")
	cmd := newTestCommand("get")
	cmd.Store = &testStore{
		Data: map[string][]byte{
			"test": []byte("test"),
		},
		ForceSearchError: expected,
	}
	cmd.Request = []string{"test"}
	actual := cmd.Dispatch()
	if actual == nil || !strings.Contains(actual.Error(), expected.Error()) {
		t.Fatalf("expected error %s, got: %s", expected, actual)
	}
}

func TestGetWriterFailure(t *testing.T) {
	expected := errors.New("write error")
	cmd := newTestCommand("get")
	cmd.Request = []string{"test"}
	cmd.Store = &testStore{
		Data: map[string][]byte{
			"test": []byte("test"),
		},
	}
	cmd.writer = func(_ io.ReadCloser) error {
		return expected
	}
	actual := cmd.Dispatch()
	if actual == nil || !strings.Contains(actual.Error(), expected.Error()) {
		t.Fatalf("expected error %s, got: %s", expected, actual)
	}
}

func TestGetStoreFailure(t *testing.T) {
	expected := errors.New("failed")
	cmd := newTestCommand("get")
	cmd.Store = &testStore{
		Data: map[string][]byte{
			"test": []byte("test"),
		},
		ForceGetError: expected,
	}
	cmd.Request = []string{"test"}
	actual := cmd.Dispatch()
	if actual == nil || !strings.Contains(actual.Error(), expected.Error()) {
		t.Fatalf("expected error %s, got: %s", expected, actual)
	}
}

// better than using a mocking library? ¯\_(ツ)_/¯
type testStore struct {
	Data             map[string][]byte
	ForceSearchError error
	ForceGetError    error
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
	if s.ForceSearchError != nil {
		return nil, s.ForceSearchError
	}
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
	if s.ForceGetError != nil {
		return nil, s.ForceGetError
	}
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
