package operations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/tkellen/memorybox/pkg/archive"
	"github.com/tkellen/memorybox/pkg/store"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"sync"
)

// Logger defines output streams for operations.
type Logger struct {
	Stdout  *log.Logger
	Stderr  *log.Logger
	Verbose *log.Logger
}

// findOne determines if there is a single item in the provided store with the
// request prefix and returns the full name.
func findOne(ctx context.Context, s store.Store, request string) (string, error) {
	matches, searchErr := s.Search(ctx, request)
	if searchErr != nil {
		return "", fmt.Errorf("get: %w", searchErr)
	}
	if len(matches) != 1 {
		return "", fmt.Errorf("%w: %d objects matched", os.ErrNotExist, len(matches))
	}
	return matches[0], nil
}

// index performs an integrity check on a provided store and returns a map keyed
// by datafile names where the value is a byte array containing the content of
// their metafile pair.
func index(ctx context.Context, logger *Logger, s store.Store, concurrency int, rehash bool) ([]json.RawMessage, error) {
	// Prepare index to receive data.
	var data []json.RawMessage
	// Confirm there is one metafile for every datafile and bail out early if
	// there isn't so consumers can fix.
	datafiles, metafiles, err := collate(ctx, s)
	if err != nil {
		return nil, err
	}
	logger.Verbose.Printf("indexing datafiles/metafiles (%d/%d) ", len(datafiles), len(metafiles))
	// Create channel to receive index entries
	entries := make(chan json.RawMessage)
	// Start concurrency gated iteration of all files in store.
	indexer, indexerCtx := errgroup.WithContext(ctx)
	indexer.Go(func() error {
		sem := semaphore.NewWeighted(int64(concurrency))
		for item := range datafiles {
			if err := sem.Acquire(indexerCtx, 1); err != nil {
				return err
			}
			item := item
			indexer.Go(func() error {
				defer sem.Release(1)
				entry, err := indexItem(ctx, s, item, rehash)
				if err != nil {
					return err
				}
				entries <- entry
				return nil
			})
		}
		return nil
	})
	collector := sync.WaitGroup{}
	collector.Add(1)
	go func() {
		defer collector.Done()
		for item := range entries {
			data = append(data, item)
		}
	}()
	// Wait for indexing to finish.
	if err := indexer.Wait(); err != nil {
		return nil, err
	}
	// Close entries channel once processing is completed.
	close(entries)
	// Wait for collector to finish collating the results.
	collector.Wait()
	return data, nil
}

// indexItem performs optional integrity checking on datafiles, mandatory
// integrity checking on their associated metafiles (simply because it is fast
// and easy to do so) and then returns the content of the metafile as a
// json.RawMessage.
func indexItem(ctx context.Context, s store.Store, name string, rehash bool) (json.RawMessage, error) {
	hash := archive.HasherFromFileName(name)
	if rehash {
		reader, err := s.Get(ctx, name)
		if err != nil {
			return nil, err
		}
		digest, _, hashErr := hash(reader)
		if hashErr != nil {
			return nil, hashErr
		}
		reader.Close()
		if name != digest {
			return nil, store.Corrupted(fmt.Errorf("%s should be named %s, possible data corruption", name, digest))
		}
	}
	metaFileName := archive.MetaFileNameFrom(name)
	reader, err := s.Get(ctx, metaFileName)
	if err != nil {
		return nil, err
	}
	content, readErr := ioutil.ReadAll(reader)
	if readErr != nil {
		return nil, readErr
	}
	reader.Close()
	dataFileName := archive.DataFileNameFromMeta(content)
	if dataFileName != name {
		return nil, store.Corrupted(fmt.Errorf("%s key in %s conflicts with filename", archive.MetaKeyFileName, metaFileName))
	}
	return content, nil
}

// collate produces two maps, one containing an entry for every datafile and
// another for every metafile in the provided store. This includes integrity
// checking. If a metafile doesn't have a corresponding datafile or vise versa,
// this will fail.
func collate(ctx context.Context, s store.Store) (datafiles map[string]struct{}, metafiles map[string]struct{}, err error) {
	datafiles = map[string]struct{}{}
	metafiles = map[string]struct{}{}
	files, searchErr := s.Search(ctx, "")
	if searchErr != nil {
		return nil, nil, searchErr
	}
	for _, file := range files {
		if archive.IsMetaFileName(file) {
			metafiles[file] = struct{}{}
			continue
		}
		datafiles[file] = struct{}{}
	}
	var missing []string
	for item := range datafiles {
		pair := archive.MetaFileNameFrom(item)
		if _, ok := metafiles[pair]; !ok {
			missing = append(missing, fmt.Sprintf("datafile %s missing metafile %s", item, pair))
		}
	}
	for item := range metafiles {
		pair := archive.DataFileNameFrom(item)
		if _, ok := datafiles[pair]; !ok {
			missing = append(missing, fmt.Sprintf("metafile %s missing datafile %s", item, pair))
		}
	}
	if len(missing) != 0 {
		err = store.Corrupted(errors.New(strings.Join(missing, "\n")))
	}
	return datafiles, metafiles, err
}
