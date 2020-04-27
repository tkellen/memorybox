package memorybox

import (
	"bufio"
	"fmt"
	"github.com/tidwall/gjson"
	"github.com/tkellen/memorybox/internal/archive"
	"golang.org/x/sync/errgroup"
	"log"
	"os"
	"strings"
	"sync"
)

type importEntry struct {
	Request  string
	Metadata string
}

// Import performs a mass put / annotation operation on any number of input
// manifest files formatted like so:
// ```
// path/to/file.jpg {"title":"some file on my machine"}
// https://images.com/photo.jpg {"title":"some photo on the internet"}
// https://images.com/audio.mp3 {"title":"some mp3 on the internet"}
// ```
// It will intelligently de-dupe manifests and remove entries that already
// appear in the store.
func Import(
	store Store,
	hashFn hashFn,
	requests []string,
	concurrency int,
	logger *log.Logger,
) error {
	// Get all metadata entries from the store.
	index, indexErr := Index(store)
	if indexErr != nil {
		return indexErr
	}
	// Build index of every existing "source" file used in the store.
	sourceIndex := map[string]bool{}
	for _, entry := range index {
		source := gjson.GetBytes(entry, archive.MetaKey+".source").String()
		sourceIndex[source] = true
	}
	// Read all import files concurrently.
	imports, collectErr := collectImports(requests)
	if collectErr != nil {
		return collectErr
	}
	// Filter duplicate import lines / any with a source that exists already.
	var putRequests []string
	var putMetadatas []string
	importIndex := map[string]importEntry{}
	dupeImportCount := 0
	inStoreAlreadyCount := 0
	for _, entry := range imports {
		// Skip items that appear in the store already.
		if _, ok := sourceIndex[entry.Request]; ok {
			inStoreAlreadyCount = inStoreAlreadyCount + 1
			continue
		}
		// De-dupe import lines that appear more than once.
		if match, ok := importIndex[entry.Request]; ok {
			// Fail if two imports have different metadata.
			if match.Metadata != entry.Metadata {
				return fmt.Errorf("%s duplicate found with differing metadata: %s vs %s", entry.Request, entry.Metadata, match.Metadata)
			}
			dupeImportCount = dupeImportCount + 1
			continue
		}
		importIndex[entry.Request] = entry
		putRequests = append(putRequests, entry.Request)
		putMetadatas = append(putMetadatas, entry.Metadata)
	}
	logger.Printf("queued: %d, duplicates removed: %d, existing removed: %d", len(putRequests), dupeImportCount, inStoreAlreadyCount)
	return PutMany(store, hashFn, putRequests, concurrency, logger, putMetadatas)
}

// collectImports reads all input files supplied to the import function
// concurrently, aggregating them into a map keyed by their request
// string.
func collectImports(requests []string) ([]importEntry, error) {
	// Start a collector goroutine to receive all entries.
	entries := make(chan importEntry)
	// Process every import file concurrently.
	process := errgroup.Group{}
	for _, item := range requests {
		process.Go(func() error {
			file, err := os.Open(item)
			if err != nil {
				return err
			}
			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				fields := strings.SplitN(scanner.Text(), " ", 2)
				entries <- importEntry{
					Request:  fields[0],
					Metadata: fields[1],
				}
			}
			return nil
		})
	}
	// Start listening on the entries channel to aggregate results.
	var imports []importEntry
	collector := sync.WaitGroup{}
	collector.Add(1)
	go func() {
		defer collector.Done()
		for item := range entries {
			imports = append(imports, item)
		}
	}()
	// Wait for import files to finish being processed.
	if err := process.Wait(); err != nil {
		return nil, err
	}
	// Close entries channel once processing is completed.
	close(entries)
	// Wait for collector to finish collating the results.
	collector.Wait()
	return imports, nil
}
