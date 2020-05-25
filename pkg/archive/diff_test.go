package archive_test

import (
	"context"
	"errors"
	"github.com/tkellen/memorybox/pkg/archive"
	"github.com/tkellen/memorybox/pkg/localdiskstore"
	"testing"
)

func TestDiff(t *testing.T) {
	type testCase struct {
		source      *localdiskstore.Store
		dest        *localdiskstore.Store
		expectedErr error
	}
	table := map[string]testCase{
		"perfect sync": {
			source:      localdiskstore.New("../../testdata/valid"),
			dest:        localdiskstore.New("../../testdata/valid"),
			expectedErr: nil,
		},
		"diff between stores": {
			source:      localdiskstore.New("../../testdata/valid"),
			dest:        localdiskstore.New("../../testdata/valid-alternate"),
			expectedErr: errors.New("[localDisk: ../../testdata/valid-alternate]: 9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08-sha256 [missing in localDisk: ../../testdata/valid]\n[localDisk: ../../testdata/valid-alternate]: memorybox-meta-9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08-sha256 [missing in localDisk: ../../testdata/valid]"),
		},
	}
	for name, test := range table {
		test := test
		t.Run(name, func(t *testing.T) {
			err := archive.Diff(context.Background(), test.source, test.dest)
			if err != nil && test.expectedErr == nil {
				t.Fatalf("expected no error, got %s", err)
			}
			if err == nil && test.expectedErr != nil {
				t.Fatal("expected error, got none")
			}
			if err != nil && test.expectedErr != nil && err.Error() != test.expectedErr.Error() {
				t.Fatalf("%s", err)
			}
		})
	}
}
