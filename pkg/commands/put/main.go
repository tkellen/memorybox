// Package put is a high level abstraction over store.Store for persisting
// content. It is designed to execute operations that are supported by the cli
// but could be trivially expanded to support other use-cases (such as being
// the work performed during a http request if a memorybox http server were ever
// built).
package put

import (
	"context"
	"github.com/tkellen/memorybox/pkg/file"
	"github.com/tkellen/memorybox/pkg/store"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
)

// Commands defines the interface needed to expose this functionality in a way
// that can be mocked for testing by consumers.
type Commands interface {
	Main(store.Store, []string) error
}

// Command implements Commands as the public API of this package.
type Command struct {
	Concurrency int
	Logger      func(format string, v ...interface{})
}

// Main persists a requested set of inputs and metadata about them to a Store.
func (c Command) Main(store store.Store, requests []string) error {
	// Memorybox creates and persists a json-encoded meta-file automatically
	// alongside any new data-file being "put" into a store. This makes a store
	// nothing more than a flat directory of content-hash-named data-files, and
	// meta-files that describe them. This work is done concurrently up to a
	// maximum number of in-flight requests controlled by the user.
	//
	// As a result, if a user tries to put a data-file AND its meta-file pair
	// into a store in a single call (this should be rare, but using memorybox
	// to move the raw content of one store into another would do this), a race
	// condition can occur where the source meta-files can be overwritten by a
	// concurrent goroutine which is persisting the data-file at the same time
	// (it will incorrectly think it needs to create a new meta-file because it
	// does not yet see that one exists).
	//
	// This is solved by persisting all metadata files in a request first. Two
	// instances of memorybox being run at the same time, both copying the
	// contents of one store to another could still suffer from this problem.
	// Seems unlikely...

	// Prepare a channel to receive each data-file that should be persisted to
	// the store. If a meta-file for a data-file exists in the requested set of
	// files to save, the meta-file will be stored before the data-file is sent
	// to this channel.
	newDataFiles := make(chan *file.File)

	// Preprocess all files as described above.
	preprocess, preprocessCtx := errgroup.WithContext(context.Background())
	preprocess.Go(func() error {
		sem := semaphore.NewWeighted(int64(c.Concurrency))
		for _, item := range requests {
			if err := sem.Acquire(preprocessCtx, 1); err != nil {
				return err
			}
			item := item // https://golang.org/doc/faq#closures_and_goroutines
			preprocess.Go(func() error {
				defer sem.Release(1)
				f, err := file.New().Load(item)
				if err != nil {
					return err
				}
				name := f.Name()
				// If this item is a meta-file, it will blindly overwrite any
				// existing meta-file because there is currently no way of
				// knowing if it is the latest. If someone is manually moving
				// meta-files it is safe to assume they are fine with this.
				if f.IsMetaFile() {
					c.Logger("%s -> %s (metafile detected)", name, name)
					return store.Put(f, name)
				}
				// If this item is a data-file, see if we already stored it once
				// before asking to do it again.
				if store.Exists(name) {
					c.Logger("%s -> %s (skipped, exists)", item, name)
					return nil
				}
				// If this is a new data-file AND we had a meta-file for it in
				// the same request, the meta-file is already in the store now.
				// It is safe to send the data-file off for persistence now.
				newDataFiles <- f
				return nil
			})
		}
		return nil
	})

	// Listen for new data files.
	newFileGroup, newFileCtx := errgroup.WithContext(context.Background())
	newFileGroup.Go(func() error {
		sem := semaphore.NewWeighted(int64(c.Concurrency))
		for item := range newDataFiles {
			if err := sem.Acquire(newFileCtx, 2); err != nil {
				return err
			}
			item := item // https://golang.org/doc/faq#closures_and_goroutines
			// Persist data-file.
			newFileGroup.Go(func() error {
				defer sem.Release(1)
				c.Logger("%s -> %s", item.Source(), item.Name())
				return store.Put(item, item.Name())
			})
			// Also persist a meta-file, but only if one doesn't already exist.
			newFileGroup.Go(func() error {
				defer sem.Release(1)
				metaFile := item.NewMetaFile()
				if !store.Exists(metaFile.Name()) {
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
	// Return when we're done sending data-files.
	return newFileGroup.Wait()
}
