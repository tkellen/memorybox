package archive

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/tkellen/memorybox/pkg/file"
	"golang.org/x/sync/errgroup"
	"io/ioutil"
	"os"
	"time"
)

// GetDataByPrefix retrieves a datafile from any backing store as long as there
// is only one match.
func GetDataByPrefix(ctx context.Context, store Store, prefix string) (*file.File, error) {
	return findAndGet(ctx, store, prefix, false)
}

// GetMetaByPrefix retrieves a metafile from any backing store as long as there
// is only one match.
func GetMetaByPrefix(ctx context.Context, store Store, prefix string) (*file.File, error) {
	return findAndGet(ctx, store, prefix, true)
}

// Put persists a datafile/metafile pair for any backing store and returns the
// meta information about the file.
func Put(ctx context.Context, store Store, f *file.File, set string) (*file.File, error) {
	if set == "" {
		if set, _ = os.Hostname(); set == "" {
			set = "unknown"
		}
	}
	eg, egCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		exist, err := store.Stat(egCtx, f.Name)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return store.Put(egCtx, f.Body, f.Name, f.LastModified)
			}
			return err
		}
		if !exist.CurrentWith(f) {
			return store.Put(egCtx, f.Body, f.Name, f.LastModified)
		}
		return nil
	})
	eg.Go(func() error {
		name := file.MetaNameFrom(f.Name)
		meta, err := GetMetaByPrefix(egCtx, store, name)
		// Persist metafile if one doesn't exist.
		if errors.Is(err, os.ErrNotExist) {
			f.Meta.Set(file.MetaKeyImportSet, set)
			return store.Put(egCtx, bytes.NewReader(*f.Meta), name, time.Now())
		}
		// If there was no error, the meta file existed already. If a consumer
		// tries to store the same file twice, there is no error. This clause
		// ensures the metadata that is output to the screen reflects what is
		// already in the store.
		if err == nil {
			f = meta
		}
		// Return an error if there was one.
		return err
	})
	if err := eg.Wait(); err != nil {
		return nil, err
	}
	return f, nil
}

// Delete removes a datafile/metafile pair for any backing store.
func Delete(ctx context.Context, store Store, name string) error {
	f, findErr := find(ctx, store, name, false)
	if findErr != nil {
		return findErr
	}
	eg, egCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		return store.Delete(egCtx, f.Name)
	})
	eg.Go(func() error {
		return store.Delete(egCtx, file.MetaNameFrom(f.Name))
	})
	return eg.Wait()
}

func find(ctx context.Context, store Store, name string, meta bool) (*file.File, error) {
	if meta {
		name = file.MetaNameFrom(name)
	}
	matches, searchErr := store.Search(ctx, name)
	if searchErr != nil {
		return nil, fmt.Errorf("get: %w", searchErr)
	}
	if len(matches) != 1 {
		return nil, fmt.Errorf("%w: %d objects matched", os.ErrNotExist, len(matches))
	}
	return matches[0], nil
}

func findAndGet(ctx context.Context, store Store, name string, meta bool) (*file.File, error) {
	match, findErr := find(ctx, store, name, meta)
	if findErr != nil {
		return nil, findErr
	}
	f, err := store.Get(ctx, match.Name)
	if err != nil {
		return nil, err
	}
	if meta {
		data, readErr := ioutil.ReadAll(f.Body)
		if readErr != nil {
			return nil, readErr
		}
		meta := file.Meta(data)
		f.Body = nil
		f.Meta = &meta
	}
	return f, nil
}
