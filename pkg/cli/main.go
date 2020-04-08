package cli

import (
	"fmt"
	"github.com/korovkin/limiter"
	"github.com/tkellen/memorybox/pkg"
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
	Index       memorybox.Index
	// All network and disk IO operations are abstracted in the members below.
	// This separation should be maintained for ease of testing.
	source  func(string, string) (io.ReadCloser, string, error)
	sink    io.WriteCloser
	cleanup func(string) error
	tempDir string
}

// New instantiates a Command with references to methods that interact with the
// network and local disk to complete their operations.
func New() *Command {
	return &Command{
		Logger:  func(format string, v ...interface{}) {},
		source:  inputReader,
		sink:    os.Stdout,
		cleanup: wipeDir,
		tempDir: path.Join(os.TempDir(), "mb"),
	}
}

// aggregateErrors does just what it sounds like.
func aggregateErrorStrings(err error, errs []string) []string {
	if err != nil {
		return append(errs, err.Error())
	}
	return errs
}

// Dispatch runs commands as defined from the cli.
func (cmd *Command) Dispatch() error {
	var errs []string
	var executor func(string) error
	cmd.Logger("Action: %s\nStore: %s", cmd.Action, cmd.Store)
	cmd.Logger("Concurrency: %d\n", cmd.Concurrency)
	if cmd.Action == "put" {
		executor = cmd.put
	}
	if cmd.Action == "get" {
		executor = cmd.get
	}
	if executor == nil {
		return fmt.Errorf("unknown action: %s", cmd.Action)
	}
	// Configure maximum concurrency.
	limit := limiter.NewConcurrencyLimiter(cmd.Concurrency)
	// Iterate over all inputs.
	for _, item := range cmd.Request {
		request := item // ensure closure below gets right value
		limit.Execute(func() {
			errs = aggregateErrorStrings(executor(request), errs)
		})
	}
	// Wait for all concurrent operations to complete.
	limit.Wait()
	// Clean up any temporary files produced during execution.
	errs = aggregateErrorStrings(cmd.cleanup(cmd.tempDir), errs)
	// Collapse any errors into a single error for output to the user.
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "\n"))
	}
	return nil
}

// Get fetches a single requested object from a store and copies it to a
// provided sink.
func (cmd *Command) get(request string) error {
	cmd.Logger("Request: %s", request)
	if cmd.Index != nil {
		matches, searchErr := cmd.Index.Search(request)
		if searchErr != nil {
			return fmt.Errorf("search: %w", searchErr)
		}
		if len(matches) != 1 {
			return fmt.Errorf("%d matches", len(matches))
		}
		request = matches[0]
	}
	data, err := cmd.Store.Get(request)
	if err != nil {
		return err
	}
	// TODO: support writing to disk
	if _, err := io.Copy(cmd.sink, data); err != nil {
		return err
	}
	cmd.sink.Close()
	return nil
}

// put persists a single input to a store if it is not already present.
func (cmd *Command) put(input string) error {
	// Get io.ReadCloser and content hash for defined input.
	data, digest, err := cmd.source(input, cmd.tempDir)
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
