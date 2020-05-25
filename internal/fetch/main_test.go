package fetch_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/tkellen/memorybox/internal/fetch"
	"github.com/tkellen/memorybox/pkg/file"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"testing"
)

func fixtureServer(t *testing.T, expected []byte) (string, func() error) {
	listen, listenErr := net.Listen("tcp", "127.0.0.1:0")
	if listenErr != nil {
		t.Fatal(listenErr)
	}
	url := fmt.Sprintf("http://%s/%s", listen.Addr().String(), expected)
	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write(expected)
			return
		}),
	}
	go server.Serve(listen)
	return url, server.Close
}

func TestFetch(t *testing.T) {
	expectedBytes := []byte("test")
	url, shutdownServer := fixtureServer(t, expectedBytes)
	defer shutdownServer()
	tempFile, _ := ioutil.TempFile("", "")
	tempFile.Write(expectedBytes)
	defer os.Remove(tempFile.Name())
	table := map[string]struct {
		context       context.Context
		input         string
		expectedErr   error
		expectedBytes []byte
	}{
		"success from url": {
			context:       context.Background(),
			input:         url,
			expectedBytes: expectedBytes,
		},
		"success from local file": {
			context:       context.Background(),
			input:         tempFile.Name(),
			expectedBytes: expectedBytes,
		},
		"fail on invalid file": {
			context:     context.Background(),
			input:       "/nope/nope/nope/nope/nope",
			expectedErr: os.ErrNotExist,
		},
	}
	for name, test := range table {
		test := test
		t.Run(name, func(t *testing.T) {
			err := fetch.Do(context.Background(), []string{test.input, test.input, test.input, test.input}, 2, func(innerCtx context.Context, index int, src *file.File) error {
				actualBytes, readErr := ioutil.ReadAll(src.Body)
				if readErr != nil {
					t.Fatal(readErr)
				}
				if !bytes.Equal(test.expectedBytes, actualBytes) {
					t.Fatalf("expected bytes %s, got %s", test.expectedBytes, actualBytes)
				}
				return nil
			})
			if err != nil && test.expectedErr == nil {
				t.Fatal(err)
			}
			if err != nil && test.expectedErr != nil && !errors.Is(err, test.expectedErr) {
				t.Fatalf("expected error: %s, got %s", test.expectedErr, err)
			}
		})
	}
}
