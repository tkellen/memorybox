package cli

import (
	"errors"
	"fmt"
	"github.com/korovkin/limiter"
	"github.com/tkellen/memorybox/lib"
	"io"
	"strings"
)

// Logger defines a method prototype for logging output.
type Logger func(string, ...interface{})

// Command specifies the execution of an action, typically configured by the
// using the command line interface.
type Command struct {
	Action      string
	Store       memorybox.Store
	Logger      Logger
	Request     string
	Inputs      []string
	Concurrency int
	Reader      func(string) (io.ReadCloser, string, error)
	Writer      func(io.ReadCloser) error
	Cleanup     func() error
}

// Dispatch runs commands as defined from the cli.
func (cmd *Command) Dispatch() error {
	cmd.Logger("Action: %s\nStore: %s", cmd.Action, cmd.Store)
	if cmd.Action == "put" {
		return cmd.putAll()
	}
	if cmd.Action == "get" {
		return cmd.get()
	}
	return errors.New("unrecognized command")
}

// putAll sends all inputs to the backing store.
func (cmd *Command) putAll() error {
	var errs []string
	cmd.Logger("Concurrency: %d\n", cmd.Concurrency)
	if len(cmd.Inputs) == 1 {
		cmd.Logger("Input: %s", cmd.Inputs[0])
	} else {
		cmd.Logger("Inputs:\n  %s", strings.Join(cmd.Inputs, "\n  "))
	}
	// Configure maximum concurrency.
	limit := limiter.NewConcurrencyLimiter(cmd.Concurrency)
	// Iterate over all inputs.
	for _, input := range cmd.Inputs {
		input := input // sad hack to ensure closure below gets right value
		limit.Execute(func() {
			if err := cmd.put(input); err != nil {
				errs = append(errs, err.Error())
			}
		})
	}
	// Wait for all concurrent operations to complete.
	limit.Wait()
	// Clean up any temporary files produced during execution.
	if err := cmd.Cleanup(); err != nil {
		errs = append(errs, err.Error())
	}
	// If we hit errors for any of the inputs, collapse them into a single
	// error for output to the user.
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "\n"))
	}
	return nil
}

// put sends a single user-defined input into the backing store.
func (cmd *Command) put(input string) error {
	// Get io.ReadCloser and content hash for defined input.
	data, digest, err := cmd.Reader(input)
	if err != nil {
		return err
	}
	defer data.Close()
	// Check to see if the file already exists in the store and skip if it does.
	if cmd.Store.Exists(digest) {
		// Close our input since the Store will not be doing it for us.
		cmd.Logger("%s -> %s (skipped, already present)", input, digest)
		return nil
	}
	// Stream file to backing store, using the hash of its content as the name.
	if err = cmd.Store.Put(data, digest); err != nil {
		return fmt.Errorf("%w", err)
	}
	cmd.Logger("%s -> %s", input, digest)
	return nil
}

// get fetches object by key and sends it to the configured writer.
func (cmd *Command) get() error {
	cmd.Logger("Request: %s", cmd.Request)
	matches, err := cmd.Store.Search(cmd.Request)
	if err != nil {
		return fmt.Errorf("search: %w", err)
	}
	if len(matches) != 1 {
		return fmt.Errorf("%d matches", len(matches))
	}
	data, getErr := cmd.Store.Get(matches[0])
	if getErr != nil {
		return getErr
	}
	return cmd.Writer(data)
}
