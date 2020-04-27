package cli

import (
	"fmt"
	"github.com/docopt/docopt-go"
	"github.com/tkellen/memorybox/internal/configfile"
	"github.com/tkellen/memorybox/internal/store"
	"io"
	"log"
	"os"
	"path"
)

const version = "dev"
const usageTemplate = `Usage:
  %[1]s [-d] get <target> <hash>
  %[1]s [--concurrency=<num> -d] put <target> <input>...
  %[1]s [--concurrency=<num> -d] import <target> <input>...
  %[1]s [-d] meta <target> <hash> [set <key> <value> | delete [<key>]]
  %[1]s [-d] config [set <target> <key> <value> | delete <target> [<key>]]

Options:
  -c --concurrency=<num>   Max concurrent operations [default: 10].
  -d --debug               Show debugging output [default: false].
  -h --help                Show this screen.
  -v --version             Show version.

Examples
  %[1]s config set local type localDisk
  %[1]s config set local home ~/memorybox
  %[1]s config set local extra value
  %[1]s config
  %[1]s config delete local extra
  %[1]s -d put local **/*.go
  %[1]s -d put local https://scaleout.team/logo.svg  
  printf "data" | %[1]s -d put local -
  %[1]s -d get local 3a
  %[1]s -d meta local 3a | jq
  %[1]s -d meta local 3a set newKey someValue
  %[1]s -d meta local 3a delete newKey
`

// Flags provides a typed interface to all supported command line arguments.
type Flags struct {
	Config      bool
	Delete      bool
	Target      string
	Set         bool
	Key         string
	Value       string
	Put         bool
	Import      bool
	Get         bool
	Meta        bool
	Input       []string
	Hash        string
	Concurrency int
	Debug       bool
}

// Runner implements simplecli.Runner in the context of memorybox.
type Runner struct {
	Logger     *log.Logger
	ConfigFile *configfile.ConfigFile
	Flags      Flags
	Store      store.Store
	HashFn     func(source io.Reader) (string, int64, error)
	PathConfig string
	PathTemp   string
}

// New creates a runner with all the required configuration.
func New(logger *log.Logger) *Runner {
	return &Runner{
		Logger:     logger,
		HashFn:     store.Sha256,
		PathConfig: "~/.memorybox/config",
		PathTemp:   path.Join(os.TempDir(), "memorybox"),
	}
}

// ConfigPath returns the canonical location of the memorybox config file.
func (run *Runner) ConfigPath() string {
	return run.PathConfig
}

// TempPath returns the path to a temp directory used during put operations
// where content must be temporarily buffered to local disk.
func (run *Runner) TempPath() string {
	return run.PathTemp
}

// Configure is responsible parsing what was provided on the command line.
func (run *Runner) Configure(args []string, configData io.Reader) error {
	// Instantiate flags from command line arguments.
	var err error
	// Respect what the user named the binary.
	usage := fmt.Sprintf(usageTemplate, args[0])
	// Parse command line flags.
	opts, _ := (&docopt.Parser{
		HelpHandler: func(parseErr error, usage string) {
			err = parseErr
		},
	}).ParseArgs(usage, args[1:], version)
	if err != nil {
		return fmt.Errorf("%s", usage)
	}
	// Populate flags struct with our command line options.
	err = opts.Bind(&run.Flags)
	// Turn logger off unless user has requested it.
	if run.Flags.Debug {
		run.Logger.SetOutput(os.Stderr)
	}
	configFile, configFileErr := configfile.NewConfigFile(configData)
	if configFileErr != nil {
		return configFileErr
	}
	run.ConfigFile = configFile
	if !run.Flags.Config {
		// Only create a backing store if we're going to interact with one.
		target := run.ConfigFile.Target(run.Flags.Target)
		store, storeErr := store.New(*target)
		if storeErr != nil {
			return fmt.Errorf("failed to load %v: %s", target, storeErr)
		}
		run.Store = store
	}
	return nil
}

// Dispatch actually runs our commands.
func (run *Runner) Dispatch() error {
	f := run.Flags
	if f.Put {
		return store.PutMany(run.Store, run.HashFn, run.Flags.Input, run.Flags.Concurrency, run.Logger, []string{})
	}
	if f.Import {
		return store.Import(run.Store, run.HashFn, run.Flags.Input, run.Flags.Concurrency, run.Logger)
	}
	if f.Get {
		return store.Get(run.Store, run.Flags.Hash, os.Stdout)
	}
	if f.Config {
		if f.Delete {
			if run.Flags.Target != "" {
				run.ConfigFile.Target(run.Flags.Target).Delete(run.Flags.Key)
				return nil
			}
			run.ConfigFile.Delete(run.Flags.Target)
			return nil
		}
		if f.Set {
			run.ConfigFile.Target(run.Flags.Target).Set(run.Flags.Key, run.Flags.Value)
			return nil
		}
		log.Printf("%s", run.ConfigFile)
		return nil
	}
	if f.Meta {
		if f.Delete {
			return store.MetaDelete(run.Store, run.Flags.Hash, run.Flags.Key)
		}
		if f.Set {
			return store.MetaSet(run.Store, run.Flags.Hash, run.Flags.Key, run.Flags.Value)
		}
		return store.MetaGet(run.Store, run.Flags.Hash, os.Stdout)
	}
	return fmt.Errorf("command not implemented")
}

// Shutdown writes the contents of the in-memory config to the on-disk config
// file for memorybox.
func (run *Runner) Shutdown(writer io.Writer) error {
	return run.ConfigFile.Save(writer)
}
