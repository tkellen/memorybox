// These are unit tests that validate how func (cmd *Command) Dispatch() behaves
// when called with configurations that are supported by the cli.
package cli

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/tkellen/memorybox/pkg/test"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"testing"
	"testing/iotest"
)

func newTestCommand(action string, seed map[string][]byte) *Command {
	cmd := New()
	cmd.Action = action
	cmd.Concurrency = 1
	// in-memory map whose keys exactly match the value of what is stored in
	// them (aka, still content addressable, but not hashed).
	cmd.Store = &test.Store{
		Data: seed,
	}
	// make a dummy index that is just an array of keys in the store
	keys := make([]string, 0, len(seed))
	for k := range seed {
		keys = append(keys, k)
	}
	// just holds an array of keys
	cmd.Index = &test.Index{
		Data: keys,
	}
	// return an io.ReadCloser that contains the contents of the input string
	cmd.source = func(input string, _ string) (io.ReadCloser, string, error) {
		return ioutil.NopCloser(strings.NewReader(input)), input, nil
	}
	_, writer, _ := os.Pipe()
	cmd.sink = writer
	cmd.cleanup = func(_ string) error {
		return nil
	}
	return cmd
}

func TestRealLogging(t *testing.T) {
	logged := ""
	cmd := newTestCommand("test", map[string][]byte{})
	cmd.Logger = func(f string, a ...interface{}) {
		logged = fmt.Sprintf(f, a...)
	}
	cmd.Dispatch()
	if logged == "" {
		t.Fatal("expected logger to be called")
	}
}

func TestUnrecognizedCommand(t *testing.T) {
	action := "wat"
	cmd := newTestCommand(action, map[string][]byte{})
	expected := fmt.Sprintf("unknown action: %s", action)
	actual := cmd.Dispatch()
	if actual == nil || !strings.Contains(actual.Error(), expected) {
		t.Fatalf("expected error %s, got: %s", expected, actual)
	}
}

func TestPutOneSuccess(t *testing.T) {
	input := "one"
	cmd := newTestCommand("put", map[string][]byte{})
	cmd.Request = []string{input}
	if err := cmd.Dispatch(); err != nil {
		t.Fatalf("did not expect: %s", err)
	}
}

func TestPutManySuccess(t *testing.T) {
	cmd := newTestCommand("put", map[string][]byte{})
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
	cmd := newTestCommand("put", map[string][]byte{})
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
	cmd := newTestCommand("put", map[string][]byte{})
	cmd.Request = []string{"one"}
	cmd.source = func(input string, _ string) (io.ReadCloser, string, error) {
		return nil, "", expected
	}
	actual := cmd.Dispatch()
	if actual == nil || !strings.Contains(actual.Error(), expected.Error()) {
		t.Fatalf("expected error %s, got: %s", expected, actual)
	}
}

func TestPutFailureReadingFileToPersist(t *testing.T) {
	cmd := newTestCommand("put", map[string][]byte{})
	cmd.Request = []string{"one"}
	cmd.source = func(input string, _ string) (io.ReadCloser, string, error) {
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
	cmd := newTestCommand("put", map[string][]byte{})
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
	expected := []byte("test")
	reader, writer, _ := os.Pipe()
	cmd := newTestCommand("get", map[string][]byte{
		"test": expected,
	})
	cmd.Request = []string{"test"}
	cmd.sink = writer
	if err := cmd.Dispatch(); err != nil {
		t.Fatalf("did not expect: %s", err)
	}
	actual, err := ioutil.ReadAll(reader)
	if err != nil {
		t.Fatalf("failed to read output: %s", err)
	}
	if !bytes.Equal(expected, actual) {
		t.Fatalf("expected %s, got: %s", expected, actual)
	}
}

func TestGetMissingFailure(t *testing.T) {
	expected := errors.New("0 matches")
	cmd := newTestCommand("get", map[string][]byte{})
	cmd.Request = []string{"test"}
	actual := cmd.Dispatch()
	if actual == nil || !strings.Contains(actual.Error(), expected.Error()) {
		t.Fatalf("expected error %s, got: %s", expected, actual)
	}
}

func TestGetMissingInvalidIndexFailure(t *testing.T) {
	expected := errors.New("not found")
	cmd := newTestCommand("get", map[string][]byte{})
	cmd.Request = []string{"test"}
	cmd.Index = &test.Index{
		Data: []string{"test"},
	}
	actual := cmd.Dispatch()
	if actual == nil || !strings.Contains(actual.Error(), expected.Error()) {
		t.Fatalf("expected error %s, got: %s", expected, actual)
	}
}

func TestGetMultipleMatchFailure(t *testing.T) {
	expected := errors.New("2 matches")
	cmd := newTestCommand("get", map[string][]byte{
		"test-one": []byte("test-one"),
		"test-two": []byte("test-two"),
	})
	cmd.Request = []string{"test"}
	actual := cmd.Dispatch()
	if actual == nil || !strings.Contains(actual.Error(), expected.Error()) {
		t.Fatalf("expected error %s, got: %s", expected, actual)
	}
}

func TestGetStoreSearchFailure(t *testing.T) {
	expected := errors.New("failed")
	cmd := newTestCommand("get", map[string][]byte{
		"test": []byte("test"),
	})
	cmd.Index = &test.Index{
		ForceSearchError: expected,
	}
	cmd.Request = []string{"test"}
	actual := cmd.Dispatch()
	if actual == nil || !strings.Contains(actual.Error(), expected.Error()) {
		t.Fatalf("expected error %s, got: %s", expected, actual)
	}
}

func TestGetWriterFailure(t *testing.T) {
	_, writer, _ := os.Pipe()
	writer.Close()
	cmd := newTestCommand("get", map[string][]byte{
		"test": []byte("test"),
	})
	cmd.Request = []string{"test"}
	cmd.sink = writer
	actual := cmd.Dispatch()
	if actual == nil {
		t.Fatalf("expected error writing")
	}
}

func TestGetStoreFailure(t *testing.T) {
	expected := errors.New("failed")
	cmd := newTestCommand("get", map[string][]byte{
		"test": []byte("test"),
	})
	cmd.Store.(*test.Store).ForceGetError = expected
	cmd.Request = []string{"test"}
	actual := cmd.Dispatch()
	if actual == nil || !strings.Contains(actual.Error(), expected.Error()) {
		t.Fatalf("expected error %s, got: %s", expected, actual)
	}
}
