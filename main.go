package main

import (
	"encoding/hex"
	"github.com/docopt/docopt-go"
	"github.com/minio/sha256-simd"
	"github.com/tkellen/memorybox/cli"
	"github.com/tkellen/memorybox/lib"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const version = "dev"
const usage = `Usage:
  $0 [--root=<path> --concurrency=<num> --debug] put local <files>...
  $0 [--root=<path> --debug] get local <hash>
  $0 [--root=<path> --debug] annotate local <hash> <key> <value>
  $0 [--concurrency=<num> --debug] put s3 <bucket> <files>...
  $0 [--debug] get s3 <bucket> <hash> 
  $0 [--debug] annotate s3 <bucket> <hash> <key> <value>

Options:
  -c --concurrency=<num>   Max number of concurrent operations [default: 10].
  -d --debug               Show debugging output [default: false].
  -h --help                Show this screen.
  -r --root=<path>         Root store path (local only) [default: ~/memorybox].
  -v --version             Show version.
`

func main() {
	var cmd *cli.Command
	var err error
	// Remove timestamp from any log messages.
	log.SetFlags(0)
	// Respect what the user named the binary.
	usage := strings.ReplaceAll(usage, "$0", filepath.Base(os.Args[0]))
	// Parse command line arguments.
	opts, _ := docopt.ParseArgs(usage, os.Args[1:], version)
	// Initialize and run desired action.
	if cmd, err = processOpts(opts); err == nil {
		err = cmd.Dispatch()
	}
	// If initialization or execution failed, log why and exit non-zero.
	if err != nil {
		log.Printf("Error(s):\n%s", err)
		os.Exit(1)
	}
}

func processOpts(opts docopt.Opts) (*cli.Command, error) {
	var err error
	var store memorybox.Store
	// Configure local storage mode.
	if opts["local"].(bool) {
		store, err = memorybox.NewLocalStore(opts["--root"].(string))
	}
	// Configure object storage mode.
	if opts["s3"].(bool) {
		store, err = memorybox.NewObjectStore(opts["bucket"].(string))
	}
	// Begin configuring command line executor.
	cmd := &cli.Command{
		Store:   store,
		Logger:  func(format string, v ...interface{}) {},
		Reader:  inputReader,
		Writer:  outputWriter,
		Cleanup: cleanup,
	}
	// Enable informational logging while in debug mode.
	if opts["--debug"].(bool) {
		cmd.Logger = log.Printf
	}
	// If using put action, configure it.
	if opts["put"].(bool) {
		cmd.Action = "put"
		cmd.Inputs = opts["<files>"].([]string)
	}
	// If using get action, configure it.
	if opts["get"].(bool) {
		cmd.Action = "get"
		cmd.Request = opts["<hash>"].(string)
	}
	// Determine maximum concurrent operations.
	if s, err := strconv.ParseInt(opts["--concurrency"].(string), 10, 8); err == nil {
		cmd.Concurrency = int(s)
	}
	return cmd, err
}

// digest computes the sha256 message digest of a provided io.ReadCloser.
func digest(input io.ReadCloser) (string, error) {
	hash := sha256.New()
	if _, err := io.Copy(hash, input); err != nil {
		return "", nil
	}
	input.Close()
	return "sha256-" + hex.EncodeToString(hash.Sum(nil)), nil
}
