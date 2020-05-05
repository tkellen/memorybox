package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/tkellen/memorybox/pkg/archive"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
	"io/ioutil"
	"log"
	"strings"
	"sync"
)

type jsonIndex struct {
	Memorybox []json.RawMessage `json:"memorybox"`
}

// Index dumps the output of all meta files to a provided sink, each meta file
// is delimited by a newline. This allows streaming modifications.
// ```
// {
//   "memorybox": [
//     {
//       "memorybox": {
//         "file": "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08-sha256",
//         "source": "https://awebsite.com/file"
//       },
//       "data": {
//         "arbitrary-key": "user-defined-value"
//       }
//     },
//     ...
//   ]
// }
// ```
func Index(
	ctx context.Context,
	store Store,
	concurrency int,
	integrityChecking bool,
	stderr *log.Logger,
	stdout *log.Logger,
) error {
	data, indexErr := index(ctx, store, concurrency, integrityChecking, stderr)
	if indexErr != nil {
		return indexErr
	}
	for _, line := range data {
		stdout.Writer().Write(append(line, "\r\n"...))
	}
	return nil
}

// index performs an integrity check on the store and returns a map keyed by
// datafile names where the value is a byte array containing the content of
// their metafile pair.
func index(ctx context.Context, store Store, concurrency int, integrityChecking bool, stderr *log.Logger) ([]json.RawMessage, error) {
	// Prepare index to receive data.
	var data []json.RawMessage
	// Confirm there is one metafile for every datafile and bail out early if
	// there isn't so consumers can fix.
	datafiles, metafiles, err := collateStore(ctx, store)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", errCorrupted, err)
	}
	stderr.Printf("indexing datafiles/metafiles (%d/%d) ", len(datafiles), len(metafiles))
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
				entry, err := indexItem(ctx, store, item, integrityChecking)
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
func indexItem(ctx context.Context, store Store, name string, integrityChecking bool) (json.RawMessage, error) {
	hash := archive.HasherFromFileName(name)
	if integrityChecking {
		reader, err := store.Get(ctx, name)
		if err != nil {
			return nil, err
		}
		digest, _, hashErr := hash(reader)
		if hashErr != nil {
			return nil, hashErr
		}
		reader.Close()
		if name != digest {
			return nil, fmt.Errorf("%w: %s should be named %s, possible data corruption", errCorrupted, name, digest)
		}
	}
	metaFileName := archive.MetaFileNameFrom(name)
	reader, err := store.Get(ctx, metaFileName)
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
		return nil, fmt.Errorf("%w: %s key in %s conflicts with filename", errCorrupted, archive.MetaKeyFileName, metaFileName)
	}
	return content, nil
}

// collateStore produces two maps, one containing an entry for every datafile
// and another for every metafile. This process includes integrity checking. If
// any metafile doesn't have a corresponding datafile or vise versa, this will
// fail.
func collateStore(ctx context.Context, store Store) (datafiles map[string]struct{}, metafiles map[string]struct{}, err error) {
	files, _ := store.Search(ctx, "")
	datafiles = map[string]struct{}{}
	metafiles = map[string]struct{}{}
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
		err = errors.New(strings.Join(missing, "\n"))
	}
	return datafiles, metafiles, err
}
