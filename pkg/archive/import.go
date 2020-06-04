package archive

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"github.com/tkellen/memorybox/internal/fetch"
	"github.com/tkellen/memorybox/pkg/file"
	"io"
	"os"
	"strings"
)

type importEntry struct {
	Request  string
	Metadata string
}

// Import performs a mass put / annotation operation on any number of manifest
// files, formatted like so:
// ```
// path/to/file.jpg	{"title":"some file on my machine"}
// https://images.com/photo.jpg	{"title":"some photo on the internet"}
// https://audio.com/audio.mp3 {"title":"some mp3 on the internet"}
// ```
// Import will intelligently de-dupe manifests. It will also remove entries that
// already appear in the store (by checking every import line against every
// metafile `memorybox.import.source` key in the store).
func Import(ctx context.Context, logger *Logger, store Store, concurrency int, set string, data io.Reader) error {
	// Get full file listing from the store.
	files, searchErr := store.Search(ctx, "")
	if searchErr != nil {
		return fmt.Errorf("listing files: %w", searchErr)
	}
	// Get listing of metafiles that have datafile pairs.
	metaFiles := files.Valid().Meta()
	// Get content of every valid metafile so their sources can be inspected.
	meta, concatErr := store.Concat(ctx, concurrency, metaFiles.Names())
	if concatErr != nil {
		return fmt.Errorf("listing files: %w", concatErr)
	}
	// Determine the original source of all valid files in the store so we can
	// use it to prevent duplicate fetches.
	existing := map[string]struct{}{}
	for index := range metaFiles {
		existing[file.Meta(meta[index]).Source()] = struct{}{}
	}
	// Process all requested imports to a structured format.
	var lines [][]string
	scanner := bufio.NewScanner(data)
	for scanner.Scan() {
		fields := strings.SplitN(scanner.Text(), " ", 2)
		// Normalize import lines with no metadata.
		if len(fields) < 2 {
			fields = append(fields, "")
		}
		lines = append(lines, fields)
	}
	// Filter duplicate import lines / any with a source that exists already.
	var requests []string
	var metadata []string
	seen := map[string][]string{}
	dupeImportCount := 0
	inStoreAlreadyCount := 0
	for _, line := range lines {
		// Skip items that appear in the store as being imported by this source.
		if _, ok := existing[line[0]]; ok {
			inStoreAlreadyCount = inStoreAlreadyCount + 1
			continue
		}
		// De-dupe import lines that appear more than once.
		if match, ok := seen[line[0]]; ok {
			// Fail if two duplicate imports have different metadata.
			if match[1] != line[1] {
				return fmt.Errorf("%w: %s duplicate import with differing metadata: %s vs %s", os.ErrInvalid, line[0], line[1], match[1])
			}
			dupeImportCount = dupeImportCount + 1
			continue
		}
		seen[line[0]] = line
		requests = append(requests, line[0])
		metadata = append(metadata, line[1])
	}
	logger.Stderr.Printf("queued: %d, duplicates removed: %d, existing removed: %d", len(requests), dupeImportCount, inStoreAlreadyCount)
	return fetch.Do(ctx, requests, concurrency, false, func(innerCtx context.Context, idx int, f *file.File) error {
		f.Meta.Merge(metadata[idx])
		logger.Stdout.Printf("%s", f.Meta)
		// Ignore errors about existing files, this may happen when imports are
		// run multiple times.
		if err := Put(innerCtx, store, f, set); !errors.Is(err, os.ErrExist) {
			return err
		}
		return nil
	})
}
