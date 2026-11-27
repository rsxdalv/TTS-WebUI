package uv

import (
	"context"
	"sync"

	"github.com/rsxdalv/TTS-WebUI/core/internal/platform"
)

// runParallel runs the tasks at the same time. The first error cancels the
// others (their context) and is the one returned, after every task has ended.
func runParallel(ctx context.Context, tasks ...func(context.Context) error) error {
	if len(tasks) == 1 {
		return tasks[0](ctx)
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var (
		wg    sync.WaitGroup
		mu    sync.Mutex
		first error
	)
	for _, task := range tasks {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := task(ctx); err != nil {
				mu.Lock()
				if first == nil {
					first = err
					cancel()
				}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	return first
}

// lockedEmit makes emit safe to call from several goroutines: the callers' emit
// functions (they keep a log slice, they write to a response) are not.
func lockedEmit(emit platform.Emit) platform.Emit {
	var mu sync.Mutex
	return func(line string) {
		mu.Lock()
		defer mu.Unlock()
		emit(line)
	}
}
