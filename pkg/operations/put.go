package operations

import (
	"context"
	"errors"
	"fmt"
	"github.com/tkellen/memorybox/internal/fetch"
	"github.com/tkellen/memorybox/pkg/archive"
	"github.com/tkellen/memorybox/pkg/store"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
	"io"
	"io/ioutil"
	"os"
	"sync"
)

// Put persists any number of requested inputs to a store at a user-defined
// level of concurrency.
//
// A note about the complexity of this function:
//
// Memorybox automatically creates json-encoded metafiles to track user-defined
// metadata for every file in storage. If a user tries to put a datafile AND
// its metafile pair into a store in a single call (this should be rare, but
// using memorybox to move the raw content of one store into another is a valid
// use-case that would cause this), a race condition can occur where a metafile
// is overwritten by a concurrent goroutine storing a datafile.
//
// This is solved by persisting all metadata files in any single request last.
// Two instances of memorybox being run at the same time, both copying the
// content of one store to another could still suffer from this problem. Seems
// unlikely...
func Put(
	ctx context.Context,
	logger *Logger,
	s store.Store,
	concurrency int,
	requests []string,
	metadata []string,
) error {
	// The import function may send metadata to associate with data being put
	// into the store. Detect if that is happening and make it easy to reason
	// about a bit further down.
	inlineMeta := len(metadata) > 0
	// Prepare a channel to receive any metafiles being manually persisted to
	// the store. These are queued to be sent last to ensure they trump any
	// automatically generated ones.
	queue := make(chan *archive.File)
	// Prepare a process function to handle every input.
	reader := func(index int, item string, src io.ReadSeeker) error {
		file, err := archive.NewSha256(item, src)
		if err != nil {
			return err
		}
		// If metadata has been supplied by the user, insert it into the
		// file now.
		if inlineMeta {
			file.MetaSetRaw(metadata[index])
		}
		// Queue metadata files that are explicitly being copied using
		// memorybox so they can be persisted last.
		if file.IsMetaFile() {
			queue <- file
			return nil
		}
		// If this is a datafile, persist it now.
		return putFile(ctx, logger, s, file)
	}
	// Process all inputs that are datafiles using the reader function above.
	datafiles, datafilesCtx := errgroup.WithContext(ctx)
	datafiles.Go(func() error {
		return fetch.Many(datafilesCtx, requests, concurrency, reader)
	})
	// Collect all metafiles coming out of the datafile step.
	collector := sync.WaitGroup{}
	collector.Add(1)
	var metaFiles []*archive.File
	go func() {
		defer collector.Done()
		for file := range queue {
			metaFiles = append(metaFiles, file)
		}
	}()
	if err := datafiles.Wait(); err != nil {
		return err
	}
	close(queue)
	// Wait for all metafiles to be collected from the queue.
	collector.Wait()
	// Any datafiles have now been stored. Now persist any metafiles the user
	// explicitly provided.
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
				return putFile(ctx, logger, s, file)
			})
		}
		return nil
	})
	// Return when we're done sending metafiles.
	return metaFileGroup.Wait()
}

// Put persists an archive.File and its metadata to the provided store.
func putFile(
	ctx context.Context,
	logger *Logger,
	s store.Store,
	file *archive.File,
) error {
	// If file is a metafile, blindly write it and signal completion. There is
	// currently no way of knowing if an incoming metafile is newer than the one
	// it might clobber. If someone is manually moving metafiles it is safe to
	// assume they are fine with this.
	if file.IsMetaFile() {
		logger.Verbose.Printf("%s -> %s (metafile detected)", file.Name(), file.Name())
		logger.Stdout.Printf("%s", file.Meta())
		return s.Put(ctx, file, file.Name())
	}
	eg := errgroup.Group{}
	// Try to store datafile.
	eg.Go(func() error {
		if s.Exists(ctx, file.Name()) {
			logger.Verbose.Printf("%s -> %s (skipped, exists)", file.Source(), file.Name())
			return nil
		}
		logger.Verbose.Printf("%s -> %s", file.Source(), file.Name())
		return s.Put(ctx, file, file.Name())
	})
	// Create a metafile, but only if needed.
	eg.Go(func() error {
		metaFile := file.MetaFile()
		metaFileName := metaFile.Name()
		// If there is no meta file for this datafile already, create it.
		if !s.Exists(ctx, metaFileName) {
			// If here, the meta file was missing, store it now.
			logger.Verbose.Printf("%s -> %s (generated)", file.Source(), metaFileName)
			logger.Stdout.Printf("%s", metaFile.Meta())
			return s.Put(ctx, metaFile, metaFileName)
		}
		// Otherwise, do some validation.
		reader, getErr := s.Get(ctx, metaFileName)
		// If there is a failure but it isn't related to a file not existing,
		// note that something went wrong and continue.
		if getErr != nil && !errors.Is(getErr, os.ErrNotExist) {
			logger.Stderr.Printf("unable to retrieve %s", metaFileName)
		}
		meta, readErr := ioutil.ReadAll(reader)
		if readErr != nil {
			return readErr
		}
		defer reader.Close()
		// This should only happen if a meta file is changed by an external
		// process.
		if !archive.IsMetaData(meta) {
			return store.Corrupted(fmt.Errorf("%s metafile %s missing %s key", meta, metaFileName, archive.MetaKey))
		}
		// If there was no error, output the metadata and stop.
		logger.Verbose.Printf("%s -> %s (skipped, exists)", file.Source(), metaFileName)
		logger.Stdout.Printf("%s", meta)
		return nil
	})
	return eg.Wait()
}
