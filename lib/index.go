package memorybox

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/tidwall/gjson"
	"github.com/tkellen/memorybox/internal/archive"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
	"io"
	"io/ioutil"
	"log"
	"sort"
	"strings"
	"sync"
)

type jsonIndex struct {
	Memorybox []json.RawMessage `json:"memorybox"`
}

// Index writes a json-formatted index to the provided sink in the form of:
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
func Index(ctx context.Context, store Store, concurrency int, logger *log.Logger, sink io.Writer) error {
	data, indexErr := index(ctx, store, concurrency, logger, false)
	if indexErr != nil {
		return indexErr
	}
	result, err := json.Marshal(jsonIndex{Memorybox: data})
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(sink, bytes.NewBuffer(result))
	if copyErr != nil {
		return copyErr
	}
	return nil
}

// index performs an integrity check on the store and returns a map keyed by
// datafile names where the value is a byte array containing the content of
// their metafile pair.
func index(ctx context.Context, store Store, concurrency int, logger *log.Logger, integrityChecking bool) ([]json.RawMessage, error) {
	// Prepare index to receive data.
	var data []json.RawMessage
	// Confirm there is one metafile for every datafile and bail out early if
	// there isn't so consumers can fix.
	datafiles, metafiles, err := collateStore(ctx, store)
	if err != nil {
		return nil, err
	}
	logger.Printf("indexing datafiles/metafiles (%d/%d) ", len(datafiles), len(metafiles))
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

// indexItem extracts metadata for the supplied data file. This process includes
// an optional integrity check where the data file is re-hashed to confirm it
// has not become corrupted.
func indexItem(ctx context.Context, store Store, name string, integrityChecking bool) (json.RawMessage, error) {
	if integrityChecking {
		dataReader, dataErr := store.Get(ctx, name)
		if dataErr != nil {
			return nil, dataErr
		}
		digest, _, hashErr := Sha256(dataReader)
		if hashErr != nil {
			return nil, hashErr
		}
		dataReader.Close()
		if name != digest {
			return nil, fmt.Errorf("%s should be named %s, possible data corruption", name, digest)
		}
	}
	metaName := archive.ToMetaFileName(name)
	metaReader, metaErr := store.Get(ctx, metaName)
	if metaErr != nil {
		return nil, metaErr
	}
	content, readErr := ioutil.ReadAll(metaReader)
	if readErr != nil {
		return nil, readErr
	}
	metaReader.Close()
	fileKey := archive.MetaKey + ".file"
	dataFileInContent := gjson.GetBytes(content, fileKey).String()
	dataFileInName := archive.ToDataFileName(name)
	if dataFileInName != dataFileInContent {
		return nil, fmt.Errorf("%s's key %s points %s (conflicts with metafile name)", name, fileKey, dataFileInContent)
	}
	return content, nil
}

// collateStore produces two maps, one containing an entry for every datafile
// and another for every metafile. This process includes integrity checking. If
// any metafile doesn't have a corresponding datafile or vise versa, this will
// fail.
func collateStore(ctx context.Context, store Store) (datafiles map[string]struct{}, metafiles map[string]struct{}, err error) {
	files, _ := store.Search(ctx, "*")
	sort.Strings(files)
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
		pair := archive.ToMetaFileName(item)
		if _, ok := metafiles[pair]; !ok {
			missing = append(missing, fmt.Sprintf("datafile %s missing metafile %s", item, pair))
		}
	}
	for item := range metafiles {
		pair := archive.ToDataFileName(item)
		if _, ok := datafiles[pair]; !ok {
			missing = append(missing, fmt.Sprintf("metafile %s missing datafile %s", item, pair))
		}
	}
	if len(missing) != 0 {
		err = errors.New(strings.Join(missing, "\n"))
	}
	return datafiles, metafiles, err
}
