package commands

import (
	"context"
	"github.com/tkellen/memorybox/internal/archive"
	"github.com/tkellen/memorybox/internal/store"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
	"io"
)

// Put persists a requested set of inputs and metadata about them to a Store.
func Put(
	store store.Store,
	hashFn func(source io.Reader) (string, int64, error),
	requests []string,
	concurrency int,
	logger func(format string, v ...interface{}),
	metadata []string,
) error {
	inlineMeta := len(metadata) > 0
	// Memorybox creates and persists a json-encoded metafile automatically
	// alongside any new datafile being "put" into a store. This makes a store
	// nothing more than a flat directory of content-hash-named datafiles, and
	// metafiles that describe them. This work is done concurrently up to a
	// maximum number of in-flight requests controlled by the user.
	//
	// As a result, if a user tries to put a datafile AND its metafile pair into
	// a store in a single call (this should be rare, but using memorybox to
	// move the raw content of one store into another is a valid use case), a
	// race condition can occur where source metafiles could be overwritten by a
	// concurrent goroutine persisting their datafile pair at the same time.
	//
	// This is solved by persisting all metadata files in any single request
	// first. Two instances of memorybox being run at the same time, both
	// copying the contents of one store to another could still suffer from this
	// problem. Seems unlikely...
	//
	// Prepare a channel to receive each data-file that should be persisted to
	// the store. If a meta-file for a data-file exists in the requested set of
	// files to save, the meta-file will be stored before the data-file is sent
	// to this channel.
	newDataFiles := make(chan *archive.File)
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
				f, err := archive.New(hashFn, item)
				if err != nil {
					return err
				}
				if inlineMeta {
					f.MetaSet("", metadata[index])
				}
				name := f.Name()
				// If this item is a meta-file, it will blindly overwrite any
				// existing meta-file because there is currently no way of
				// knowing if it is the latest. If someone is manually moving
				// metafiles it is safe to assume they are fine with this.
				if f.IsMetaDataFile() {
					logger("%s -> %s (metafile detected)", name, name)
					return store.Put(f, name)
				}
				// If this item is a datafile, see if we already stored it once
				// before asking to do it again.
				if store.Exists(name) {
					logger("%s -> %s (skipped, exists)", item, name)
					return nil
				}
				// If this is a new datafile AND we had a metafile for it in
				// the same request, the metafile is already in the store now.
				// It is safe to send the datafile off for persistence now.
				newDataFiles <- f
				return nil
			})
		}
		return nil
	})
	// Listen for newFile data files.
	newFileGroup, newFileCtx := errgroup.WithContext(context.Background())
	newFileGroup.Go(func() error {
		sem := semaphore.NewWeighted(int64(concurrency))
		for item := range newDataFiles {
			if err := sem.Acquire(newFileCtx, 2); err != nil {
				return err
			}
			item := item // https://golang.org/doc/faq#closures_and_goroutines
			// Persist datafile.
			newFileGroup.Go(func() error {
				defer sem.Release(1)
				logger("%s -> %s", item.Source(), item.Name())
				return store.Put(item, item.Name())
			})
			// Also persist a metafile, but only if one doesn't already exist.
			newFileGroup.Go(func() error {
				defer sem.Release(1)
				if !store.Exists(archive.MetaFileName(item.Name())) {
					metaFile := archive.NewMetaFile(item)
					return store.Put(metaFile, metaFile.Name())
				}
				return nil
			})
		}
		return nil
	})
	// Wait for the preprocess step to complete before closing the newDataFile
	// channel.
	err := preprocess.Wait()
	close(newDataFiles)
	if err != nil {
		return err
	}
	// Return when we're done sending datafiles.
	return newFileGroup.Wait()
}
