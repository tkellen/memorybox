package store

import (
	"bufio"
	"context"
	"fmt"
	"github.com/tidwall/gjson"
	"github.com/tkellen/memorybox/internal/fetch"
	"github.com/tkellen/memorybox/pkg/archive"
	"golang.org/x/sync/errgroup"
	"io"
	"log"
	"strings"
	"sync"
)

type importEntry struct {
	Request  string
	Metadata string
}

// Import performs a mass put / annotation operation on any number of manifest
// files, formatted like so:
// ```
// path/to/file.jpg {"title":"some file on my machine"}
// https://images.com/photo.jpg {"title":"some photo on the internet"}
// https://audio.com/audio.mp3 {"title":"some mp3 on the internet"}
// ```
// Import will intelligently de-dupe manifests and remove entries that already
// appear in the store as being sourced from the filepath or URL in the manifest
// file.
func Import(
	ctx context.Context,
	store Store,
	inputs []string,
	concurrency int,
	stderr *log.Logger,
	stdout *log.Logger,
) error {
	imports, err := collectImports(ctx, inputs, concurrency)
	if err != nil {
		return err
	}
	// Get all metadata entries from the store.
	index, indexErr := index(ctx, store, concurrency, false, stderr)
	if indexErr != nil {
		return indexErr
	}
	// Build index of every existing "source" file used in the store.
	sourceIndex := map[string]bool{}
	for _, entry := range index {
		source := gjson.GetBytes(entry, archive.MetaKey+".source").String()
		sourceIndex[source] = true
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
	stdout.Printf("queued: %d, duplicates removed: %d, existing removed: %d", len(putRequests), dupeImportCount, inStoreAlreadyCount)
	return Put(ctx, store, putRequests, putMetadatas, concurrency, stderr, stdout)
}

// collectImports reads all input files supplied to the import function
// concurrently, aggregating them into an array of ImportEntries.
func collectImports(ctx context.Context, requests []string, concurrency int) ([]importEntry, error) {
	// Start a collector goroutine to receive all entries.
	entries := make(chan importEntry)
	// Prepare handler to read every import file concurrently.
	reader := func(ctx context.Context, index int, item string, src io.ReadSeeker) error {
		scanner := bufio.NewScanner(src)
		for scanner.Scan() {
			fields := strings.SplitN(scanner.Text(), "\t", 2)
			// Allow import lines with no metadata.
			if len(fields) < 2 {
				fields = append(fields, "")
			}
			entries <- importEntry{
				Request:  fields[0],
				Metadata: fields[1],
			}
		}
		return nil
	}
	// Process all import files using the reader function above.
	process, processCtx := errgroup.WithContext(ctx)
	process.Go(func() error {
		return fetch.Many(processCtx, requests, concurrency, reader)
	})
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
