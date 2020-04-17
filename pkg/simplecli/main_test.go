// Sure would be nice if a VFS implementation existed for golang.
package simplecli

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
	startup            func([]string, io.Reader) error
	saveConfig         func(io.Writer) error
	dispatch           func() error
}

func (run testRunner) ConfigPath() string {
	return run.configPath()
}
func (run testRunner) TempPath() string {
	return run.tempPath()
}
func (run testRunner) Configure(args []string, reader io.Reader) error {
	return run.startup(args, reader)
}
func (run testRunner) Shutdown(writer io.Writer) error {
	return run.saveConfig(writer)
}
func (run testRunner) Dispatch() func() error {
	return func() error {
		return nil
	}
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
	table := map[string]struct {
		run                  *testRunner
		setup                func(*testRunner) error
		configFileContent    []byte
		configFileCreateFail bool
		tempPathCreateFail   bool
		expectedErr          error
	}{
		"success": {
			run: newTestRunner(),
			setup: func(*testRunner) error {
				return nil
			},
			configFileContent: []byte("test"),
			expectedErr:       nil,
		},
		"failure on creating config": {
			run:               newTestRunner(),
			configFileContent: []byte("test"),
			setup: func(run *testRunner) error {
				// prevent configuration file from being created by putting a
				// directory in its place
				return os.MkdirAll(run.configPath(), 0755)
			},
			expectedErr: errors.New("is a directory"),
			// ^ figure out how to use errors.Is (this will never happen)
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
			expectedErr: errors.New("mkdir"),
			// ^ figure out how to use errors.Is (this will never happen)
		},
		"failure on creating temp directory": {
			run: newTestRunner(),
			setup: func(run *testRunner) error {
				// prevent temporary directory from being created by putting a
				// file in its place
				if err := os.MkdirAll(path.Dir(run.tempPath()), 0755); err != nil {
					return err
				}
				file, err := os.Create(run.tempPath())
				file.Close()
				return err
			},
			expectedErr: errors.New("mkdir"),
			// ^ figure out how to use errors.Is (this will never happen)
		},
		"failure on running startup": {
			run: newTestRunner(),
			setup: func(run *testRunner) error {
				// force startup call to fail
				run.startup = func(i []string, reader io.Reader) error {
					return errors.New("bad time")
				}
				return nil
			},
			expectedErr: errors.New("bad time"),
			// ^ figure out how to use errors.Is (this will never happen)
		},
		// TODO: test what is passed into run.Configure and run.Shutdown

	}
	for name, test := range table {
		t.Run(name, func(t *testing.T) {
			defer func() {
				os.RemoveAll(test.run.tempPath())
				os.RemoveAll(path.Dir(test.run.configPath()))
			}()
			setupErr := test.setup(test.run)
			if setupErr != nil {
				t.Fatalf("setting up test: %s", setupErr)
			}
			err := Run(test.run, []string{"one", "two"})
			if err != nil && test.expectedErr == nil {
				t.Fatalf("did not expect error: %s", err)
			}
			if err != nil && test.expectedErr != nil && !strings.Contains(err.Error(), test.expectedErr.Error()) {
				t.Fatalf("expected error %s, got %s", test.expectedErr, err)
			}
		})
	}
}
