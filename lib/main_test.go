package memorybox_test

import (
	"errors"
	"github.com/google/go-cmp/cmp"
	"github.com/tkellen/memorybox/lib"
	"strings"
	"testing"
)

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
