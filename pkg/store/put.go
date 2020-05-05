package store

import (
	"context"
	"errors"
	"github.com/tkellen/memorybox/internal/fetch"
	"github.com/tkellen/memorybox/pkg/archive"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
	"io"
	"log"
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
	store Store,
	requests []string,
	metadata []string,
	concurrency int,
	stderr *log.Logger,
	stdout *log.Logger,
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
	reader := func(ctx context.Context, index int, item string, src io.ReadSeeker) error {
		file, err := archive.NewSha256(item, src)
		if err != nil {
			return err
		}
		// If metadata has been supplied by the user, insert it into the
		// file now.
		if inlineMeta {
			file.MetaSet("", metadata[index])
		}
		// Queue metadata files that are explicitly being copied using
		// memorybox so they can be persisted last.
		if file.IsMetaFile() {
			queue <- file
			return nil
		}
		// If this is a datafile, persist it now.
		return putFile(ctx, store, file, stderr, stdout)
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
				return putFile(ctx, store, file, stderr, stdout)
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
	store Store,
	file *archive.File,
	stderr *log.Logger,
	stdout *log.Logger,
) error {
	// If file is a metafile, blindly write it and signal completion. There is
	// currently no way of knowing if an incoming metafile is newer than the one
	// it might clobber. If someone is manually moving metafiles it is safe to
	// assume they are fine with this.
	if file.IsMetaFile() {
		stderr.Printf("%s -> %s (metafile detected)", file.Name(), file.Name())
		stdout.Printf("%s\n", file.Meta())
		return store.Put(ctx, file, file.Name())
	}
	eg := errgroup.Group{}
	// Try to store datafile.
	eg.Go(func() error {
		if store.Exists(ctx, file.Name()) {
			stderr.Printf("%s -> %s (skipped, exists)", file.Source(), file.Name())
			return nil
		}
		stderr.Printf("%s -> %s", file.Source(), file.Name())
		return store.Put(ctx, file, file.Name())
	})
	// Create a metafile, but only if needed.
	eg.Go(func() error {
		metaFile := file.MetaFile()
		metaFileName := metaFile.Name()
		// See if there is a metafile for this datafile already.
		meta, err := getBytes(ctx, store, metaFileName)
		// If data arrived, there is a metafile already, don't overwrite it.
		if err == nil {
			stderr.Printf("%s -> %s (skipped, exists)", file.Source(), metaFileName)
			stdout.Printf("%s\n", meta)
			return nil
		}
		// If there is a failure but it isn't related to a file not existing,
		// hard fail and stop, something went wrong.
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			stderr.Printf("unable to validate presence of %s", metaFileName)
		}
		// Actually store the metafile now.
		stderr.Printf("%s -> %s (generated)", file.Source(), metaFileName)
		stdout.Printf("%s\n", metaFile.Meta())
		return store.Put(ctx, metaFile, metaFileName)
	})
	return eg.Wait()
}
