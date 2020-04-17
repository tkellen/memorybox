// Package put is a high level abstraction over store.Store for persisting
// content. It is designed to execute operations that are supported by the cli.
package put

import (
	"fmt"
	"github.com/korovkin/limiter"
	"github.com/tkellen/memorybox/pkg/hashreader"
	"github.com/tkellen/memorybox/pkg/store"
	"strings"
)

// Logger is just what you think it is.
type Logger func(format string, v ...interface{})

// Commands defines the interface needed to expose this functionality in a way
// that can be mocked for testing by consumers.
type Commands interface {
	Main(store.Store, []string, int, Logger, string) error
}

// Command implements Commands as the public API of this package.
type Command struct{}

// Main takes an array of inputs and persists them to a Store using a hash of
// their contents as the identifier of the input.
func (Command) Main(store store.Store, input []string, concurrency int, logger Logger, tempPath string) error {
	return limitRunner(func(request string) error {
		reader, digest, err := hashreader.HashReader(request, tempPath)
		defer reader.Close()
		if err != nil {
			return err
		}
		if store.Exists(digest) {
			logger("%s -> %s (skipped, already exists)", request, digest)
			return nil
		}
		logger("%s -> %s", request, digest)
		return store.Put(reader, digest)
	}, input, concurrency)
}

func limitRunner(fn func(string) error, requests []string, max int) error {
	var errs []string
	limit := limiter.NewConcurrencyLimiter(max)
	// Iterate over all inputs.
	for _, item := range requests {
		request := item // ensure closure below gets right value
		limit.Execute(func() {
			if err := fn(request); err != nil {
				errs = append(errs, err.Error())
			}
		})
	}
	// Wait for all concurrent operations to complete.
	limit.Wait()
	// Collapse any errors into a single error for output to the user.
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "\n"))
	}
	return nil
}
