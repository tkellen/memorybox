package store

import (
	"context"
	"github.com/tkellen/memorybox/internal/archive"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
	"io"
	"log"
)

// Put retrieves and hashes the data at the request string and persists it along
// with a metadata file to the provided backing store.
func Put(store Store, hashFn hashFn, request string, logger *log.Logger) error {
	f, err := archive.New(hashFn, request)
	if err != nil {
		return err
	}
	return PutFile(store, f, logger)
}

// PutFile persists an archive.File and its metadata to the provided store.
func PutFile(store Store, file *archive.File, logger *log.Logger) error {
	eg := errgroup.Group{}
	eg.Go(func() error {
		logger.Printf("%s -> %s", file.Source(), file.Name())
		defer file.Close()
		return store.Put(file, file.Name())
	})
	eg.Go(func() error {
		if !store.Exists(archive.MetaFileName(file.Name())) {
			metaFile := archive.NewMetaFile(file)
			defer metaFile.Close()
			return store.Put(metaFile, metaFile.Name())
		}
		return nil
	})
	return eg.Wait()
}

// PutMany persists any number of requested inputs to a store with concurrency
// control in place.
func PutMany(
	store Store,
	hashFn func(source io.Reader) (string, int64, error),
	requests []string,
	concurrency int,
	logger *log.Logger,
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
					logger.Printf("%s -> %s (metafile detected)", name, name)
					defer f.Close()
					return store.Put(f, name)
				}
				// If this item is a datafile, see if we already stored it once
				// before asking to do it again.
				if store.Exists(name) {
					logger.Printf("%s -> %s (skipped, exists)", item, name)
					defer f.Close()
					return nil
				}
				// If this is a new datafile AND we had a metafile for it in
				// the same request, the metafile is already in the store now.
				// It is safe to send the datafile off for persistence now.
				return PutFile(store, f, logger)
			})
		}
		return nil
	})
	// Listen for newFile data files.
	newFileGroup, newFileCtx := errgroup.WithContext(context.Background())
	newFileGroup.Go(func() error {
		sem := semaphore.NewWeighted(int64(concurrency))
		for item := range newDataFiles {
			if err := sem.Acquire(newFileCtx, 1); err != nil {
				return err
			}
			newFileGroup.Go(func() error {
				defer sem.Release(1)
				return PutFile(store, item, logger)
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
