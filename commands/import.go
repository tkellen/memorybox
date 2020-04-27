package commands

import (
	"bufio"
	"github.com/tkellen/memorybox/internal/store"
	"io"
	"os"
	"strings"
)

// Import assumes all input files contain a newline separated list of items to
// "put". Each line can contain anything that would be valid to specify when
// using put directly.
func Import(
	store store.Store,
	hashFn func(source io.Reader) (string, int64, error),
	imports []string,
	concurrency int,
	logger func(format string, v ...interface{}),
) error {
	var requests []string
	var metadata []string
	for _, item := range imports {
		file, err := os.Open(item)
		if err != nil {
			return err
		}
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			fields := strings.SplitN(scanner.Text(), " ", 2)
			requests = append(requests, fields[0])
			metadata = append(metadata, fields[1])
		}
	}
	return Put(store, hashFn, requests, concurrency, logger, metadata)
}
