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
	cmd.read = func(input string, _ io.ReadCloser, _ string) (io.ReadCloser, string, error) {
		return ioutil.NopCloser(strings.NewReader(input)), input, nil
	}
	_, writer, _ := os.Pipe()
	cmd.stdout = writer
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

func TestPutFailureReadingInput(t *testing.T) {
	cmd := newTestCommand("put", map[string][]byte{})
	cmd.Request = []string{"one"}
	readCount := 0
	cmd.read = func(input string, _ io.ReadCloser, _ string) (io.ReadCloser, string, error) {
		// This method is called twice during a put, once to open the input for
		// hashing, and a second time to open the input for streaming to the
		// store. We'll use this method three times to catch both failures. Here
		// is what is being simulated for each call:
		// 1. First call failing simulates a failure trying to read the file
		//    that is being put. This ends the first test.
		// 2. Second call succeeding simulates a successful first read.
		// 3. Third call failing simulates a failure to read the file a second
		//    time for streaming to the store. This ends the second test.
		readCount++
		if readCount == 1 || readCount == 3 {
			return nil, "", fmt.Errorf("bad %d", readCount)
		}
		return ioutil.NopCloser(strings.NewReader(input)), input, nil
	}
	actualFirstRead := cmd.Dispatch()
	if actualFirstRead == nil || !strings.Contains(actualFirstRead.Error(), "bad 1") {
		t.Fatalf("expected error %s, got: %s", "bad 1", actualFirstRead)
	}

	actualSecondRead := cmd.Dispatch()
	if actualSecondRead == nil || !strings.Contains(actualSecondRead.Error(), "bad 3") {
		t.Fatalf("expected error %s, got: %s", "bad 3", actualSecondRead)
	}
}

func TestPutFailureBadReaders(t *testing.T) {
	cmd := newTestCommand("put", map[string][]byte{})
	cmd.Request = []string{"somedata"}
	readCount := 0
	cmd.read = func(input string, _ io.ReadCloser, _ string) (io.ReadCloser, string, error) {
		// This method is called twice during a put, once to open the input for
		// hashing, and a second time to open the input for streaming to the
		// store. We'll use this method three times to catch both failures. Here
		// is what is being simulated for each call:
		// 1. First call returning a bad reader simulates trying to put a remote
		//    url and the request timing out. This will end the first test when
		//    hashing the response fails.
		// 2. Second call returning a good reader will simulate a successful
		//    remote request where the data is persisted to a temporary file as
		//    it is hashed.
		// 3. Third call returning a bad reader will simulate a bad read on the
		//    temporary file created in step 2. This will end the second test
		//    case.
		readCount++
		if readCount == 1 || readCount == 3 {
			return ioutil.NopCloser(
				iotest.TimeoutReader(strings.NewReader(input)),
			), input, nil
		}
		return ioutil.NopCloser(strings.NewReader(input)), input, nil
	}
	hashFailure := cmd.Dispatch()
	if hashFailure == nil {
		t.Fatal("expected error failing to read input for hashing")
	}
	if !strings.Contains(hashFailure.Error(), "hashing: timeout") {
		t.Fatalf("expected timout error: %s", hashFailure)
	}

	putFailure := cmd.Dispatch()
	if putFailure == nil {
		t.Fatal("expected error failing to send input to store")
	}
	if !strings.Contains(putFailure.Error(), "timeout") {
		t.Fatalf("expected timout error: %s", putFailure)
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
	cmd.stdout = writer
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
	cmd.stdout = writer
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
