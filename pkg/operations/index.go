package operations

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"github.com/mattetti/filebuffer"
	"github.com/tidwall/gjson"
	"github.com/tkellen/memorybox/pkg/archive"
	"github.com/tkellen/memorybox/pkg/store"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
	"io"
)

// Index dumps the output of all meta files to a provided sink, each meta file
// is delimited by a newline. This allows streaming modifications when piped
// back to IndexUpdate.
func Index(ctx context.Context, logger *Logger, s store.Store, concurrency int, rehash bool) error {
	data, indexErr := index(ctx, logger, s, concurrency, rehash)
	if indexErr != nil {
		return fmt.Errorf("indexing: %w", indexErr)
	}
	for _, item := range data {
		logger.Stdout.Printf("%s", item)
	}
	return nil
}

// IndexUpdate updates metafiles.
func IndexUpdate(ctx context.Context, logger *Logger, s store.Store, concurrency int, updates io.Reader) error {
	reader := bufio.NewReader(updates)
	eg, egCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		sem := semaphore.NewWeighted(int64(concurrency))
		lineNo := 0
		for {
			lineNo = lineNo + 1
			if err := sem.Acquire(egCtx, 1); err != nil {
				return err
			}
			meta, err := reader.ReadBytes('\n')
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return err
			}
			currentLine := lineNo // https://golang.org/doc/faq#closures_and_goroutines
			eg.Go(func() error {
				defer sem.Release(1)
				file, err := archive.NewSha256("test", filebuffer.New(meta))
				if err != nil {
					return err
				}
				if err := validateMeta(egCtx, s, meta); err != nil {
					return fmt.Errorf("line %d: %w", currentLine, err)
				}
				return putFile(ctx, logger, s, file)
			})
		}
		return nil
	})
	return eg.Wait()
}

func validateMeta(ctx context.Context, s store.Store, data []byte) error {
	sizeCheck := len(data) - archive.MetaFileMaxSize
	if sizeCheck > 0 {
		return fmt.Errorf("exceeds maximum meta size (%d) by %d bytes", archive.MetaFileMaxSize, sizeCheck)
	}
	dataFileName := gjson.GetBytes(data, archive.MetaKeyFileName).String()
	if dataFileName == "" {
		return fmt.Errorf("no data file reference at key %s", archive.MetaKeyFileName)
	}
	if !s.Exists(ctx, dataFileName) {
		return fmt.Errorf("unable to locate data file %s", dataFileName)
	}
	return nil
}
