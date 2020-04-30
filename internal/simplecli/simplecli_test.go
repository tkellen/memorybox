package simplecli_test

import (
	"errors"
	"github.com/segmentio/ksuid"
	"github.com/tkellen/memorybox/internal/simplecli"
	"io"
	"io/ioutil"
	"os"
	"path"
	"testing"
)

type testRunner struct {
	configPathLocation string
	tempPathLocation   string
	configPath         func() string
	startup            func([]string, io.Reader) error
	saveConfig         func(io.Writer) error
	dispatch           func() error
}

func (run testRunner) ConfigPath() string {
	return run.configPath()
}
func (run testRunner) Configure(args []string, reader io.Reader) error {
	return run.startup(args, reader)
}
func (run testRunner) SaveConfig(writer io.Writer) error {
	return run.saveConfig(writer)
}
func (run testRunner) Dispatch() error {
	return nil
}
func (run testRunner) Terminate() {
}

func newTestRunner() *testRunner {
	configPath := path.Join(os.TempDir(), ksuid.New().String(), ksuid.New().String())
	return &testRunner{
		configPath: func() string {
			return configPath
		},
		startup: func(args []string, reader io.Reader) error {
			_, err := ioutil.ReadAll(reader)
			return err
		},
		saveConfig: func(writer io.Writer) error {
			_, err := writer.Write([]byte("good"))
			return err
		},
		dispatch: func() error {
			return nil
		},
	}
}

func TestRun(t *testing.T) {
	type testCase struct {
		run                  *testRunner
		setup                func(*testRunner) error
		configFileContent    []byte
		configFileCreateFail bool
		expectedErr          bool
	}
	table := map[string]testCase{
		"success": {
			run: newTestRunner(),
			setup: func(*testRunner) error {
				return nil
			},
			configFileContent: []byte("test"),
			expectedErr:       false,
		},
		"failure on creating config": {
			run:               newTestRunner(),
			configFileContent: []byte("test"),
			setup: func(run *testRunner) error {
				// prevent configuration file from being created by putting a
				// directory in its place
				return os.MkdirAll(run.configPath(), 0755)
			},
			expectedErr: true,
		},
		"failure on creating config directory": {
			run:               newTestRunner(),
			configFileContent: []byte("test"),
			setup: func(run *testRunner) error {
				// prevent configuration directory from being created by putting
				// a file in is place.
				file, err := os.Create(path.Dir(run.configPath()))
				file.Close()
				return err
			},
			expectedErr: true,
		},
		"failure on running startup": func() testCase {
			err := errors.New("bad time")
			return testCase{
				run: newTestRunner(),
				setup: func(run *testRunner) error {
					// force startup call to fail
					run.startup = func(i []string, reader io.Reader) error {
						return err
					}
					return nil
				},
				expectedErr: true,
			}
		}(),
		// TODO: test what is passed into run.Configure and run.SaveConfig
	}
	for name, test := range table {
		t.Run(name, func(t *testing.T) {
			defer func() {
				os.RemoveAll(path.Dir(test.run.configPath()))
			}()
			setupErr := test.setup(test.run)
			if setupErr != nil {
				t.Fatalf("setting up test: %s", setupErr)
			}
			err := simplecli.Run(test.run, []string{"one", "two"})
			if err == nil && test.expectedErr {
				t.Fatalf("did not expect error: %s", err)
			}
			if err != nil && !test.expectedErr {
				t.Fatal("expected error")
			}
		})
	}
}
