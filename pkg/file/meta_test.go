package file_test

import (
	"encoding/json"
	"fmt"
	"github.com/google/go-cmp/cmp"
	"github.com/mattetti/filebuffer"
	"github.com/tkellen/memorybox/pkg/file"
	"reflect"
	"testing"
	"time"
)

func TestMetaNameFrom(t *testing.T) {
	table := map[string]struct {
		input    string
		expected string
	}{
		"filenames become metaFile names": {
			input:    "test",
			expected: file.MetaFilePrefix + "test",
		},
		"metaFile names remain the same": {
			input:    file.MetaFilePrefix + "test",
			expected: file.MetaFilePrefix + "test",
		},
	}
	for name, test := range table {
		test := test
		t.Run(name, func(t *testing.T) {
			actual := file.MetaNameFrom(test.input)
			if diff := cmp.Diff(test.expected, actual); diff != "" {
				t.Fatal(diff)
			}
		})
	}
}

func TestIsMetaFileName(t *testing.T) {
	table := map[string]struct {
		input    string
		expected bool
	}{
		"metaFile names return true": {
			input:    file.MetaFilePrefix + "test",
			expected: true,
		},
		"other things return false": {
			input:    "test",
			expected: false,
		},
	}
	for name, test := range table {
		test := test
		t.Run(name, func(t *testing.T) {
			actual := file.IsMetaFileName(test.input)
			if diff := cmp.Diff(test.expected, actual); diff != "" {
				t.Fatal(diff)
			}
		})
	}
}

func TestValidateMeta(t *testing.T) {
	largeJsonContent := []byte(fmt.Sprintf(`{"memorybox":{"name":"test"},"data":"%s"}`, make([]byte, file.MetaFileMaxSize, file.MetaFileMaxSize)))
	table := map[string]struct {
		input       []byte
		expectedErr bool
	}{
		"not being valid json means it is not metadata": {
			input:       []byte(`[:not-metadata:]`),
			expectedErr: true,
		},
		"being valid json is not enough to be considered metadata": {
			input:       []byte(`{}`),
			expectedErr: true,
		},
		"valid json with a metadata key is metadata": {
			input:       []byte(`{"memorybox":{}}`),
			expectedErr: false,
		},
		"files larger than file.MetaFileMaxSize are not valid metadata": {
			input:       largeJsonContent,
			expectedErr: true,
		},
	}
	for name, test := range table {
		test := test
		t.Run(name, func(t *testing.T) {
			err := file.ValidateMeta(test.input)
			if test.expectedErr && err == nil {
				t.Fatalf("did not expect error, got %v", err)
			}
		})
	}
}

func TestMeta_DataFileName(t *testing.T) {
	f := &file.File{
		Name:         "test",
		Source:       "test",
		Size:         4,
		LastModified: time.Now(),
	}
	table := map[string]struct {
		meta     *file.Meta
		expected string
	}{
		"dataFile names are found in valid metadata": {
			meta:     file.NewMetaFromFile(f),
			expected: f.Name,
		},
		"empty string is returned for invalid input": {
			meta:     (*file.Meta)(&[]byte{'['}),
			expected: "",
		},
	}
	for name, test := range table {
		test := test
		t.Run(name, func(t *testing.T) {
			actual := test.meta.DataFileName()
			if diff := cmp.Diff(test.expected, actual); diff != "" {
				t.Fatal(diff)
			}
		})
	}
}

func TestMeta_Source(t *testing.T) {
	f, err := file.NewSha256("test", filebuffer.New([]byte("test")), time.Now())
	if err != nil {
		t.Fatalf("test setup: %s", err)
	}
	if f.Source != f.Meta.Source() {
		t.Fatalf("expected source to be %s, got %s", f.Source, f.Meta.Source())
	}
}

func TestMeta_SetGetDelete(t *testing.T) {
	type testCase struct {
		key      string
		input    string
		expected interface{}
	}
	table := map[string]testCase{
		"empty key returns nil": {
			key:      "",
			input:    "value",
			expected: nil,
		},
		"string values can be set and retrieved": {
			key:      "test",
			input:    "value",
			expected: "value",
		},
		"string values containing integers are cast as such": {
			key:      "test",
			input:    "100",
			expected: json.RawMessage("100"),
		},
		"string values containing floating point numbers are cast as such": {
			key:      "test",
			input:    "100.1",
			expected: json.RawMessage("100.1"),
		},
		"string value true is cast as json boolean": {
			key:      "test",
			input:    "true",
			expected: json.RawMessage("true"),
		},
		"string value false is cast as json boolean": {
			key:      "test",
			input:    "false",
			expected: json.RawMessage("false"),
		},
		"valid json encoded strings are stored as json.RawMessages": {
			key:      "test",
			input:    `{"test":"value"}`,
			expected: json.RawMessage(`{"test":"value"}`),
		},
		"invalid json encoded strings are stored as plain strings": {
			key:      "test",
			input:    `{"test":"value}`,
			expected: `{"test":"value}`,
		},
	}
	for name, test := range table {
		test := test
		t.Run(name, func(t *testing.T) {
			meta := file.Meta(`{"memorybox":{"file":"test-identity"}}`)
			meta.Set(test.key, test.input)
			actualAfterSet := meta.Get(test.key)
			if diff := cmp.Diff(test.expected, actualAfterSet); diff != "" {
				t.Fatalf(diff)
			}
			meta.Delete(test.key)
			actualAfterDelete := meta.Get(test.key)
			if actualAfterDelete != nil {
				t.Fatal("expected delete to remove key")
			}
		})
	}
}

func TestMeta_Merge(t *testing.T) {
	type testCase struct {
		input       string
		expectedErr bool
		checks      map[string]interface{}
	}
	table := map[string]testCase{
		"object is merged into meta": {
			input:       `{"keyOne":"foo","keyBar":"baz"}`,
			expectedErr: false,
			checks: map[string]interface{}{
				"keyOne": "foo",
				"keyBar": "baz",
			},
		},
		"memorybox key is ignored": {
			input:       `{"memorybox":{},"keyOne":"foo","keyBar":"baz"}`,
			expectedErr: false,
			checks: map[string]interface{}{
				"memorybox": json.RawMessage("{\"file\":\"test-identity\"}"),
				"keyOne":    "foo",
				"keyBar":    "baz",
			},
		},
		"invalid json errors": {
			input:       `[ar":"baz"}`,
			expectedErr: true,
		},
	}
	for name, test := range table {
		test := test
		t.Run(name, func(t *testing.T) {
			meta := file.Meta(`{"memorybox":{"file":"test-identity"}}`)
			err := meta.Merge(test.input)
			if err != nil && !test.expectedErr {
				t.Fatalf("expected no error, saw %s", err)
			}
			if err == nil && test.expectedErr {
				t.Fatalf("expected error, got %s", meta)
			}
			if test.checks != nil {
				for key, value := range test.checks {
					if !reflect.DeepEqual(value, meta.Get(key)) {
						t.Fatalf("expected %s, got %s", value, meta.Get(key))
					}
				}
			}
		})
	}
}
