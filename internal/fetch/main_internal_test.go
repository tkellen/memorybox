package fetch

import (
	"bytes"
	"context"
	"errors"
	"github.com/mattetti/filebuffer"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"testing"
)

func Test_fetch(t *testing.T) {
	expectedBytes := []byte("test")
	table := map[string]struct {
		input       string
		sys         *sys
		expectedErr error
	}{
		"success from stdin": {
			input: "-",
			sys: func() *sys {
				sys := new(context.Background())
				sys.Stdin = ioutil.NopCloser(bytes.NewReader(expectedBytes))
				return sys
			}(),
			expectedErr: nil,
		},
		"success from url": {
			input: "http://totally.legit",
			sys: func() *sys {
				sys := new(context.Background())
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
				return sys
			}(),
			expectedErr: nil,
		},
		"fail on inability to make http request for input url": {
			input: "http://that.is.not.a.valid.url",
			sys: func() *sys {
				sys := new(context.Background())
				sys.Get = http.Get
				return sys
			}(),
			expectedErr: errBadRequest,
		},
		"fail on non-200 http response from url input": {
			input: "http://totally.legit",
			sys: func() *sys {
				sys := new(context.Background())
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
				return sys
			}(),
			expectedErr: errBadRequest,
		},
		"fail on inability to buffer streaming input to disk": {
			input: "-",
			sys: func() *sys {
				sys := new(context.Background())
				sys.TempDir = path.Join("tmp", "nope", "bad")
				return sys
			}(),
			expectedErr: os.ErrNotExist,
		},
		"fail on inability to copy data to temp file": {
			input: "-",
			sys: func() *sys {
				file := filebuffer.New([]byte("test"))
				file.Close()
				sys := new(context.Background())
				sys.Stdin = file
				return sys
			}(),
			expectedErr: os.ErrClosed,
		},
	}
	for name, test := range table {
		test := test
		t.Run(name, func(t *testing.T) {
			file, deleteWhenDone, err := test.sys.fetch(test.input)
			if deleteWhenDone {
				defer os.Remove(file.Name())
			}
			if err != nil && test.expectedErr == nil {
				t.Fatal(err)
			}
			if err != nil && test.expectedErr != nil && !errors.Is(err, test.expectedErr) {
				t.Fatalf("expected error: %s, got %s", test.expectedErr, err)
			}
			if err == nil {
				defer file.Close()
				actualBytes, readErr := ioutil.ReadAll(file)
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
