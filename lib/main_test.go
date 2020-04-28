package memorybox_test

import (
	"bytes"
	"errors"
	"github.com/google/go-cmp/cmp"
	"github.com/tkellen/memorybox/lib"
	"strings"
	"testing"
	"testing/iotest"
)

func TestSha256(t *testing.T) {
	input := []byte("test")
	expected := "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08-sha256"
	expectedSize := int64(len(input))
	actual, actualSize, goodErr := memorybox.Sha256(bytes.NewReader(input))
	if goodErr != nil {
		t.Fatal(goodErr)
	}
	if expected != actual {
		t.Fatalf("expected %s, got %s", expected, actual)
	}
	if expectedSize != actualSize {
		t.Fatalf("expected size %d, got %d", expectedSize, actualSize)
	}
	_, _, err := memorybox.Sha256(iotest.TimeoutReader(bytes.NewReader([]byte("test"))))
	if err == nil {
		t.Fatal("expected error on bad reader")
	}
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
				"home": "/",
			},
			expected: "LocalDiskStore: /",
		},
		"s3": {
			config: map[string]string{
				"type": "s3",
				"home": "bucket",
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
			result, err := memorybox.NewStore(test.config)
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
