package cli

import (
	"errors"
	"fmt"
	"github.com/korovkin/limiter"
	"github.com/tkellen/memorybox/lib"
	"io"
	"os"
	"path"
	"strings"
)

// Command describes a request created by using the cli.
type Command struct {
	Action      string
	Request     []string
	Concurrency int
	Logger      func(string, ...interface{})
	Store       memorybox.Store
	// All network and disk IO operations are abstracted in the methods below.
	// This separation should be maintained for ease of testing.
	reader  func(string, string) (io.ReadCloser, string, error)
	writer  func(io.ReadCloser) error
	cleanup func(string) error
	tempDir string
}

// New instantiates a Command with references to methods that interact with the
// network and local disk to complete their operations (these are mocked to unit
// test the public functionality of this package).
func New() *Command {
	return &Command{
		Logger:  func(format string, v ...interface{}) {},
		reader:  inputReader,
		writer:  outputWriter,
		cleanup: wipeDir,
		tempDir: path.Join(os.TempDir(), "mb"),
	}
}

// Dispatch runs commands as defined from the cli.
func (cmd *Command) Dispatch() error {
	cmd.Logger("Action: %s\nStore: %s", cmd.Action, cmd.Store)
	if cmd.Action == "put" {
		return cmd.put()
	}
	if cmd.Action == "get" {
		return cmd.get()
	}
	return errors.New("unrecognized command")
}

// Get fetches a single requested object from a store and sends it to stdout.
func (cmd *Command) get() error {
	cmd.Logger("Request: %s", cmd.Request[0])
	matches, err := cmd.Store.Search(cmd.Request[0])
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
	return cmd.writer(data)
}

// put attempts to persist any number of inputs into a store with a user-defined
// limit on the number of operations that can be run concurrently.
func (cmd *Command) put() error {
	var errs []string
	cmd.Logger("Concurrency: %d\n", cmd.Concurrency)
	if len(cmd.Request) == 1 {
		cmd.Logger("Input: %s", cmd.Request[0])
	} else {
		cmd.Logger("Inputs:\n  %s", strings.Join(cmd.Request, "\n  "))
	}
	// Configure maximum concurrency.
	limit := limiter.NewConcurrencyLimiter(cmd.Concurrency)
	// Iterate over all inputs.
	for _, input := range cmd.Request {
		input := input // sad hack to ensure closure below gets right value
		limit.Execute(func() {
			if err := cmd.putOne(input); err != nil {
				errs = append(errs, err.Error())
			}
		})
	}
	// Wait for all concurrent operations to complete.
	limit.Wait()
	// Clean up any temporary files produced during execution.
	if err := cmd.cleanup(cmd.tempDir); err != nil {
		errs = append(errs, err.Error())
	}
	// If we hit errors for any of the inputs, collapse them into a single
	// error for output to the user.
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "\n"))
	}
	return nil
}

// putOne persists a single input to a store if it is not already present.
func (cmd *Command) putOne(input string) error {
	// Get io.ReadCloser and content hash for defined input.
	data, digest, err := cmd.reader(input, cmd.tempDir)
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
