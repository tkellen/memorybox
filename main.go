package main

import (
	"fmt"
	"github.com/docopt/docopt-go"
	"github.com/minio/minio-go"
	"github.com/minio/minio-go/pkg/credentials"
	"github.com/mitchellh/go-homedir"
	"github.com/tkellen/memorybox/pkg/cli"
	"github.com/tkellen/memorybox/pkg/localstore"
	"github.com/tkellen/memorybox/pkg/objectstore"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const version = "dev"
const usage = `Usage:
  $0 [--root=<path> --concurrency=<num> --debug] put local [--] <files>...
  $0 [--root=<path> --debug] get local <hash>
  $0 [--root=<path> --debug] annotate local <hash> <key> <value>
  $0 [--concurrency=<num> --debug] put s3 <bucket> [--] <files>...
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
	// Remove timestamp from any log messages.
	log.SetFlags(0)
	// Respect what the user named the binary.
	usage := strings.ReplaceAll(usage, "$0", filepath.Base(os.Args[0]))
	// Parse command line arguments.
	opts, _ := docopt.ParseArgs(usage, os.Args[1:], version)
	// Initialize and run desired action.
	err := execute(opts)
	// If initialization or execution failed, log why and exit non-zero.
	if err != nil {
		log.Printf("Error(s):\n%s", err)
		os.Exit(1)
	}
}

func execute(opts docopt.Opts) error {
	// Begin configuring command line executor.
	cmd := cli.New()
	// Configure local store mode.
	if flag, ok := opts["local"].(bool); ok && flag {
		if root, ok := opts["--root"].(string); ok {
			expandedRoot, err := homedir.Expand(root)
			if err != nil {
				return fmt.Errorf("unable to expand path: %w", err)
			}
			store, err := localstore.New(expandedRoot)
			if err != nil {
				return err
			}
			cmd.Store = store
		}
	}
	// Configure object storage mode.
	if flag, ok := opts["s3"].(bool); ok && flag {
		if bucket, ok := opts["<bucket>"].(string); ok {
			creds := credentials.NewEnvAWS()
			client, err := minio.NewWithCredentials("s3.amazonaws.com", creds, true, "us-east-1")
			if err != nil {
				return err
			}
			cmd.Store = objectstore.New(client, bucket)
		}
	}
	// Enable informational logging while in debug mode.
	if flag, ok := opts["--debug"].(bool); ok && flag {
		cmd.Logger = log.Printf
	}
	// If using put action, configure it.
	if flag, ok := opts["put"].(bool); ok && flag {
		cmd.Action = "put"
		if files, ok := opts["<files>"].([]string); ok {
			cmd.Request = files
		}
	}
	// If using get action, configure it.
	if flag, ok := opts["get"].(bool); ok && flag {
		cmd.Action = "get"
		if hash, ok := opts["<hash>"].(string); ok {
			cmd.Request = []string{hash}
		}
	}
	// Determine maximum concurrent operations.
	if flag, ok := opts["--concurrency"].(string); ok {
		concurrency, err := strconv.ParseInt(flag, 10, 8)
		if err == nil {
			cmd.Concurrency = int(concurrency)
		}
	}
	return cmd.Dispatch()
}
