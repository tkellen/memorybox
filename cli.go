package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/jessevdk/go-flags"
	"github.com/mitchellh/go-homedir"
	"github.com/tkellen/cli"
	"github.com/tkellen/memorybox/internal/fetch"
	"github.com/tkellen/memorybox/pkg/archive"
	"github.com/tkellen/memorybox/pkg/operations"
	"github.com/tkellen/memorybox/pkg/store"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"path"
	"syscall"
	"time"
)

// Run executes memorybox functionality from command line arguments.
func Run(args []string, stdout io.Writer, stderr io.Writer) int {
	// Disable global logger output.
	log.SetOutput(ioutil.Discard)
	// Create context to pass into all commands to enable cancellation.
	background, cancel := context.WithCancel(context.Background())
	// Start building context for commands.
	ctx := ctx{
		name: path.Base(args[0]),
		logger: &operations.Logger{
			Stdout:  log.New(stdout, "", 0),
			Stderr:  log.New(stderr, "", 0),
			Verbose: log.New(ioutil.Discard, "", 0),
		},
		background: background,
	}
	// Start goroutine to capture user requesting early shutdown (CTRL+C).
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		ctx.logger.Stderr.Print("shutdown signal received, cleaning up")
		// Tell all goroutines that their context has been cancelled.
		cancel()
		// Give some time to clean up gracefully.
		time.Sleep(time.Second * 5)
	}()
	// Extract global options and return remaining command line arguments.
	remain, err := flags.NewParser(&ctx.flag, flags.PassDoubleDash).ParseArgs(args[1:])
	if err != nil {
		ctx.logger.Stderr.Print(err)
		return 1
	}
	// Enable verbose debugging to error stream if user has requested it.
	if ctx.flag.Debugging {
		ctx.logger.Verbose.SetOutput(ctx.logger.Stderr.Writer())
	}
	ctx.logger.Verbose.Printf("%s", ctx.flag)
	// Find full path to configuration file.
	fullPath, _ := homedir.Expand(ctx.flag.ConfigPath)
	// Ensure configuration directory exists.
	if err := os.MkdirAll(path.Dir(fullPath), 0755); err != nil {
		ctx.logger.Stderr.Print(err)
		return 1
	}
	// Open / ensure configuration file exists.
	file, fileErr := os.OpenFile(fullPath, os.O_RDWR|os.O_CREATE, 0644)
	if fileErr != nil {
		ctx.logger.Stderr.Print(err)
		return 1
	}
	// Load configuration file.
	config, configErr := cli.NewConfigFile(file, cli.ConfigTarget{
		"type": "localDisk",
		"path": "~/memorybox",
	})
	if configErr != nil {
		ctx.logger.Stderr.Print(err)
		return 1
	}
	ctx.config = config
	// Running this program may modify the configuration file. Ensure it is
	// saved at exit.
	defer func() {
		// Force truncating this is probably a bad plan. Fix this!
		file.Seek(0, io.SeekStart)
		file.Truncate(0)
		config.Save(file)
		file.Close()
	}()
	// Instantiate store, but only if we need it.
	if len(remain) > 0 && remain[0] != "config" {
		// Fetch target store config.
		target, targetErr := config.Target(ctx.flag.Target)
		if targetErr != nil {
			ctx.logger.Stderr.Print(err)
			return 1
		}
		ctx.target = target
		s, storeErr := store.New(*ctx.target)
		if storeErr != nil {
			ctx.logger.Stderr.Print(fmt.Errorf("failed to load %v: %s", ctx.flag.Target, storeErr))
			return 1
		}
		ctx.store = s
	}
	if err := ctx.commands().Dispatch(remain); err != nil {
		if !errors.Is(err, context.Canceled) {
			ctx.logger.Stderr.Print(err)
		}
		return 1
	}
	return 0
}

// flag describes options that are globally available for all commands.
type flag struct {
	Debugging  bool   `short:"d" long:"debug"`
	ConfigPath string `short:"c" long:"config" default:"~/.memorybox/config"`
	Max        int    `short:"m" long:"max" default:"10"`
	Target     string `short:"t" long:"target" default:"default"`
}

// String pretty prints the content of all program options for debugging.
func (f flag) String() string {
	return fmt.Sprintf("flags (debugging: %v, config: %s, max: %d, target: %s)", f.Debugging, f.ConfigPath, f.Max, f.Target)
}

// ctx contains all the state required to call memorybox functionality.
type ctx struct {
	name       string
	background context.Context
	logger     *operations.Logger
	config     *cli.ConfigFile
	target     *cli.ConfigTarget
	store      store.Store
	flag       flag
}

// commands outputs a cli.Tree that can be used to execute commands.
func (ctx *ctx) commands() *cli.Tree {
	return &cli.Tree{
		Fn: ctx.help,
		SubCommands: cli.Map{
			"version": ctx.version,
			"help":    ctx.help,
			"hash":    cli.Fn{Fn: ctx.hash, MinArgs: 1, Help: ctx.help},
			"get":     cli.Fn{Fn: ctx.get, MinArgs: 1, Help: ctx.help},
			"put":     cli.Fn{Fn: ctx.put, MinArgs: 1, Help: ctx.help},
			"delete":  cli.Fn{Fn: ctx.delete, MinArgs: 1, Help: ctx.help},
			"import":  cli.Fn{Fn: ctx.importFn, MinArgs: 1, Help: ctx.help},
			"index": cli.Tree{
				Fn: ctx.index,
				SubCommands: cli.Map{
					"rehash": ctx.indexRehash,
					"update": cli.Fn{Fn: ctx.indexUpdate, MinArgs: 1, Help: ctx.help},
				},
			},
			"meta": cli.Tree{
				Fn: cli.Fn{Fn: ctx.metaGet, MinArgs: 1, Help: ctx.help},
				SubCommands: cli.Map{
					"set":    cli.Fn{Fn: ctx.metaSet, MinArgs: 3, Help: ctx.help},
					"delete": cli.Fn{Fn: ctx.metaDelete, MinArgs: 2, Help: ctx.help},
				},
			},
		},
	}
}

const usageTemplate = `Usage:
  %[1]s version
  %[1]s hash <input>
  %[1]s [options] get <request>
  %[1]s [options] put <input>...
  %[1]s [options] delete <request>...
  %[1]s [options] import <input>
  %[1]s [options] index [rehash | update <input>]
  %[1]s [options] meta <request> [set <key> <value> | delete <key>]

Options:
  -c --config=<path>       Path to config file [default: ~/.memorybox/config].
  -d --debug               Show debugging output [default: false].  
  -m --max=<num>           Max concurrent operations [default: 10].
  -t --target=<name>       Target store [default: default].
`

func (ctx *ctx) help(_ []string) error {
	return fmt.Errorf(usageTemplate, ctx.name)
}

func (ctx *ctx) hash(args []string) error {
	return fetch.One(ctx.background, args[0], func(request string, src io.ReadSeeker) error {
		digest, _, _ := archive.Sha256(src)
		ctx.logger.Stdout.Println(digest)
		return nil
	})
}

func (ctx *ctx) get(args []string) error {
	return operations.Get(ctx.background, ctx.logger, ctx.store, args[0])
}

func (ctx *ctx) put(args []string) error {
	return operations.Put(ctx.background, ctx.logger, ctx.store, ctx.flag.Max, args, []string{})
}

func (ctx *ctx) delete(args []string) error {
	return operations.Delete(ctx.background, ctx.store, ctx.flag.Max, args)
}

func (ctx *ctx) importFn(args []string) error {
	return fetch.One(ctx.background, args[0], func(request string, src io.ReadSeeker) error {
		return operations.Import(ctx.background, ctx.logger, ctx.store, ctx.flag.Max, src)
	})
}

func (ctx *ctx) index(_ []string) error {
	return operations.Index(ctx.background, ctx.logger, ctx.store, ctx.flag.Max, false)
}

func (ctx *ctx) indexRehash(_ []string) error {
	return operations.Index(ctx.background, ctx.logger, ctx.store, ctx.flag.Max, true)
}

func (ctx *ctx) indexUpdate(args []string) error {
	return fetch.One(ctx.background, args[0], func(request string, src io.ReadSeeker) error {
		return operations.IndexUpdate(ctx.background, ctx.logger, ctx.store, ctx.flag.Max, src)
	})
}

func (ctx *ctx) metaGet(args []string) error {
	return operations.MetaGet(ctx.background, ctx.logger, ctx.store, args[0])
}

func (ctx *ctx) metaSet(args []string) error {
	return operations.MetaSet(ctx.background, ctx.logger, ctx.store, args[0], args[1], args[2])
}

func (ctx *ctx) metaDelete(args []string) error {
	return operations.MetaDelete(ctx.background, ctx.logger, ctx.store, args[0], args[1])
}

func (ctx *ctx) version(_ []string) error {
	ctx.logger.Stdout.Printf("%s", version)
	return nil
}
