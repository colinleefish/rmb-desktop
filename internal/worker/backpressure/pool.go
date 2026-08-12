package backpressure

import (
	"context"
	"sync"
)

// RunParallel runs jobs for each id with at most limit goroutines.
func RunParallel(ctx context.Context, ids []string, limit int, fn func(ctx context.Context, id string)) {
	if len(ids) == 0 {
		return
	}
	if limit < 1 {
		limit = 1
	}
	if limit > len(ids) {
		limit = len(ids)
	}

	sem := make(chan struct{}, limit)
	var wg sync.WaitGroup
	for _, id := range ids {
		if err := ctx.Err(); err != nil {
			break
		}
		select {
		case <-ctx.Done():
			wg.Wait()
			return
		case sem <- struct{}{}:
		}
		id := id
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			fn(ctx, id)
		}()
	}
	wg.Wait()
}
