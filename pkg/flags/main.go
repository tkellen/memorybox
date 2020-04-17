// Package flags is responsible for converting command line options into a typed
// struct representing their values.
package flags

import (
	"fmt"
	"github.com/docopt/docopt-go"
)

const usageTemplate = `Usage:
  %[1]s [options] <target> get <hash>
  %[1]s [options] <target> put <input>...
  %[1]s <target> meta <hash> [delete [<key>] | set <key> <value>]
  %[1]s config [delete <target> [<key>] | set <target> <key> <value>]

Options:
  -c --concurrency=<num>   Max number of concurrent operations [default: 10].
  -d --debug               Show debugging output [default: false].
  -h --help                Show this screen.
  -v --version             Show version.

Examples
  %[1]s config set local type localDisk
  %[1]s config set local home ~/memorybox
  %[1]s config set local extra value
  %[1]s config
  %[1]s config delete local extra
  %[1]s config set bucket type s3
  %[1]s config set bucket home s3://bucket-name
  %[1]s config delete bucket
  printf "data" | %[1]s local put -
  %[1]s local put **/*.go
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
	Get         bool
	Meta        bool
	Input       []string
	Hash        string
	Concurrency int
	Debug       bool
}

// New creates an instance of Flags and populates it by parsing command line
// flags using docopts.
func New(args []string, version string) (Flags, error) {
	var err error
	// Respect what the user named the binary.
	usage := fmt.Sprintf(usageTemplate, args[0])
	flags := Flags{}
	// Parse command line flags.
	opts, _ := (&docopt.Parser{
		HelpHandler: func(parseErr error, usage string) {
			err = parseErr
		},
	}).ParseArgs(usage, args[1:], version)
	if err != nil {
		return flags, fmt.Errorf("%s", usage)
	}
	// Populate flags struct with our command line options.
	err = opts.Bind(&flags)
	return flags, err
}

// Method returns a string value representing which command is expected to be
// run for a given configuration of command line options. Consumers are expected
// to use this information to choose which method to invoke when running the
// program.
func (f Flags) Method() string {
	if f.Put {
		return "PutMain"
	}
	if f.Get {
		return "GetMain"
	}
	if f.Config {
		if f.Delete {
			return "ConfigDelete"
		}
		if f.Set {
			return "ConfigSet"
		}
		return "ConfigMain"
	}
	if f.Meta {
		if f.Delete {
			return "MetaDelete"
		}
		if f.Set {
			return "MetaSet"
		}
		return "MetaMain"
	}
	return "NotImplemented"
}
