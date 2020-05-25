package archive

import (
	"context"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
)

// Sync converges the content of two provided stores so they are identical.
func Sync(ctx context.Context, logger *Logger, source Store, dest Store, mode string, concurrency int) error {
	sourceFiles, sourceErr := source.Search(ctx, "")
	if sourceErr != nil {
		return sourceErr
	}
	destFiles, destErr := dest.Search(ctx, "")
	if destErr != nil {
		return destErr
	}
	destIndex := destFiles.ByName()
	eg, egCtx := errgroup.WithContext(ctx)
	sem := semaphore.NewWeighted(int64(concurrency))
	if mode == "metafiles" {
		sourceFiles = sourceFiles.Meta()
	}
	if mode == "datafiles" {
		sourceFiles = sourceFiles.Data()
	}
	eg.Go(func() error {
		for _, src := range sourceFiles {
			// Skip incoming files that are up-to-date in the destination store.
			if dest, ok := destIndex[src.Name]; ok {
				if dest.CurrentWith(src) {
					logger.Verbose.Printf("%s (skipped)\n", src.Name)
					continue
				}
			}
			if err := sem.Acquire(egCtx, 1); err != nil {
				return err
			}
			src := src
			eg.Go(func() error {
				file, err := source.Get(egCtx, src.Name)
				if err != nil {
					return err
				}
				defer func() {
					logger.Verbose.Printf("%s (synced)\n", src.Name)
					file.Close()
					sem.Release(1)
				}()
				return dest.Put(egCtx, file, file.Name, file.LastModified)
			})
		}
		return nil
	})
	return eg.Wait()
}
