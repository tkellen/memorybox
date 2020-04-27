package archive

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"strings"
	"testing"
)

// identityHash is a noop hashing function for testing that returns a string
// value of the input (assumes ASCII input).
func identityHash(source io.Reader) (string, int64, error) {
	bytes, err := ioutil.ReadAll(source)
	if err != nil {
		return "", 0, err
	}
	return string(bytes) + "-identity", int64(len(bytes)), nil
}

func TestFile_init(t *testing.T) {
	expectedBytes := []byte("test")
	tempFile, tempFileErr := ioutil.TempFile(os.TempDir(), "*")
	if tempFileErr != nil {
		t.Fatalf("test setup: %s", tempFileErr)
	}
	defer os.Remove(tempFile.Name())
	table := map[string]struct {
		input       interface{}
		setup       func(sys *sys)
		expectedErr error
	}{
		"success from stdin": {
			input: "-",
			setup: func(sys *sys) {
				sys.Stdin = ioutil.NopCloser(bytes.NewReader(expectedBytes))
			},
			expectedErr: nil,
		},
		"success from url": {
			input: "http://totally.legit",
			setup: func(sys *sys) {
				// Mock every http request to contain our expected bytes.
				sys.Get = func(url string) (resp *http.Response, err error) {
					return &http.Response{
						Status:        "OK",
						StatusCode:    200,
						Proto:         "HTTP/1.1",
						ProtoMajor:    1,
						ProtoMinor:    1,
						Body:          ioutil.NopCloser(bytes.NewReader(expectedBytes)),
						ContentLength: int64(len(expectedBytes)),
					}, nil
				}
			},
			expectedErr: nil,
		},
		"fail on inability to make http request for input url": {
			input: "http://that.is.not.a.valid.url",
			setup: func(sys *sys) {
				sys.Get = http.Get
			},
			expectedErr: errors.New("no such host"),
		},
		"fail on non-200 http response from url input": {
			input: "http://totally.legit",
			setup: func(sys *sys) {
				// Mock every http request to fail with a 400 error code.
				sys.Get = func(url string) (resp *http.Response, err error) {
					return &http.Response{
						Status:        "BadRequest",
						StatusCode:    400,
						Proto:         "HTTP/1.1",
						ProtoMajor:    1,
						ProtoMinor:    1,
						Body:          ioutil.NopCloser(bytes.NewReader(expectedBytes)),
						ContentLength: int64(len(expectedBytes)),
					}, nil
				}
			},
			expectedErr: errors.New("http code: 400"),
		},
		"fail on inability to create temporary directory": {
			input: "-",
			setup: func(sys *sys) {
				sys.TempDirBase = path.Join("tmp", "nope", "bad")
			},
			expectedErr: os.ErrNotExist,
		},
		"fail on inability to buffer streaming input to disk": {
			input: "-",
			setup: func(sys *sys) {
				sys.TempDir = path.Join("tmp", "nope", "bad")
			},
			expectedErr: os.ErrNotExist,
		},
		"fail on inability to open file to check if it is a metafile": {
			input: ioutil.NopCloser(bytes.NewBuffer(expectedBytes)),
			setup: func(sys *sys) {
				sys.ReadFile = func(name string) ([]byte, error) {
					return nil, errors.New("bad time")
				}
			},
			expectedErr: errors.New("detecting metafile"),
		},
	}
	for name, test := range table {
		test := test
		t.Run(name, func(t *testing.T) {
			f := new(identityHash)
			test.setup(f.sys)
			f, err := f.init(test.input)
			if err != nil && test.expectedErr == nil {
				t.Fatal(err)
			}
			if err != nil && test.expectedErr != nil && !errors.Is(err, test.expectedErr) && !strings.Contains(err.Error(), test.expectedErr.Error()) {
				t.Fatalf("expected error: %s, got %s", test.expectedErr, err)
			}
			if err == nil {
				defer f.Close()
				actualBytes, readErr := ioutil.ReadAll(f)
				if readErr != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(expectedBytes, actualBytes) {
					t.Fatalf("expected bytes %s, got %s", expectedBytes, actualBytes)
				}
			}
		})
	}
}

func TestFile_Close(t *testing.T) {
	f := new(identityHash)
	// a temp directory that cannot be cleaned up by os.RemoveAll
	f.sys.TempDir = "/tmp/."
	if err := f.Close(); err == nil {
		t.Fatal("expected error removing temporary directory")
	}
}
