// Package simplecli handles the work that many command line tool frameworks
// bury in a sea of reflection based magic. In addition to making testing easy,
// it can be trivially reused by accepting that ~40 lines of easy to follow and
// modify code is better than thousands in a dependency.
package simplecli

import (
	"github.com/mitchellh/go-homedir"
	"io"
	"os"
	"os/signal"
	"path"
	"syscall"
)

// CLIRunner defines an interface for the lifecycle of a command line program.
type CLIRunner interface {
	// ConfigPath should return a path to a configuration file to read.
	ConfigPath() string
	// TempPath should return a path to a temporary directory the application
	// will use while running.
	TempPath() string
	// Configure receives the contents (if any) of the file referenced by
	// running ConfigPath. The concrete implementation of this interface should
	// load it  as needed and run any other startup steps required here.
	Configure([]string, io.Reader) error
	// Dispatch will perform the work being requested by the user's command line
	// options.
	Dispatch() error
	// SaveConfig receives a writable interface to the configuration file
	// returned by running ConfigPath. Writing to the configuration file and any
	// other shutdown related activities specific to the implementation of this
	// interface belong here.
	SaveConfig(io.Writer) error
	// Terminate is called when a SIGTERM signal is received. Implementors of
	// this interface should do whatever work is needed to stop work gracefully
	// here.
	Terminate()
}

// Run coordinates with all methods in the Runner interface to complete the
// cycle of running a command line application. This includes the following:
// 1. Ensuring a temporary directory exists before running.
// 2. Ensuring a configuration directory exists before running.
// 3. Ensuring a configuration file is created/opened in read/write mode.
// 4. Runs our application.
// 5. Notifies our application if it needs to shut down in response to SIGTERM.
// 6. Removing the temporary directory and all of its contents before exiting.
// 6. If running the application changed the configuration, persisting to the
//    configuration file.
func Run(cli CLIRunner, args []string) error {
	// Ensure SIGINT is handled gracefully.
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		cli.Terminate()
	}()
	// Ensure temporary directory exists.
	if err := os.MkdirAll(cli.TempPath(), 0755); err != nil {
		return err
	}
	// Ensure temporary directory is removed when we're done working.
	defer os.RemoveAll(cli.TempPath())
	// Find full path to configuration file.
	fullPath, _ := homedir.Expand(cli.ConfigPath())
	// Ensure configuration directory exists.
	if err := os.MkdirAll(path.Dir(fullPath), 0755); err != nil {
		return err
	}
	// Open / ensure configuration file exists
	file, fileErr := os.OpenFile(fullPath, os.O_RDWR|os.O_CREATE, 0644)
	if fileErr != nil {
		return fileErr
	}
	// Start CLI.
	err := cli.Configure(args, file)
	if err != nil {
		return err
	}
	// Running our program may modify the configuration file. Ensure it is saved
	// when we are done.
	defer func() {
		// Force truncating this is probably a bad plan. Fix this!
		file.Seek(0, 0)
		file.Truncate(0)
		cli.SaveConfig(file)
		file.Close()
	}()
	// Run command the user requested.
	return cli.Dispatch()
}
