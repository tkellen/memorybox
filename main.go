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

type Command struct {
	Store memorybox.Store
	Inputs []string
	Logger memorybox.Logger
	Concurrency int
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
	// Prep our backing store.
	if cli["local"].(bool) {
		store, err = memorybox.NewLocalStore(cli["--root"].(string))
	}
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
		Store: store,
		Inputs: cli["<files>"].([]string),
		Concurrency: concurrency,
		Logger: func(string, ...interface{}) {},
	}
	// Enable informational logging while in debug mode.
	if cli["--debug"].(bool) {
		command.Logger = log.Printf
	}
	// Execute our configuration
	if cli["put"].(bool) {
		err = put(command)
	}
	if cli["get"].(bool) {
		err = get(command)
	}
	if err != nil {
		log.Printf("%s", err)
		os.Exit(1)
	}
}

func put(command *Command) error {
	var incomplete []error
	// Configure concurrency limiting.
	limit := limiter.NewConcurrencyLimiter(command.Concurrency)
	// Process every input with a limit on how fast this is performed.
	for _, input := range command.Inputs {
		// Save input in var that will be in scope for closure below.
		path := input
		// Execute puts as fast as we've allowed.
		limit.Execute(func() {
			if err := memorybox.Put(path, command.Store, command.Logger); err != nil {
				incomplete = append(incomplete, err)
			}
		})
	}
	// Wait for all operations to complete
	limit.Wait()
	if len(incomplete) > 0 {
		return fmt.Errorf("some put operations failed: %s", incomplete)
	}
	return nil
}

func get(command *Command) error {
	return memorybox.Get(command.Inputs[0], command.Store, command.Logger)
}