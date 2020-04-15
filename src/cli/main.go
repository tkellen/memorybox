// Package cli handles the work that many command line tool frameworks bury in a
// sea of implicit reflection-based conventions. In addition to making testing
// easy, it can be trivially reused and easily modified by accepting that owning
// ~50 lines of code is better than a single line import.
package cli

import (
	"github.com/mitchellh/go-homedir"
	"io"
	"os"
	"path"
)

// Runner defines an interface to run a command line program.
type Runner interface {
	ConfigPath() string
	TempPath() string
	Configure([]string, io.Reader) error
	SaveConfig(io.Writer) error
	Dispatch() error
}

// Run coordinates with all methods in the Runner interface to complete the
// cycle of running a command line application. This includes the following:
// 1. Ensures a temporary directory exists before running.
// 2. Ensures a configuration directory exists before running.
// 3. Ensures a configuration file is created/opened in read/write mode.
// 4. Run our application.
// 5. Remove the temporary directory and all of its contents before exiting.
// 6. If running the application changed the configuration, persist that to the
//    configuration file.
func Run(runner Runner, args []string) error {
	// Ensure temporary directory exists.
	if err := os.MkdirAll(runner.TempPath(), 0755); err != nil {
		return err
	}
	// Ensure temporary directory is removed when we're done working.
	defer os.RemoveAll(runner.TempPath())
	// Find full path to configuration file.
	fullPath, _ := homedir.Expand(runner.ConfigPath())
	// Ensure configuration directory exists.
	if err := os.MkdirAll(path.Dir(fullPath), 0755); err != nil {
		return err
	}
	// Open / ensure configuration file exists
	file, fileErr := os.OpenFile(fullPath, os.O_RDWR|os.O_CREATE, 0644)
	if fileErr != nil {
		return fileErr
	}
	// Instantiate configuration and storage engine.
	err := runner.Configure(args, file)
	if err != nil {
		return err
	}
	// Running our program may modify the configuration file. Ensure it is saved
	// when we are done.
	defer func() {
		file.Seek(0, 0)
		file.Truncate(0)
		runner.SaveConfig(file)
		file.Close()
	}()
	return runner.Dispatch()
}
