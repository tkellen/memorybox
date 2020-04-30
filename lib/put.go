package memorybox

import (
	"context"
	"github.com/tkellen/memorybox/internal/fetch"
	"github.com/tkellen/memorybox/pkg/archive"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
	"log"
	"os"
	"sync"
)

// Put persists any number of requested inputs to a store with concurrency
// control in place.
//
// A note about the complexity of this function:
//
// Memorybox creates and persists a json-encoded metafile automatically
// alongside any new datafile being "put" into a store. This makes a store
// nothing more than a flat directory of content-hash-named datafiles, and
// metafiles that describe them. This work is done concurrently up to a
// maximum number of in-flight requests controlled by the user.
//
// As a result, if a user tries to put a datafile AND its metafile pair into
// a store in a single call (this should be rare, but using memorybox to
// move the raw content of one store into another is a valid use case), a
// race condition can occur where the metafile is overwritten by a concurrent
// goroutine creating one to pair with the datafile.
//
// This is solved by persisting all metadata files in any single request
// last. Two instances of memorybox being run at the same time, both
// copying the contents of one store to another could still suffer from this
// problem. Seems unlikely...
func Put(
	ctx context.Context,
	store Store,
	requests []string,
	concurrency int,
	logger *log.Logger,
	metadata []string,
) error {
	// The import function may send metadata to associate with data being put
	// into the store. Detect if that is happening and make it easy to reason
	// about a bit further down.
	inlineMeta := len(metadata) > 0
	// Prepare a channel to receive any metafiles being persisted to the store.
	// These are queued to be sent last to ensure they trump any automatically
	// generated ones.
	queue := make(chan *archive.File)
	// Preprocess all files as described above.
	preprocess, preprocessCtx := errgroup.WithContext(ctx)
	preprocess.Go(func() error {
		sem := semaphore.NewWeighted(int64(concurrency))
		for index, item := range requests {
			if err := sem.Acquire(preprocessCtx, 1); err != nil {
				return err
			}
			index := index
			item := item // https://golang.org/doc/faq#closures_and_goroutines
			preprocess.Go(func() error {
				defer sem.Release(1)
				data, deleteWhenDone, fetchErr := fetch.Do(ctx, item)
				if fetchErr != nil {
					return fetchErr
				}
				if deleteWhenDone {
					defer os.Remove(data.Name())
				}
				defer data.Close()
				file, err := archive.NewSha256(item, data)
				if err != nil {
					return err
				}
				if inlineMeta {
					file.MetaSet("", metadata[index])
				}
				// Store metadata files that are explicitly being copied using
				// memorybox to prevent data races with them being automatically
				// created by memorybox.
				if file.IsMetaFile() {
					queue <- file
					return nil
				}
				// If this is a datafile, persist it now.
				return putFile(ctx, store, file, logger)
			})
		}
		return nil
	})
	// Collect all metafiles coming out of the preprocess step.
	collector := sync.WaitGroup{}
	collector.Add(1)
	var metaFiles []*archive.File
	go func() {
		defer collector.Done()
		for file := range queue {
			metaFiles = append(metaFiles, file)
		}
	}()
	if err := preprocess.Wait(); err != nil {
		return err
	}
	close(queue)
	// Wait for all metafiles to be collected from the queue.
	collector.Wait()
	// Any datafiles have now been stored. Any metafiles that were put manually
	// should be persisted now/last.
	metaFileGroup, metaFileCtx := errgroup.WithContext(ctx)
	metaFileGroup.Go(func() error {
		sem := semaphore.NewWeighted(int64(concurrency))
		for _, file := range metaFiles {
			file := file // https://golang.org/doc/faq#closures_and_goroutines
			if err := sem.Acquire(metaFileCtx, 1); err != nil {
				return err
			}
			metaFileGroup.Go(func() error {
				defer sem.Release(1)
				return putFile(ctx, store, file, logger)
			})
		}
		return nil
	})
	// Return when we're done sending metafiles.
	return metaFileGroup.Wait()
}

// Put persists an archive.File and its metadata to the provided store.
func putFile(ctx context.Context, store Store, file *archive.File, logger *log.Logger) error {
	// If file is a metafile, blindly write it and signal completion. There is
	// currently no way of knowing if an incoming metafile is newer than the one
	// it might clobber. If someone is manually moving metafiles it is safe to
	// assume they are fine with this.
	if file.IsMetaFile() {
		logger.Printf("%s -> %s (metafile detected)", file.Name(), file.Name())
		return store.Put(ctx, file, file.Name())
	}
	eg := errgroup.Group{}
	// Try to store datafile.
	eg.Go(func() error {
		if store.Exists(ctx, file.Name()) {
			logger.Printf("%s -> %s (skipped, exists)", file.Source(), file.Name())
			return nil
		}
		logger.Printf("%s -> %s", file.Source(), file.Name())
		return store.Put(ctx, file, file.Name())
	})
	// Create metafile silently (only if required).
	eg.Go(func() error {
		metaFile := file.MetaFile()
		if store.Exists(ctx, metaFile.Name()) {
			return nil
		}
		return store.Put(ctx, metaFile, metaFile.Name())
	})
	return eg.Wait()
}
