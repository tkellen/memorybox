package config

import (
	"bytes"
	"errors"
	"fmt"
	"gopkg.in/yaml.v2"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"testing"
	"testing/iotest"
)

func TestConfig_Target(t *testing.T) {
	table := map[string]struct {
		config                 *Config
		expectedTargetUnderKey string
	}{
		"no targets initialized": {
			expectedTargetUnderKey: "default",
			config:                 &Config{},
		},
		"no target requested": {
			expectedTargetUnderKey: "default",
			config: &Config{
				File: File{Targets: map[string]Target{
					"default": {},
				}},
			},
		},
		"existing target requested": {
			expectedTargetUnderKey: "test",
			config: &Config{
				Flags: Flags{Target: "test"},
				File: File{Targets: map[string]Target{
					"test": {},
				}},
			},
		},
		"missing target creates target": {
			expectedTargetUnderKey: "missing",
			config: &Config{
				Flags: Flags{Target: "missing"},
				File:  File{Targets: map[string]Target{}},
			},
		},
	}
	for name, test := range table {
		t.Run(name, func(t *testing.T) {
			actual := test.config.Target()
			expected := test.config.File.Targets[test.expectedTargetUnderKey]
			if &expected == actual {
				t.Fatalf("expected target %s, got %s", &expected, actual)
			}
		})
	}
}

func TestConfig_Delete(t *testing.T) {
	table := map[string]struct {
		config              *Config
		targetToDelete      string
		expectedTargetCount int
	}{
		"delete existing target": {
			config: &Config{
				Flags: Flags{Target: "test"},
				File: File{Targets: map[string]Target{
					"test": {},
				}},
			},
			targetToDelete: "test",
		},
		"delete non-existing target": {
			config: &Config{
				Flags: Flags{Target: "test"},
				File: File{Targets: map[string]Target{
					"nope": {},
				}},
			},
			targetToDelete: "test",
		},
	}
	for name, test := range table {
		t.Run(name, func(t *testing.T) {
			test.config.Delete(test.targetToDelete)
			if _, ok := test.config.Targets[test.targetToDelete]; ok {
				t.Fatal("deleted target still present")
			}
		})
	}
}

func TestConfig_Load(t *testing.T) {
	goodInput := []byte("targets:\n  test:\n    home: ~/memorybox\n    type: localdisk\n")
	table := map[string]struct {
		input       io.Reader
		expected    []byte
		expectedErr error
	}{
		"load valid yaml": {
			input:       bytes.NewReader(goodInput),
			expected:    goodInput,
			expectedErr: nil,
		},
		"load invalid yaml": {
			input:       bytes.NewReader([]byte("notyaml")),
			expected:    []byte("targets: {}\n"),
			expectedErr: errors.New("cannot unmarshal"),
		},
		"load bad reader": {
			input:       iotest.TimeoutReader(bytes.NewReader([]byte("notyaml"))),
			expected:    []byte("targets: {}\n"),
			expectedErr: errors.New("timeout"),
		},
	}
	for name, test := range table {
		t.Run(name, func(t *testing.T) {
			config := Config{}
			err := config.Load(test.input)
			if test.expectedErr == nil && err != nil {
				t.Fatalf("did not expect error: %s", err)
			}
			if err != nil && test.expectedErr != nil && !strings.Contains(err.Error(), test.expectedErr.Error()) {
				t.Fatalf("expected error: %s, got %s", test.expectedErr, err)
			}
			actual, _ := yaml.Marshal(config.File)
			if !bytes.Equal(test.expected, actual) {
				t.Fatalf("load failed, expected %s, got %s", test.expected, actual)
			}
		})
	}
}

func TestConfig_Save(t *testing.T) {
	cfg := &Config{
		File: File{Targets: map[string]Target{
			"test": {
				"type": "localdisk",
				"home": "~/memorybox",
			},
		}},
	}
	badReadWriter, err := ioutil.TempFile("", "*")
	if err != nil {
		t.Fatalf("setting up test: %s", err)
	}
	defer os.RemoveAll(badReadWriter.Name())
	badReadWriter.Close()
	table := map[string]struct {
		config       *Config
		readerWriter io.ReadWriter
		expected     []byte
		expectedErr  error
	}{
		"success": {
			config:       cfg,
			readerWriter: bytes.NewBuffer([]byte{}),
			expected:     []byte("targets:\n  test:\n    home: ~/memorybox\n    type: localdisk\n"),
			expectedErr:  nil,
		},
		"failure": {
			config:       cfg,
			readerWriter: badReadWriter,
			expected:     nil,
			expectedErr:  errors.New("already closed"),
		},
	}
	for name, test := range table {
		t.Run(name, func(t *testing.T) {
			err := test.config.Save(test.readerWriter)
			if test.expectedErr == nil && err != nil {
				t.Fatalf("did not expect error: %s", err)
			}
			if err != nil && test.expectedErr != nil && !strings.Contains(err.Error(), test.expectedErr.Error()) {
				t.Fatalf("expected error: %s, got %s", test.expectedErr, err)
			}
			actual, _ := ioutil.ReadAll(test.readerWriter)
			if !bytes.Equal(test.expected, actual) {
				t.Fatalf("save failed, expected %s, got %s", test.expected, actual)
			}
		})
	}
}

func TestFile_String(t *testing.T) {
	expected := "targets:\n  test:\n    home: ~/memorybox\n    type: localdisk\n"
	actual := &File{
		Targets: map[string]Target{
			"test": {
				"type": "localdisk",
				"home": "~/memorybox",
			},
		},
	}
	if expected != fmt.Sprintf("%s", actual) {
		t.Fatalf("expected %s, got %s", expected, actual)
	}
}

func TestTarget_Set(t *testing.T) {
	target := &Target{}
	target.Set("key", "value")
	if len(*target) != 1 {
		t.Fatal("expected one item in target configuration")
	}
}

func TestTarget_Get(t *testing.T) {
	expected := "value"
	target := &Target{"key": expected}
	actual := target.Get("key")
	if expected != actual {
		t.Fatalf("expected %s, got %s", expected, actual)
	}
}
