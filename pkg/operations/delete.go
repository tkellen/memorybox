package operations

import (
	"context"
	"github.com/tkellen/memorybox/pkg/archive"
	"github.com/tkellen/memorybox/pkg/store"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
)

// Delete removes any number of requested data files and associated meta files
// from the store.
func Delete(
	ctx context.Context,
	s store.Store,
	concurrency int,
	requests []string,
) error {
	eg, egCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		sem := semaphore.NewWeighted(int64(concurrency))
		for _, request := range requests {
			request := request // https://golang.org/doc/faq#closures_and_goroutines
			if err := sem.Acquire(egCtx, 1); err != nil {
				return err
			}
			eg.Go(func() error {
				defer sem.Release(1)
				pair := []string{request, archive.MetaFileNameFrom(request)}
				for _, file := range pair {
					if err := s.Delete(ctx, file); err != nil {
						return err
					}
				}
				return nil
			})
		}
		return nil
	})
	return eg.Wait()
}
