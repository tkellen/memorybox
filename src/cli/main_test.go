// Sure would be nice if a VFS implementation existed for golang.
package cli

import (
	"errors"
	"github.com/segmentio/ksuid"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"testing"
)

type testRunner struct {
	configPathLocation string
	tempPathLocation   string
	configPath         func() string
	tempPath           func() string
	configure          func([]string, io.Reader) error
	saveConfig         func(io.Writer) error
	dispatch           func() error
}

func (m testRunner) ConfigPath() string {
	return m.configPath()
}
func (m testRunner) TempPath() string {
	return m.tempPath()
}
func (m testRunner) Configure(args []string, reader io.Reader) error {
	return m.configure(args, reader)
}
func (m testRunner) SaveConfig(writer io.Writer) error {
	return m.saveConfig(writer)
}
func (m testRunner) Dispatch() error {
	return m.dispatch()
}

func newTestRunner() *testRunner {
	configPath := path.Join(os.TempDir(), ksuid.New().String(), ksuid.New().String())
	tempPath := path.Join(os.TempDir(), ksuid.New().String())
	return &testRunner{
		configPath: func() string {
			return configPath
		},
		tempPath: func() string {
			return tempPath
		},
		configure: func(args []string, reader io.Reader) error {
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
	table := map[string]struct {
		runner               *testRunner
		setup                func(*testRunner) error
		configFileContent    []byte
		configFileCreateFail bool
		tempPathCreateFail   bool
		args                 []string
		expectedErr          error
	}{
		"success": {
			runner: newTestRunner(),
			setup: func(*testRunner) error {
				return nil
			},
			configFileContent: []byte("test"),
			args:              []string{"test", "thing"},
			expectedErr:       nil,
		},
		"failure on creating config": {
			runner:            newTestRunner(),
			configFileContent: []byte("test"),
			setup: func(runner *testRunner) error {
				// prevent configuration file from being created by putting a
				// directory in its place
				return os.MkdirAll(runner.configPath(), 0755)
			},
			args:        []string{"test", "thing"},
			expectedErr: errors.New("is a directory"),
			// ^ figure out how to use errors.Is (this will never happen)
		},
		"failure on creating config directory": {
			runner:            newTestRunner(),
			configFileContent: []byte("test"),
			setup: func(runner *testRunner) error {
				// prevent configuration directory from being created by putting
				// a file in is place.
				file, err := os.Create(path.Dir(runner.configPath()))
				file.Close()
				return err
			},
			args:        []string{"test", "thing"},
			expectedErr: errors.New("mkdir"),
			// ^ figure out how to use errors.Is (this will never happen)
		},
		"failure on creating temp directory": {
			runner: newTestRunner(),
			setup: func(runner *testRunner) error {
				// prevent temporary directory from being created by putting a
				// file in its place
				if err := os.MkdirAll(path.Dir(runner.tempPath()), 0755); err != nil {
					return err
				}
				file, err := os.Create(runner.tempPath())
				file.Close()
				return err
			},
			args:        []string{"test", "thing"},
			expectedErr: errors.New("mkdir"),
			// ^ figure out how to use errors.Is (this will never happen)
		},
		"failure on running configure": {
			runner: newTestRunner(),
			setup: func(runner *testRunner) error {
				// force configure call to fail
				runner.configure = func(i []string, reader io.Reader) error {
					return errors.New("bad time")
				}
				return nil
			},
			args:        []string{"test", "thing"},
			expectedErr: errors.New("bad time"),
			// ^ figure out how to use errors.Is (this will never happen)
		},
		// TODO: test what is passed into runner.Configure and runner.SaveConfig

	}
	for name, test := range table {
		t.Run(name, func(t *testing.T) {
			defer func() {
				os.RemoveAll(test.runner.tempPath())
				os.RemoveAll(path.Dir(test.runner.configPath()))
			}()
			setupErr := test.setup(test.runner)
			if setupErr != nil {
				t.Fatalf("setting up test: %s", setupErr)
			}
			err := Run(test.runner, test.args)
			if err != nil && test.expectedErr == nil {
				t.Fatalf("did not expect error: %s", err)
			}
			if err != nil && test.expectedErr != nil && !strings.Contains(err.Error(), test.expectedErr.Error()) {
				t.Fatalf("expected error %s, got %s", test.expectedErr, err)
			}
		})
	}
}
