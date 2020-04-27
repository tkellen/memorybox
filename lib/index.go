package memorybox

import (
	"github.com/tkellen/memorybox/internal/archive"
	"io/ioutil"
	"sort"
	"strings"
)

// Index returns a map of byte arrays containing the content of every
// metadata file in the store, keyed by the name of the file they describe.
// This is gonna break down hard on large stores.
func Index(store Store) (map[string][]byte, error) {
	index := map[string][]byte{}
	metafiles, err := store.Search(archive.MetaFilePrefix)
	if err != nil {
		return nil, err
	}
	sort.Strings(metafiles)
	for _, metafile := range metafiles {
		reader, getErr := store.Get(metafile)
		if getErr != nil {
			return nil, getErr
		}
		bytes, readErr := ioutil.ReadAll(reader)
		if readErr != nil {
			return nil, readErr
		}
		reader.Close()
		index[strings.TrimPrefix(metafile, archive.MetaFilePrefix)] = bytes
	}
	return index, nil
}
