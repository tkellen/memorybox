package configfile

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

func TestConfigNew(t *testing.T) {
	table := map[string]struct {
		input       io.Reader
		expectedErr bool
	}{
		"bad config reader": {
			input:       ioutil.NopCloser(bytes.NewReader([]byte("invalidyaml"))),
			expectedErr: true,
		},
		"good config reader": {
			input:       ioutil.NopCloser(bytes.NewReader([]byte("targets:"))),
			expectedErr: false,
		},
	}
	for name, test := range table {
		t.Run(name, func(t *testing.T) {
			_, err := New(test.input)
			if test.expectedErr && err == nil {
				t.Fatal("expected error")
			}
			if !test.expectedErr && err != nil {
				t.Fatalf("did not expect error: %s", err)
			}
		})
	}

}

func TestConfigFile_String(t *testing.T) {
	expected := "targets:\n  test:\n    home: ~/memorybox\n    type: localDisk\n"
	actual := &ConfigFile{
		Targets: map[string]Target{
			"test": {
				"type": "localDisk",
				"home": "~/memorybox",
			},
		},
	}
	if expected != fmt.Sprintf("%s", actual) {
		t.Fatalf("expected %s, got %s", expected, actual)
	}
}

func TestConfigFile_Target(t *testing.T) {
	table := map[string]struct {
		configFile             *ConfigFile
		expectedTargetUnderKey string
	}{
		"no targets initialized": {
			expectedTargetUnderKey: "default",
			configFile:             &ConfigFile{},
		},
		"no target requested": {
			expectedTargetUnderKey: "default",
			configFile: &ConfigFile{
				Targets: map[string]Target{
					"default": {},
				},
			},
		},
		"existing target requested": {
			expectedTargetUnderKey: "test",
			configFile: &ConfigFile{
				Targets: map[string]Target{
					"test": {},
				},
			},
		},
		"missing target creates target": {
			expectedTargetUnderKey: "missing",
			configFile: &ConfigFile{
				Targets: map[string]Target{},
			},
		},
	}
	for name, test := range table {
		t.Run(name, func(t *testing.T) {
			actual := test.configFile.Target(test.expectedTargetUnderKey)
			expected := test.configFile.Targets[test.expectedTargetUnderKey]
			if &expected == actual {
				t.Fatalf("expected target %s, got %s", &expected, actual)
			}
		})
	}
}

func TestConfigFile_Delete(t *testing.T) {
	table := map[string]struct {
		configFile          *ConfigFile
		targetToDelete      string
		expectedTargetCount int
	}{
		"delete existing target": {
			configFile: &ConfigFile{
				Targets: map[string]Target{
					"test": {},
				},
			},
			targetToDelete: "test",
		},
		"delete non-existing target": {
			configFile: &ConfigFile{
				Targets: map[string]Target{
					"nope": {},
				},
			},
			targetToDelete: "test",
		},
	}
	for name, test := range table {
		t.Run(name, func(t *testing.T) {
			test.configFile.Delete(test.targetToDelete)
			if _, ok := test.configFile.Targets[test.targetToDelete]; ok {
				t.Fatal("deleted target still present")
			}
		})
	}
}

func TestConfigFile_Load(t *testing.T) {
	goodInput := []byte("targets:\n  test:\n    home: ~/memorybox\n    type: localDisk\n")
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
			configFile := ConfigFile{}
			err := configFile.Load(test.input)
			if test.expectedErr == nil && err != nil {
				t.Fatalf("did not expect error: %s", err)
			}
			if err != nil && test.expectedErr != nil && !strings.Contains(err.Error(), test.expectedErr.Error()) {
				t.Fatalf("expected error: %s, got %s", test.expectedErr, err)
			}
			actual, _ := yaml.Marshal(configFile)
			if !bytes.Equal(test.expected, actual) {
				t.Fatalf("load failed, expected %s, got %s", test.expected, actual)
			}
		})
	}
}

func TestConfigFile_Save(t *testing.T) {
	cfg := &ConfigFile{
		Targets: map[string]Target{
			"test": {
				"type": "localDisk",
				"home": "~/memorybox",
			},
		},
	}
	badReadWriter, err := ioutil.TempFile("", "*")
	if err != nil {
		t.Fatalf("setting up test: %s", err)
	}
	defer os.RemoveAll(badReadWriter.Name())
	badReadWriter.Close()
	table := map[string]struct {
		configFile   *ConfigFile
		readerWriter io.ReadWriter
		expected     []byte
		expectedErr  error
	}{
		"success": {
			configFile:   cfg,
			readerWriter: bytes.NewBuffer([]byte{}),
			expected:     []byte("targets:\n  test:\n    home: ~/memorybox\n    type: localDisk\n"),
			expectedErr:  nil,
		},
		"failure": {
			configFile:   cfg,
			readerWriter: badReadWriter,
			expected:     nil,
			expectedErr:  errors.New("already closed"),
		},
	}
	for name, test := range table {
		t.Run(name, func(t *testing.T) {
			err := test.configFile.Save(test.readerWriter)
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

func TestTarget_Delete(t *testing.T) {
	target := (&Target{"key": "value"}).Delete("key")
	if _, ok := (*target)["key"]; ok {
		t.Fatal("expected key to be removed.")
	}
}
