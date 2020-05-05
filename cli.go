package main

import (
	"context"
	"fmt"
	"github.com/jessevdk/go-flags"
	"github.com/mitchellh/go-homedir"
	"github.com/tkellen/cli"
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
func Run(args []string, stdout io.Writer, stderr io.Writer) (int, error) {
	// Create context to pass into all commands to enable cancellation.
	background, cancel := context.WithCancel(context.Background())
	// Start building context for commands.
	ctx := ctx{
		name: path.Base(args[0]),
		ui: &ui{
			output:  log.New(stdout, "", 0),
			error:   log.New(stderr, "", 0),
			verbose: log.New(ioutil.Discard, "", 0),
		},
		background: background,
	}
	// Start goroutine to capture user requesting early shutdown (CTRL+C).
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		ctx.ui.error.Print("shutdown signal received, cleaning up")
		// Tell all goroutines that their context has been cancelled.
		cancel()
		// Give some time to clean up gracefully.
		time.Sleep(time.Second * 5)
	}()
	// Extract global options and return remaining command line arguments.
	remain, err := flags.NewParser(&ctx.flag, flags.PassDoubleDash).ParseArgs(args[1:])
	if err != nil {
		return 1, err
	}
	// Enable verbose debugging to error stream if user has requested it.
	if ctx.flag.Debugging {
		ctx.ui.verbose.SetOutput(ctx.ui.error.Writer())
	}
	ctx.ui.verbose.Printf("%s", ctx.flag)
	// Find full path to configuration file.
	fullPath, expandErr := homedir.Expand(ctx.flag.ConfigPath)
	if expandErr != nil {
		return 1, expandErr
	}
	// Ensure configuration directory exists.
	if err := os.MkdirAll(path.Dir(fullPath), 0755); err != nil {
		return 1, err
	}
	// Open / ensure configuration file exists.
	file, fileErr := os.OpenFile(fullPath, os.O_RDWR|os.O_CREATE, 0644)
	if fileErr != nil {
		return 1, fileErr
	}
	// Load configuration file.
	config, configErr := cli.NewConfigFile(file, cli.ConfigTarget{
		"type": "localDisk",
		"path": "~/memorybox",
	})
	if configErr != nil {
		return 1, configErr
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
			return 1, targetErr
		}
		ctx.target = target
		store, storeErr := store.New(*ctx.target)
		if storeErr != nil {
			return 1, fmt.Errorf("failed to load %v: %s", ctx.flag.Target, storeErr)
		}
		ctx.store = store
	}
	if err := ctx.commands().Dispatch(remain); err != nil {
		return 1, err
	}
	return 0, nil
}

// ui defines output streams for command invocations.
type ui struct {
	output  *log.Logger
	error   *log.Logger
	verbose *log.Logger
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
	ui         *ui
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
			"get":     cli.Fn{Fn: ctx.get, MinArgs: 1, Help: ctx.help},
			"put":     cli.Fn{Fn: ctx.put, MinArgs: 1, Help: ctx.help},
			"import":  cli.Fn{Fn: ctx.importFn, MinArgs: 1, Help: ctx.help},
			"index": cli.Tree{
				Fn: ctx.index,
				SubCommands: cli.Map{
					"rehash": ctx.indexRehash,
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
  %[1]s [options] get <hash>
  %[1]s [options] put <input>...
  %[1]s [options] import <input>...
  %[1]s [options] index [rehash]
  %[1]s [options] meta <hash> [set <key> <value> | delete [<key>]]

Options:
  -c --config=<path>       Path to config file [default: ~/.memorybox/config].
  -d --debug               Show debugging output [default: false].  
  -m --max=<num>           Max concurrent operations [default: 10].
  -t --target=<name>       Target store [default: default].
`

func (ctx *ctx) help(_ []string) error {
	return fmt.Errorf(usageTemplate, ctx.name)
}

func (ctx *ctx) get(args []string) error {
	return store.Get(ctx.background, ctx.store, args[0], ctx.ui.output)
}

func (ctx *ctx) put(args []string) error {
	return store.Put(ctx.background, ctx.store, args, []string{}, ctx.flag.Max, ctx.ui.verbose, ctx.ui.output)
}

func (ctx *ctx) importFn(args []string) error {
	return store.Import(ctx.background, ctx.store, args, ctx.flag.Max, ctx.ui.verbose, ctx.ui.output)
}

func (ctx *ctx) index(_ []string) error {
	return store.Index(ctx.background, ctx.store, ctx.flag.Max, false, ctx.ui.verbose, ctx.ui.output)
}

func (ctx *ctx) indexRehash(_ []string) error {
	return store.Index(ctx.background, ctx.store, ctx.flag.Max, true, ctx.ui.verbose, ctx.ui.output)
}

func (ctx *ctx) metaGet(args []string) error {
	return store.MetaGet(ctx.background, ctx.store, args[0], ctx.ui.output)
}

func (ctx *ctx) metaSet(args []string) error {
	return store.MetaSet(ctx.background, ctx.store, args[0], args[1], args[2], ctx.ui.verbose, ctx.ui.output)
}

func (ctx *ctx) metaDelete(args []string) error {
	return store.MetaDelete(ctx.background, ctx.store, args[0], args[1], ctx.ui.verbose, ctx.ui.output)
}

func (ctx *ctx) version(_ []string) error {
	ctx.ui.output.Printf("%s", version)
	return nil
}
