package operations

import (
	"bufio"
	"context"
	"fmt"
	"github.com/tidwall/gjson"
	"github.com/tkellen/memorybox/pkg/archive"
	"github.com/tkellen/memorybox/pkg/store"
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
// https://audio.com/audio.mp3	{"title":"some mp3 on the internet"}
// ```
// Import will intelligently de-dupe manifests and remove entries that already
// appear in the store as being sourced from the filepath or URL in the manifest
// file.
func Import(
	ctx context.Context,
	logger *Logger,
	s store.Store,
	concurrency int,
	imports io.Reader,
) error {
	var entries []importEntry
	scanner := bufio.NewScanner(imports)
	for scanner.Scan() {
		fields := strings.SplitN(scanner.Text(), "\t", 2)
		// Allow import lines with no metadata.
		if len(fields) < 2 {
			fields = append(fields, "")
		}
		entries = append(entries, importEntry{
			Request:  fields[0],
			Metadata: fields[1],
		})
	}
	// Get all metadata entries from the store.
	index, indexErr := index(ctx, logger, s, concurrency, false)
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
	for _, entry := range entries {
		// Skip items that appear in the store already.
		if _, ok := sourceIndex[entry.Request]; ok {
			inStoreAlreadyCount = inStoreAlreadyCount + 1
			continue
		}
		// De-dupe import lines that appear more than once.
		if match, ok := importIndex[entry.Request]; ok {
			// Fail if two imports have different metadata.
			if match.Metadata != entry.Metadata {
				return fmt.Errorf("%w: %s duplicate found with differing metadata: %s vs %s", os.ErrInvalid, entry.Request, entry.Metadata, match.Metadata)
			}
			dupeImportCount = dupeImportCount + 1
			continue
		}
		importIndex[entry.Request] = entry
		putRequests = append(putRequests, entry.Request)
		putMetadatas = append(putMetadatas, entry.Metadata)
	}
	logger.Stderr.Printf("queued: %d, duplicates removed: %d, existing removed: %d", len(putRequests), dupeImportCount, inStoreAlreadyCount)
	return Put(ctx, logger, s, concurrency, putRequests, putMetadatas)
}
