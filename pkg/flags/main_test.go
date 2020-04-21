package flags

import (
	"errors"
	"github.com/google/go-cmp/cmp"
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	table := map[string]struct {
		args        []string
		expected    Flags
		expectedErr error
	}{
		"good args": {
			args:        []string{"memorybox", "config"},
			expected:    Flags{Config: true, Input: []string{}, Concurrency: 10},
			expectedErr: nil,
		},
		"bad args": {
			args:        []string{"memorybox"},
			expected:    Flags{},
			expectedErr: errors.New("--help"),
		},
	}
	for name, test := range table {
		t.Run(name, func(t *testing.T) {
			flags, err := New(test.args, "test")
			if test.expectedErr == nil && err != nil {
				t.Fatalf("did not expect error: %s", err)
			}
			if err != nil && test.expectedErr != nil && !strings.Contains(err.Error(), test.expectedErr.Error()) {
				t.Fatalf("expected error: %s, got %s", test.expectedErr, err)
			}
			if diff := cmp.Diff(flags, test.expected); diff != "" {
				t.Fatalf(diff)
			}
		})
	}
}

func TestMethod(t *testing.T) {
	table := map[string]Flags{
		"PutMain":        {Put: true},
		"GetMain":        {Get: true},
		"ConfigMain":     {Config: true},
		"ConfigSet":      {Config: true, Set: true},
		"ConfigDelete":   {Config: true, Delete: true},
		"MetaMain":       {Meta: true},
		"MetaSet":        {Meta: true, Set: true},
		"MetaDelete":     {Meta: true, Delete: true},
		"NotImplemented": Flags{},
	}
	for name, test := range table {
		t.Run(name, func(t *testing.T) {
			actual := test.Method()
			if name != actual {
				t.Fatalf("expected %s, got %s", name, actual)
			}
		})
	}
}
