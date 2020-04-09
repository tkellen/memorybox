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
	// All disk IO operations are abstracted in the members below.
	// This separation should be maintained for ease of testing.
	cleanup func(string) error
	read    func(string, io.ReadCloser, string) (io.ReadCloser, string, error)
	stdin   io.ReadCloser
	stdout  io.WriteCloser
	tempDir string
}

// New instantiates a Command with references to methods that interact with the
// network and local disk to complete their operations.
func New() *Command {
	return &Command{
		Logger:  func(format string, v ...interface{}) {},
		cleanup: os.RemoveAll,
		read:    read,
		stdout:  os.Stdout,
		stdin:   os.Stdin,
		tempDir: path.Join(os.TempDir(), "mb"),
	}
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
	cmd.stdout.Close()
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
	if _, err := io.Copy(cmd.stdout, data); err != nil {
		return err
	}
	return nil
}

// Put persists a single input to a store if it is not already present.
func (cmd *Command) put(input string) error {
	var err error
	var reader io.ReadCloser
	var filepath, digest string
	// Get an io.ReadCloser and the filename on disk for the supplied input. If
	// the input isn't already on disk (putting data from stdin or a url), this
	// method will tee the reader to a file in the temporary directory so it is
	// persisted while we compute the hash.
	if reader, filepath, err = cmd.read(input, cmd.stdin, cmd.tempDir); err != nil {
		return fmt.Errorf("reading: %s", err)
	}
	// Compute a sha256 message digest for the content of our input.
	if digest, err = hash(reader); err != nil {
		return fmt.Errorf("hashing: %s", err)
	}
	// Check to see if the file already exists in the store and skip if it does.
	if cmd.Store.Exists(digest) {
		// Close our input since the Store will not be doing it for us.
		cmd.Logger("%s -> %s (skipped, already present)", input, digest)
		return nil
	}
	// We consumed the the reader computing our hash. Open the backing data
	// file again.
	if reader, filepath, err = cmd.read(filepath, cmd.stdin, cmd.tempDir); err != nil {
		return err
	}
	// Stream file to backing store, using the hash of its content as the name.
	if err = cmd.Store.Put(reader, digest); err != nil {
		return fmt.Errorf("%w", err)
	}
	cmd.Logger("%s -> %s", input, digest)
	return nil
}
