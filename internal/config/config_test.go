package config_test

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/tkellen/memorybox/internal/config"
	"gopkg.in/yaml.v2"
	"io"
	"reflect"
	"strings"
	"testing"
	"testing/iotest"
)

func TestConfig_String(t *testing.T) {
	expected := "targets:\n  test:\n    path: ~/app\n    type: localDisk\n"
	actual := &config.Config{
		Targets: map[string]config.Target{
			"test": {
				"type": "localDisk",
				"path": "~/app",
			},
		},
	}
	if expected != fmt.Sprintf("%s", actual) {
		t.Fatalf("expected %s, got %s", expected, actual)
	}
}

func TestConfig_Create(t *testing.T) {
	existingTarget := config.Target{
		"key": "value",
	}
	table := map[string]struct {
		cfg            *config.Config
		targetName     string
		storeType      string
		expectedTarget config.Target
	}{
		"new targets are added": {
			cfg: &config.Config{
				Targets: map[string]config.Target{},
			},
			targetName:     "test",
			storeType:      "s3",
			expectedTarget: nil, // doesn't exist yet
		},
		"existing targets are not overwritten": {
			cfg: &config.Config{
				Targets: map[string]config.Target{
					"existing": existingTarget,
				},
			},
			targetName:     "existing",
			storeType:      "s3",
			expectedTarget: existingTarget,
		},
	}
	for name, test := range table {
		t.Run(name, func(t *testing.T) {
			test.cfg.Create(test.targetName, test.storeType)
			target, ok := test.cfg.Targets[test.targetName]
			if !ok {
				t.Fatal("expected target to be created")
			}
			if test.expectedTarget != nil && !reflect.DeepEqual(target, test.expectedTarget) {
				t.Fatalf("expected target %v, got %v", test.expectedTarget, target)
			}
		})
	}
}

func TestConfig_Target(t *testing.T) {
	expectedTarget := &config.Target{}
	table := map[string]struct {
		cfg         *config.Config
		lookup      string
		expected    *config.Target
		expectedErr bool
	}{
		"existing target requested": {
			cfg: &config.Config{
				Targets: map[string]config.Target{
					"test": *expectedTarget,
				},
			},
			lookup:      "test",
			expected:    expectedTarget,
			expectedErr: false,
		},
		"missing target requested": {
			cfg: &config.Config{
				Targets: map[string]config.Target{},
			},
			lookup:      "test",
			expected:    expectedTarget,
			expectedErr: true,
		},
	}
	for name, test := range table {
		t.Run(name, func(t *testing.T) {
			actual, err := test.cfg.Target(test.lookup)
			if !test.expectedErr && err != nil {
				t.Fatalf("did not expect error %s", err)
			}
			if test.expected == actual {
				t.Fatalf("expected target %s, got %s", test.expected, actual)
			}
		})
	}
}

func TestConfig_Delete(t *testing.T) {
	table := map[string]struct {
		cfg                 *config.Config
		targetToDelete      string
		expectedTargetCount int
	}{
		"delete existing target": {
			cfg: &config.Config{
				Targets: map[string]config.Target{
					"test": {},
				},
			},
			targetToDelete: "test",
		},
		"delete non-existing target": {
			cfg: &config.Config{
				Targets: map[string]config.Target{
					"nope": {},
				},
			},
			targetToDelete: "test",
		},
	}
	for name, test := range table {
		t.Run(name, func(t *testing.T) {
			test.cfg.Delete(test.targetToDelete)
			if _, ok := test.cfg.Targets[test.targetToDelete]; ok {
				t.Fatal("deleted target still present")
			}
		})
	}
}

func TestConfig_Load(t *testing.T) {
	goodInput := []byte("targets:\n  test:\n    path: ~/app\n    type: localDisk\n")
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
			cfg := config.Config{}
			err := cfg.Load(test.input)
			if test.expectedErr == nil && err != nil {
				t.Fatalf("did not expect error: %s", err)
			}
			if err != nil && test.expectedErr != nil && !strings.Contains(err.Error(), test.expectedErr.Error()) {
				t.Fatalf("expected error: %s, got %s", test.expectedErr, err)
			}
			actual, _ := yaml.Marshal(cfg)
			if !bytes.Equal(test.expected, actual) {
				t.Fatalf("load failed, expected %s, got %s", test.expected, actual)
			}
		})
	}
}

/*
func TestConfig_Save(t *testing.T) {
	cfg := &config.Config{
		Targets: map[string]config.Target{
			"test": {
				"type": "localDisk",
				"path": "~/app",
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
		cfg   *config.Config
		readerWriter io.ReadWriter
		expected     []byte
		expectedErr  error
	}{
		"success": {
			cfg:   cfg,
			readerWriter: bytes.NewBuffer([]byte{}),
			expected:     []byte("targets:\n  test:\n    path: ~/app\n    type: localDisk\n"),
			expectedErr:  nil,
		},
		"failure": {
			cfg:   cfg,
			readerWriter: badReadWriter,
			expected:     nil,
			expectedErr:  errors.New("already closed"),
		},
	}
	for name, test := range table {
		t.Run(name, func(t *testing.T) {
			err := test.cfg.Save()
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
*/
func TestTarget_Set(t *testing.T) {
	target := &config.Target{}
	target.Set("key", "value")
	if len(*target) != 1 {
		t.Fatal("expected one item in target configuration")
	}
}

func TestTarget_Get(t *testing.T) {
	expected := "value"
	target := &config.Target{"key": expected}
	actual := target.Get("key")
	if expected != actual {
		t.Fatalf("expected %s, got %s", expected, actual)
	}
}

func TestTarget_Delete(t *testing.T) {
	target := (&config.Target{"key": "value"}).Delete("key")
	if _, ok := (*target)["key"]; ok {
		t.Fatal("expected key to be removed.")
	}
}
