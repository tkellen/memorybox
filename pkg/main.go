// Package memorybox combines all other packages to produce the command line
// interface to this tool.
package memorybox

import (
	"fmt"
	"github.com/tkellen/memorybox/pkg/commands/config"
	"github.com/tkellen/memorybox/pkg/commands/get"
	"github.com/tkellen/memorybox/pkg/commands/meta"
	"github.com/tkellen/memorybox/pkg/commands/put"
	"github.com/tkellen/memorybox/pkg/configfile"
	"github.com/tkellen/memorybox/pkg/flags"
	"github.com/tkellen/memorybox/pkg/store"
	"io"
	"log"
	"os"
	"path"
	"reflect"
)

const version = "dev"

// Runner implements simplecli.Runner in the context of memorybox.
type Runner struct {
	Logger     func(format string, v ...interface{})
	ConfigFile *configfile.ConfigFile
	Flags      flags.Flags
	Store      store.Store
	ConfigCmd  config.Commands
	GetCmd     get.Commands
	PutCmd     put.Commands
	MetaCmd    meta.Commands
}

// ConfigPath returns the canonical location of the memorybox config file.
func (*Runner) ConfigPath() string {
	return "~/.memorybox/config"
}

// TempPath returns the path to a temp directory used during put operations
// where content must be temporarily buffered to local disk.
func (*Runner) TempPath() string {
	return path.Join(os.TempDir(), "memorybox")
}

// Configure is responsible for fully populating the Runner struct with all
// context needed to actually run memorybox.
func (run *Runner) Configure(args []string, configData io.Reader) error {
	// Instantiate flags from command line arguments.
	flags, flagsErr := flags.New(args, version)
	if flagsErr != nil {
		return flagsErr
	}
	run.Flags = flags
	// Make logger silent (or not) depending on flags.
	if flags.Debug {
		run.Logger = log.Printf
	} else {
		run.Logger = func(format string, v ...interface{}) {}
	}
	// Load supplied configuration file.
	configFile, configFileErr := configfile.New(configData)
	if configFileErr != nil {
		return configFileErr
	}
	run.ConfigFile = configFile
	// If we are running the config command we do not need a backing store.
	if !flags.Config {
		// Load backing store.
		store, storeErr := store.New(*configFile.Target(flags.Target))
		if storeErr != nil {
			return storeErr
		}
		run.Store = store
	}
	// Load all command interfaces.
	run.ConfigCmd = config.Command{}
	run.GetCmd = get.Command{}
	run.PutCmd = put.Command{
		Concurrency: flags.Concurrency,
		Logger:      run.Logger,
	}
	run.MetaCmd = meta.Command{}
	return nil
}

// Dispatch determines which function simplecli should execute and returns a
// reference to it.
func (run *Runner) Dispatch() func() error {
	method := run.Flags.Method()
	return reflect.ValueOf(run).MethodByName(method).Interface().(func() error)
}

// Shutdown writes the contents of the in-memory config to the on-disk config
// file for memorybox.
func (run *Runner) Shutdown(writer io.Writer) error {
	return run.ConfigFile.Save(writer)
}

// All methods that follow call commands using configuration derived from a
// single invocation of the memorybox CLI.

// GetMain fetches an object from the configured backing store.
func (run *Runner) GetMain() error {
	return run.GetCmd.Main(run.Store, run.Flags.Hash, os.Stdout)
}

// PutMain persists the requested inputs into the configured backing store.
func (run *Runner) PutMain() error {
	return run.PutCmd.Main(run.Store, run.Flags.Input)
}

// ConfigMain displays the current configuration file contents.
func (run *Runner) ConfigMain() error {
	return run.ConfigCmd.Main(run.ConfigFile)
}

// ConfigSet modifies the configuration of a target in the configuration file.
func (run *Runner) ConfigSet() error {
	return run.ConfigCmd.Set(run.ConfigFile, run.Flags.Target, run.Flags.Key, run.Flags.Value)
}

// ConfigDelete removes a target from the configuration file.
func (run *Runner) ConfigDelete() error {
	return run.ConfigCmd.Delete(run.ConfigFile, run.Flags.Target, run.Flags.Key)
}

// MetaMain displays the metadata associated with a requested object.
func (run *Runner) MetaMain() error {
	return run.MetaCmd.Main(run.Store, run.Flags.Hash, os.Stdout)
}

// MetaSet associates metadata with a requested object.
func (run *Runner) MetaSet() error {
	return run.MetaCmd.Set(run.Store, run.Flags.Hash, run.Flags.Key, run.Flags.Value)
}

// MetaDelete remove an entry in the metadata associated with an object.
func (run *Runner) MetaDelete() error {
	return run.MetaCmd.Delete(run.Store, run.Flags.Hash, run.Flags.Key)
}

// NotImplemented reports a command line invocation for which a command could
// not be found. This should probably be in the Dispatch method.
func (run *Runner) NotImplemented() error {
	return fmt.Errorf("command not implemented")
}
