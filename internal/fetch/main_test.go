package fetch_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/tkellen/memorybox/internal/fetch"
	"io"
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

func TestOne(t *testing.T) {
	expectedBytes := []byte("test")
	url, shutdownServer := fixtureServer(t, expectedBytes)
	defer shutdownServer()
	table := map[string]struct {
		context       context.Context
		input         string
		expectedErr   error
		expectedBytes []byte
	}{
		"success": {
			context:       context.Background(),
			input:         url,
			expectedBytes: expectedBytes,
		},
		"failure on cancelled context": {
			context: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			}(),
			input:       url,
			expectedErr: context.Canceled,
		},
		"failure on missing file": {
			context:     context.Background(),
			input:       "/nope/nope/nope/nope/nope",
			expectedErr: os.ErrNotExist,
		},
	}
	for name, test := range table {
		test := test
		t.Run(name, func(t *testing.T) {
			err := fetch.One(test.context, test.input, func(request string, src io.ReadSeeker) error {
				actualBytes, readErr := ioutil.ReadAll(src)
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

func TestMany(t *testing.T) {
	expectedBytes := []byte("test")
	url, shutdownServer := fixtureServer(t, expectedBytes)
	defer shutdownServer()
	table := map[string]struct {
		context       context.Context
		input         string
		expectedErr   error
		expectedBytes []byte
	}{
		"success": {
			context:       context.Background(),
			input:         url,
			expectedBytes: expectedBytes,
		},
		"failure on cancelled context": {
			context: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			}(),
			input:       url,
			expectedErr: context.Canceled,
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
			err := fetch.Many(test.context, []string{test.input, test.input, test.input, test.input}, 2, func(index int, request string, src io.ReadSeeker) error {
				actualBytes, readErr := ioutil.ReadAll(src)
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
