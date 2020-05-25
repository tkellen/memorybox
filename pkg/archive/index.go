package archive

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/tkellen/memorybox/pkg/file"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
	"io"
	"time"
)

// Index concats all metafiles in the provided store, one per line.
func Index(ctx context.Context, store Store, concurrency int) ([][]byte, error) {
	files, searchErr := store.Search(ctx, "")
	if searchErr != nil {
		return nil, searchErr
	}
	meta, concatErr := store.Concat(ctx, concurrency, files.Meta().Names())
	if concatErr != nil {
		return nil, concatErr
	}
	return meta, nil
}

// IndexUpdate reads a provided reader line by line where each line is expected
// to be the content of a metafile. The data within is persisted to the store
func IndexUpdate(ctx context.Context, logger *Logger, store Store, concurrency int, updates io.Reader) error {
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
			data, err := reader.ReadBytes('\n')
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return err
			}
			currentLine := lineNo // https://golang.org/doc/faq#closures_and_goroutines
			eg.Go(func() error {
				defer sem.Release(1)
				if err := file.ValidateMeta(data); err != nil {
					return fmt.Errorf("line %d: %w", currentLine, err)
				}
				name := file.MetaNameFrom(file.Meta(data).DataFileName())
				if err := file.ValidateMeta(data); err != nil {
					logger.Verbose.Printf("%s updated", name)
				}
				logger.Stdout.Printf("%s", data)
				return store.Put(ctx, bytes.NewBuffer(data), name, time.Now())
			})
		}
		return nil
	})
	return eg.Wait()
}
