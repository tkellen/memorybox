package main

import (
	"fmt"
	"github.com/docopt/docopt-go"
	"github.com/korovkin/limiter"
	"github.com/tkellen/memorybox/lib"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const version = "dev"
const usage = `memorybox

Usage:
  $0 [--root=<path> --concurrency=<num> --debug] put local <files>...
  $0 [--concurrency=<num> --debug] put s3 <bucket> <files>...
  $0 [--root=<path> --debug] get local <files>
  $0 [--debug] get s3 <bucket> <files>

Options:
  -c --concurrency=<num>   Max number of concurrent operations [default: 10].
  -d --debug               Show debugging output [default: false].
  -h --help                Show this screen.
  -r --root=<path>         Root store path (local only) [default: ~/memorybox].
  -v --verbose             Show version.
`

// Command configures how the cli should execute.
type Command struct {
	Store       memorybox.Store
	Inputs      []string
	Logger      memorybox.Logger
	Concurrency int
}

// String returns a human friendly representation of the command configuration.
func (c *Command) String() string {
	return fmt.Sprintf("%s\nInputs:\n  %s\nConcurrency: %d", c.Store, strings.Join(c.Inputs, "\n  "), c.Concurrency)
}

func main() {
	var concurrency int
	var store memorybox.Store
	var err error
	// Remove timestamp from any log messages.
	log.SetFlags(0)
	// Respect what the user named the binary.
	usage := strings.ReplaceAll(usage, "$0", filepath.Base(os.Args[0]))
	// Parse command line arguments.
	cli, _ := docopt.ParseArgs(usage, os.Args[1:], version)
	// If we are using a local storage, initialize the store for it.
	if cli["local"].(bool) {
		store, err = memorybox.NewLocalStore(cli["--root"].(string))
	}
	// If we are using object storage, initialize the store for it.
	if cli["s3"].(bool) {
		store, err = memorybox.NewObjectStore(cli["<bucket>"].(string))
	}
	// Bail out if we couldn't instantiate the backing store.
	if err != nil {
		log.Fatal(err)
	}
	// Determine maximum concurrent operations.
	if s, err := strconv.ParseInt(cli["--concurrency"].(string), 10, 8); err == nil {
		concurrency = int(s)
	}
	// Assemble configuration for running command.
	command := &Command{
		Store:       store,
		Inputs:      cli["<files>"].([]string),
		Concurrency: concurrency,
		Logger:      func(string, ...interface{}) {}, // No logs by default.
	}
	// Enable informational logging while in debug mode.
	if cli["--debug"].(bool) {
		command.Logger = log.Printf
	}
	// Run the put command (if it was specified).
	if cli["put"].(bool) {
		err = putCmd(command)
	}
	// Run the get command (if it was specified).
	if cli["get"].(bool) {
		err = getCmd(command)
	}
	// Exit non-zero if there were any errors.
	if err != nil {
		log.Printf("Error(s):\n%s", err)
		os.Exit(1)
	}
}

// Send all inputs to the store, capping how many requests we run simultaneously
// to match our desired concurrency limit.
func putCmd(cmd *Command) error {
	var errs []string
	cmd.Logger("Command: put\n%s", cmd)
	limit := limiter.NewConcurrencyLimiter(cmd.Concurrency)
	for _, input := range cmd.Inputs {
		path := input
		limit.Execute(func() {
			if err := memorybox.Put(path, cmd.Store, cmd.Logger); err != nil {
				errs = append(errs, fmt.Sprintf("put: %s", err))
			}
		})
	}
	limit.Wait()
	if len(errs) > 0 {
		return fmt.Errorf(strings.Join(errs, "\n"))
	}
	return nil
}

func getCmd(cmd *Command) error {
	cmd.Logger("Command: get\n%s", cmd)
	return memorybox.Get(cmd.Inputs[0], cmd.Store, cmd.Logger)
}
