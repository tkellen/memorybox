// Package memorybox combines all other packages and presents a public API which
// enables the consumption of them all. All other packages in this project are
// self contained / have no cross-package imports.
package memorybox

import (
	"fmt"
	"github.com/docopt/docopt-go"
	"github.com/korovkin/limiter"
	"github.com/tkellen/memorybox/src/cli"
	"github.com/tkellen/memorybox/src/config"
	"github.com/tkellen/memorybox/src/hashreader"
	"github.com/tkellen/memorybox/src/localdiskstore"
	"github.com/tkellen/memorybox/src/objectstore"
	"io"
	"log"
	"os"
	"path"
	"strings"
)

const version = "dev"
const usageTemplate = `Usage:
  %[1]s config (show | delete <target> | set <target> <key> <value>)
  %[1]s [options] (put | get) <target> <files>...

Options:
  -c --concurrency=<num>   Max number of concurrent operations [default: 10].
  -d --debug               Show debugging output [default: false].
  -h --help                Show this screen.
  -v --version             Show version.

Examples
  %[1]s config set local type localdisk
  %[1]s config set local home ~/memorybox
  %[1]s config set bucket type s3
  %[1]s config set bucket home s3://bucket-name
  %[1]s config show
  printf "data" | %[1]s config put local -
  %[1]s config put s3 **/*.jpg
`

// Store defines a storage engine that can save and retrieve content.
type Store interface {
	Exists(string) bool
	Get(string) (io.ReadCloser, error)
	Put(io.ReadCloser, string) error
	String() string
}

func Run(args []string) error {
	return cli.Run(&Runner{}, args)
}

// Runner implements an interface compatible with cli.Runner.
type Runner struct {
	*config.Config
	Store
}

func (*Runner) ConfigPath() string {
	return "~/.memorybox/config"
}
func (*Runner) TempPath() string {
	return path.Join(os.TempDir(), "memorybox")
}

// Configure parses command line arguments, loads a configuration file and
// prepares internal state required to run the application.
func (run *Runner) Configure(args []string, configData io.Reader) error {
	var parseErr error
	helpHandler := func(err error, usage string) {
		parseErr = err
	}
	// Parse command line arguments.
	usage := fmt.Sprintf(usageTemplate, "memorybox")
	parser := &docopt.Parser{
		HelpHandler:  helpHandler,
		OptionsFirst: true,
	}
	opts, _ := parser.ParseArgs(usage, args, version)
	if parseErr != nil {
		return fmt.Errorf("%s", usage)
	}
	// Populate flags struct with our command line options.
	if err := opts.Bind(&run.Config.Flags); err != nil {
		return err
	}
	// Load supplied configuration file.
	if err := run.Config.Load(configData); err != nil {
		return err
	}
	// Load defined target.
	target := run.Config.Target()
	// Initialize our target backing store.
	switch storeType := target.Get("type"); {
	case storeType == "localdisk":
		run.Store = localdiskstore.NewFromTarget(*target)
	case storeType == "s3":
		run.Store = objectstore.NewFromTarget(*target)
	default:
		return fmt.Errorf("unknown store type: %s", storeType)
	}
	return nil
}

// Configure provides a public method to persist a configuration file.
func (run *Runner) SaveConfig(writer io.Writer) error {
	return run.Config.Save(writer)
}

// Dispatch provides actually runs our program.
func (run *Runner) Dispatch() error {
	cfg := run.Config
	flags := cfg.Flags
	store := run.Store
	logger := func(format string, v ...interface{}) {}
	if flags.Debug {
		// Ensure timestamp is not included in logging messages.
		log.SetFlags(0)
		logger = log.Printf
	}
	if flags.Config {
		if flags.Show {
			log.Printf("%s", cfg)
			return nil
		}
		if flags.Delete {
			cfg.Delete(cfg.Flags.Target)
			return nil
		}
		if flags.Set {
			cfg.Target().Set(flags.Key, flags.Value)
			return nil
		}
	}
	var method func(string) error
	if flags.Put {
		method = func(request string) error {
			reader, digest, err := hashreader.Read(request, run.TempPath())
			if err != nil {
				return err
			}
			if store.Exists(digest) {
				logger("%s -> %s (skipped, already exists)", request, digest)
				return nil
			}
			logger("%s -> %s", request, digest)
			return run.Store.Put(reader, digest)
		}
	}
	if flags.Get {
		method = func(request string) error {
			data, err := store.Get(request)
			if err != nil {
				return err
			}
			if _, err := io.Copy(os.Stdout, data); err != nil {
				return err
			}
			return nil
		}
	}
	return limitRunner(method, flags.Files, flags.Concurrency)
}

func limitRunner(fn func(string) error, requests []string, max int) error {
	var errs []string
	limit := limiter.NewConcurrencyLimiter(max)
	// Iterate over all inputs.
	for _, item := range requests {
		request := item // ensure closure below gets right value
		limit.Execute(func() {
			if err := fn(request); err != nil {
				errs = append(errs, err.Error())
			}
		})
	}
	// Wait for all concurrent operations to complete.
	limit.Wait()
	// Collapse any errors into a single error for output to the user.
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "\n"))
	}
	return nil
}
