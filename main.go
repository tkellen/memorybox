package main

import (
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
  $0 [--root=<path> --concurrency=<num> --debug] put local <glob>...
  $0 [--concurrency=<num> --debug] put s3 <bucket> <glob>...
  $0 [--root=<path> --debug] get local <glob>
  $0 [--debug] get s3 <bucket> <glob>

Options:
  -c --concurrency=<num>   Max number of concurrent operations [default: 10].
  -d --debug               Show debugging output [default: false].
  -h --help                Show this screen.
  -r --root=<path>         Root store path (local only) [default: ~/memorybox].
  -v --verbose             Show version.
`

func main() {
	var command string
	var concurrency int
	var store memorybox.Store
	var err error
	// Respect what the user named the binary.
	usage := strings.ReplaceAll(usage, "$0", filepath.Base(os.Args[0]))
	// Parse command line arguments.
	cli, _ := docopt.ParseArgs(usage, os.Args[1:], version)
	// Determine which command we are running.
	if cli["put"].(bool) {
		command = "put"
	}
	if cli["get"].(bool) {
		command = "get"
	}
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
	// Make informational logging silent by default with a noop function.
	var logger memorybox.Logger
	// Enable information logging while in debug mode.
	if cli["--debug"].(bool) {
		logger = log.Printf
	}
	// Remove timestamp from any log messages.
	log.SetFlags(0)
	// Determine which files we are operating on.
	files := cli["<glob>"].([]string)

	// Execute put command.
	if command == "put" {
		// Configure maximum concurrent operations.
		if s, err := strconv.ParseInt(cli["--concurrency"].(string), 10, 8); err == nil {
			concurrency = int(s)
		}
		// Configure concurrency limiting.
		limit := limiter.NewConcurrencyLimiter(concurrency)
		// Process every input with a limit on how fast this is performed.
		for _, input := range files {
			// Save input in a variable that will be in scope for the closure below.
			path := input
			// Execute the puts as fast as we've allowed.
			limit.Execute(func() {
				if err := memorybox.Put(path, store, logger); err != nil {
					log.Printf("bad deal: %s", err)
				}
			})
		}
		// Wait for all copy operations to complete
		limit.Wait()
	}

	// Execute get command.
	if command == "get" {
		err = memorybox.Get(files[0], store, logger)
	}

	if err != nil {
		log.Printf("%s", err)
		os.Exit(1)
	}
}
