package main

import (
	"bytes"
	"context"
	"fmt"
	"github.com/jessevdk/go-flags"
	"github.com/tkellen/cli"
	"github.com/tkellen/memorybox/internal/config"
	"github.com/tkellen/memorybox/internal/fetch"
	"github.com/tkellen/memorybox/internal/lambda"
	"github.com/tkellen/memorybox/pkg/archive"
	"github.com/tkellen/memorybox/pkg/file"
	"github.com/tkellen/memorybox/pkg/localdiskstore"
	"github.com/tkellen/memorybox/pkg/objectstore"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"path"
	"syscall"
	"time"
)

// ctx contains all the state required to call memorybox functionality.
type ctx struct {
	name       string
	background context.Context
	config     *config.Config
	logger     *archive.Logger
	flag       flag
}

// flag describes options that are globally available for all command.
type flag struct {
	Debugging  bool   `short:"d" long:"debug"`
	ConfigPath string `short:"c" long:"config" default:"~/.memorybox/config"`
	Max        int    `short:"m" long:"max" default:"10"`
	Target     string `short:"t" long:"target" default:"default"`
	Lambda     bool   `short:"l" long:"lambda"`
}

// String pretty prints the content of all program options for debugging.
func (f flag) String() string {
	return fmt.Sprintf("flags (debugging: %v, config: %s, max: %d, target: %s)", f.Debugging, f.ConfigPath, f.Max, f.Target)
}

// Run executes memorybox functionality from command line arguments.
func Run(args []string, stdout io.Writer, stderr io.Writer) int {
	// Disable global logger output.
	log.SetOutput(ioutil.Discard)
	// Create context to pass into all command to enable cancellation.
	background, cancel := context.WithCancel(context.Background())
	// Start building context for command.
	ctx := &ctx{
		name: path.Base(args[0]),
		logger: &archive.Logger{
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
		time.Sleep(time.Second * 20)
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
	// Get configuration file from environment variable or disk.
	cfg, configErr := config.NewFromEnvOrFile(ctx.flag.ConfigPath, "MEMORYBOX_CONFIG")
	if configErr != nil {
		ctx.logger.Stderr.Print(configErr)
		return 1
	}
	ctx.config = cfg
	ctx.logger.Verbose.Printf("%s", ctx.flag)
	// Run command in lambda if requested and not already doing so.
	if ctx.flag.Lambda && os.Getenv("MEMORYBOX_LAMBDA_MODE") == "" {
		code, err := RunLambda(ctx, args)
		if err != nil {
			ctx.logger.Stderr.Print(err)
		}
		return code
	}
	if err := ctx.command().Dispatch(remain); err != nil {
		if ctx.background.Err() == nil {
			ctx.logger.Stderr.Print(err)
		}
		return 1
	}
	return 0
}

func RunLambda(ctx *ctx, args []string) (int, error) {
	var stdin io.Reader
	fi, _ := os.Stdin.Stat()
	if fi.Size() == 0 {
		stdin = bytes.NewReader([]byte{})
	} else {
		stdin = os.Stdin
	}
	stdout, stderr, code, runErr := lambda.Run(ctx.background, ctx.config.String(), args, stdin)
	if runErr != nil {
		return 1, runErr
	}
	if stdout != "" {
		ctx.logger.Stdout.Print(stdout)
	}
	if stderr != "" {
		ctx.logger.Stderr.Print(stderr)
	}
	return code, nil
}

// command outputs a cli.Tree that can be used to execute command.
func (ctx *ctx) command() *cli.Tree {
	return &cli.Tree{
		Fn: ctx.help,
		SubCommands: cli.Map{
			"version": ctx.version,
			"help":    ctx.help,
			"hash":    cli.Fn{Fn: ctx.hash, MinArgs: 1, Help: ctx.help},
			"get":     cli.Fn{Fn: ctx.get, MinArgs: 1, Help: ctx.help},
			"put":     cli.Fn{Fn: ctx.put, MinArgs: 1, Help: ctx.help},
			"sync":    cli.Fn{Fn: ctx.sync, MinArgs: 3, Help: ctx.help},
			"diff":    cli.Fn{Fn: ctx.diff, MinArgs: 2, Help: ctx.help},
			"delete":  cli.Fn{Fn: ctx.delete, MinArgs: 1, Help: ctx.help},
			"import":  cli.Fn{Fn: ctx.importFn, MinArgs: 2, Help: ctx.help},
			"index": cli.Tree{
				Fn: ctx.index,
				SubCommands: cli.Map{
					"update": ctx.indexUpdate,
				},
			},
			"lambda": cli.Tree{
				Fn: ctx.help,
				SubCommands: cli.Map{
					"create": ctx.lambdaCreate,
					"delete": ctx.lambdaDelete,
				},
			},
			"check": cli.Tree{
				Fn: cli.Fn{Fn: ctx.check, MinArgs: 1, Help: ctx.help},
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
  %[1]s hash <input>...
  %[1]s [-cdt] get <ref>
  %[1]s [-cdmt] put <path-or-url>...
  %[1]s [-cdmt] delete <ref>
  %[1]s [-cdmt] meta <ref> [set <key> <value> | delete <key>]
  %[1]s [-cdmt] index [update]
  %[1]s [-cdmt] import <name> <input>
  %[1]s [-cdmt] check (pairing | metafiles | datafiles)
  %[1]s [-cdmt] sync (metafiles | datafiles | all) <sourceTarget> <destTarget>
  %[1]s [-cdmt] diff <sourceTarget> <destTarget>
  %[1]s [-cdmt] lambda (create | delete)

Options:
  -c --config=<path>       Path to config file [default: ~/.memorybox/config].
  -l --lambda              Run in lambda.
  -d --debug               Show debugging output [default: false].  
  -m --max=<num>           Max concurrent operations [default: 10].
  -t --target=<name>       Target store [default: default].
`

func (ctx *ctx) withStore(target string, fn func(archive.Store) error) error {
	t, targetErr := ctx.config.Target(target)
	if targetErr != nil {
		return targetErr
	}
	var store archive.Store
	switch backend := t.Get("backend"); backend {
	case localdiskstore.Name:
		store = localdiskstore.New(t.Get("path"))
	case objectstore.Name:
		store = objectstore.NewFromConfig(*t)
	default:
		return fmt.Errorf("unknown backend %s", backend)
	}
	return func() error {
		defer ctx.config.Save()
		return fn(store)
	}()
}

func (ctx *ctx) version(_ []string) error {
	ctx.logger.Stdout.Print(version)
	return nil
}

func (ctx *ctx) help(_ []string) error {
	return fmt.Errorf(usageTemplate, ctx.name)
}

func (ctx *ctx) hash(args []string) error {
	return fetch.Do(ctx.background, args, ctx.flag.Max, false, func(innerCtx context.Context, _ int, file *file.File) error {
		ctx.logger.Stdout.Println(file.Name)
		return nil
	})
}

func (ctx *ctx) get(args []string) error {
	return ctx.withStore(ctx.flag.Target, func(store archive.Store) error {
		file, getErr := archive.GetDataByPrefix(ctx.background, store, args[0])
		if getErr != nil {
			return getErr
		}
		_, err := io.Copy(ctx.logger.Stdout.Writer(), file)
		return err
	})
}

func (ctx *ctx) put(args []string) error {
	return ctx.withStore(ctx.flag.Target, func(store archive.Store) error {
		return fetch.Do(ctx.background, args, ctx.flag.Max, true, func(innerCtx context.Context, index int, file *file.File) error {
			if err := archive.Put(innerCtx, store, file, ""); err != nil {
				return err
			}
			ctx.logger.Stdout.Print(file.Meta)
			return nil
		})
	})
}

func (ctx *ctx) delete(args []string) error {
	return ctx.withStore(ctx.flag.Target, func(store archive.Store) error {
		return archive.Delete(ctx.background, store, args[0])
	})
}

func (ctx *ctx) importFn(args []string) error {
	name, importFile := args[0], args[1]
	return ctx.withStore(ctx.flag.Target, func(store archive.Store) error {
		return fetch.Do(ctx.background, []string{importFile}, ctx.flag.Max, false, func(innerCtx context.Context, _ int, f *file.File) error {
			return archive.Import(innerCtx, ctx.logger, store, ctx.flag.Max, name, f)
		})
	})
}

func (ctx *ctx) index(_ []string) error {
	return ctx.withStore(ctx.flag.Target, func(store archive.Store) error {
		index, err := archive.Index(ctx.background, store, ctx.flag.Max)
		if err != nil {
			return err
		}
		for _, line := range index {
			ctx.logger.Stdout.Printf("%s", bytes.TrimRight(line, "\n"))
		}
		return nil
	})
}

func (ctx *ctx) indexUpdate(args []string) error {
	return ctx.withStore(ctx.flag.Target, func(store archive.Store) error {
		var input io.Reader
		var err error
		input = os.Stdin
		fi, _ := os.Stdin.Stat()
		if fi.Size() == 0 && len(args) == 0 {
			return fmt.Errorf("must supply a file or standard in stream")
		}
		if len(args) > 0 && args[0] != "-" {
			input, err = os.Open(args[0])
			if err != nil {
				return err
			}
		}
		return archive.IndexUpdate(ctx.background, ctx.logger, store, ctx.flag.Max, input)
	})
}

func (ctx *ctx) check(args []string) error {
	return ctx.withStore(ctx.flag.Target, func(store archive.Store) error {
		result, err := archive.Check(ctx.background, store, ctx.flag.Max, args[0])
		if err == nil {
			ctx.logger.Stdout.Printf("%s", result)
			return nil
		}
		return err
	})
}

func (ctx *ctx) sync(args []string) error {
	return ctx.withStore(args[1], func(srcStore archive.Store) error {
		return ctx.withStore(args[2], func(destStore archive.Store) error {
			return archive.Sync(ctx.background, ctx.logger, srcStore, destStore, args[0], ctx.flag.Max)
		})
	})
}

func (ctx *ctx) diff(args []string) error {
	return ctx.withStore(args[0], func(srcStore archive.Store) error {
		return ctx.withStore(args[1], func(destStore archive.Store) error {
			return archive.Diff(ctx.background, srcStore, destStore)
		})
	})
}

func (ctx *ctx) withMeta(name string, fn func(*file.File, archive.Store) error) error {
	return ctx.withStore(ctx.flag.Target, func(store archive.Store) error {
		f, err := archive.GetMetaByPrefix(ctx.background, store, name)
		if err != nil {
			return err
		}
		return fn(f, store)
	})
}

func (ctx *ctx) metaGet(args []string) error {
	return ctx.withMeta(args[0], func(f *file.File, _ archive.Store) error {
		ctx.logger.Stdout.Print(f.Meta)
		return nil
	})
}

func (ctx *ctx) metaSet(args []string) error {
	return ctx.withMeta(args[0], func(f *file.File, store archive.Store) error {
		f.Meta.Set(args[1], args[2])
		ctx.logger.Stdout.Print(f.Meta)
		return store.Put(ctx.background, bytes.NewReader(*f.Meta), f.Name, time.Now())
	})
}

func (ctx *ctx) metaDelete(args []string) error {
	return ctx.withMeta(args[0], func(f *file.File, store archive.Store) error {
		f.Meta.Delete(args[1])
		ctx.logger.Stdout.Print(f.Meta)
		return store.Put(ctx.background, bytes.NewReader(*f.Meta), f.Name, time.Now())
	})
}

func (ctx *ctx) lambdaCreate(_ []string) error {
	script, err := lambda.CreateScript(version)
	if err != nil {
		return err
	}
	ctx.logger.Stdout.Print(script)
	return nil
}

func (ctx *ctx) lambdaDelete(_ []string) error {
	script, err := lambda.DeleteScript()
	if err != nil {
		return err
	}
	ctx.logger.Stdout.Print(script)
	return nil
}
