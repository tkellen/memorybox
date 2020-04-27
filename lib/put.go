package memorybox

import (
	"context"
	"github.com/tkellen/memorybox/internal/archive"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
	"io"
	"log"
	"sync"
)

// PutMany persists any number of requested inputs to a store with concurrency
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
func PutMany(
	store Store,
	hashFn func(source io.Reader) (string, int64, error),
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
	preprocess, preprocessCtx := errgroup.WithContext(context.Background())
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
				file, err := archive.New(hashFn, item)
				if err != nil {
					return err
				}
				if inlineMeta {
					file.MetaSet("", metadata[index])
				}
				// Store metadata files first always.
				if file.IsMetaDataFile() {
					queue <- file
					return nil
				}
				// If this is a datafile, persist it.
				return Put(store, file, logger)
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
	// Wait for all datafiles to be collected from the queue.
	collector.Wait()
	// Any metadata files have now been stored. All files that remain are
	// datafiles and should be persisted.
	metaFileGroup, metaFileCtx := errgroup.WithContext(context.Background())
	metaFileGroup.Go(func() error {
		sem := semaphore.NewWeighted(int64(concurrency))
		for _, file := range metaFiles {
			file := file // https://golang.org/doc/faq#closures_and_goroutines
			if err := sem.Acquire(metaFileCtx, 1); err != nil {
				return err
			}
			metaFileGroup.Go(func() error {
				defer sem.Release(1)
				return Put(store, file, logger)
			})
		}
		return nil
	})
	// Return when we're done sending metafiles.
	return metaFileGroup.Wait()
}

// Put persists an archive.File and its metadata to the provided store.
func Put(store Store, file *archive.File, logger *log.Logger) error {
	// Always close files to ensure the temporary backing files are cleaned.
	defer file.Close()
	// If file is a metafile, blindly write it and signal completion. There is
	// currently no way of knowing if an incoming metafile is newer than the one
	// it might clobber. If someone is manually moving metafiles it is safe to
	// assume they are fine with this.
	if file.IsMetaDataFile() {
		logger.Printf("%s -> %s (metafile detected)", file.Name(), file.Name())
		return store.Put(file, file.Name())
	}
	eg := errgroup.Group{}
	// Try to store datafile.
	eg.Go(func() error {
		if store.Exists(file.Name()) {
			logger.Printf("%s -> %s (skipped, exists)", file.Source(), file.Name())
			return nil
		}
		logger.Printf("%s -> %s", file.Source(), file.Name())
		return store.Put(file, file.Name())
	})
	// Create metafile silently (only if required).
	eg.Go(func() error {
		metaFile := archive.NewMetaFile(file)
		defer metaFile.Close()
		if store.Exists(metaFile.Name()) {
			return nil
		}
		return store.Put(metaFile, metaFile.Name())
	})
	return eg.Wait()
}
