package main

import (
	"fmt"
	"github.com/docopt/docopt-go"
	"github.com/minio/minio-go/v6"
	"github.com/minio/minio-go/v6/pkg/credentials"
	"github.com/mitchellh/go-homedir"
	"github.com/tkellen/memorybox/pkg/cli"
	"github.com/tkellen/memorybox/pkg/localstore"
	"github.com/tkellen/memorybox/pkg/objectstore"
	"log"
	"os"
	"path"
)

const version = "dev"
const usage = `Usage:
  %[1]s [options] put local <files>...
  %[1]s [options] put s3 <bucket> <files>...
  %[1]s [options] get local <files>...
  %[1]s [options] get s3 <bucket> <files>...

Options:
  -c --concurrency=<num>   Max number of concurrent operations [default: 10].
  -d --debug               Show debugging output [default: false].
  -r --root=<path>         Root store path (local only) [default: ~/memorybox].
  -h --help                Show this screen.
  -v --version             Show version.`

type Flags struct {
	Put         bool
	Get         bool
	Local       bool
	Root        string
	S3          bool
	Bucket      string
	Files       []string
	Concurrency int
	Debug       bool
}

func (flags Flags) run() error {
	cmd := cli.New()
	cmd.Request = flags.Files
	if flags.Put {
		cmd.Action = "put"
	}
	if flags.Get {
		cmd.Action = "get"
	}
	cmd.Concurrency = flags.Concurrency
	if flags.Debug {
		cmd.Logger = log.Printf
	}
	if flags.Local {
		root, err := homedir.Expand(flags.Root)
		if err != nil {
			return err
		}
		store, err := localstore.New(root)
		if err != nil {
			return err
		}
		cmd.Store = store
	}
	if flags.S3 {
		// TODO support this in some real way with flags?
		creds := credentials.NewEnvAWS()
		client, err := minio.NewWithCredentials("s3.amazonaws.com", creds, true, "us-east-1")
		if err != nil {
			return err
		}
		cmd.Store = objectstore.New(client, flags.Bucket)
	}
	return cmd.Dispatch()
}

func main() {
	var err error
	var flags Flags
	// Ensure timestamp is not included in logging messages.
	log.SetFlags(0)
	// Parse command line arguments.
	opts, _ := docopt.ParseArgs(fmt.Sprintf(usage, path.Base(os.Args[0])), os.Args[1:], version)
	// Populate flags struct with our command line options.
	if err = opts.Bind(&flags); err == nil {
		err = flags.run()
	}
	if err != nil {
		log.Printf("Error(s):\n%s", err)
		os.Exit(1)
	}
}