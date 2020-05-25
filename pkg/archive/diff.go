package archive

import (
	"context"
	"errors"
	"fmt"
	"github.com/tkellen/memorybox/pkg/file"
	"sort"
	"strings"
)

// Diff shows the differences between two stores.
func Diff(ctx context.Context, source Store, dest Store) error {
	var diffs []string
	index := map[Store]map[string]*file.File{}
	for _, store := range []Store{source, dest} {
		if files, err := store.Search(ctx, ""); err != nil {
			return err
		} else {
			index[store] = files.ByName()
		}
	}
	compares := []error{
		compare(index, source, dest),
		compare(index, dest, source),
	}
	for _, err := range compares {
		if err != nil {
			diffs = append(diffs, err.Error())
		}
	}
	if len(diffs) > 0 {
		return errors.New(strings.Join(diffs, "\n"))
	}
	return nil
}

func compare(index map[Store]map[string]*file.File, source Store, dest Store) error {
	var diffs []string
	for file := range index[source] {
		if _, ok := index[dest][file]; !ok {
			diffs = append(diffs, fmt.Sprintf("[%s]: %s [missing in %s]", source, file, dest))
		}
	}
	if len(diffs) > 0 {
		// make results deterministic
		sort.Strings(diffs)
		return errors.New(strings.Join(diffs, "\n"))
	}
	return nil
}
