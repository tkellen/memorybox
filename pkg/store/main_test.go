package store_test

import (
	"errors"
	"fmt"
	"github.com/google/go-cmp/cmp"
	"github.com/tkellen/memorybox/pkg/store"
	"net"
	"net/http"
	"path"
	"strings"
	"testing"
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

func TestNew(t *testing.T) {
	table := map[string]struct {
		config      map[string]string
		expected    string
		expectedErr error
	}{
		"localDisk": {
			config: map[string]string{
				"type": "localDisk",
				"path": "/",
			},
			expected: "LocalDiskStore: /",
		},
		"s3": {
			config: map[string]string{
				"type":   "s3",
				"bucket": "bucket",
			},
			expected: "ObjectStore: bucket",
		},
		"testing": {
			config: map[string]string{
				"type": "testing",
			},
			expected: "TestingStore",
		},
		"unknown": {
			config: map[string]string{
				"type": "NOPE",
			},
			expectedErr: errors.New("NOPE"),
		},
	}
	for name, test := range table {
		test := test
		t.Run(name, func(t *testing.T) {
			result, err := store.New(test.config)
			if err != nil && test.expectedErr == nil {
				t.Fatal(err)
			}
			if err != nil && test.expectedErr != nil && !strings.Contains(err.Error(), test.expectedErr.Error()) {
				t.Fatalf("expected err %s, got %s", test.expectedErr, err)
			}
			if err == nil {
				if diff := cmp.Diff(test.expected, result.String()); diff != "" {
					t.Fatal(diff)
				}
			}
		})
	}
}
