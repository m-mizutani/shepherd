package async

import (
	"context"
	"errors"
	"runtime/debug"
	"sync"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/shepherd/pkg/utils/errutil"
)

// RunParallel runs every fn concurrently in its own goroutine, recovers from
// panics, and waits for all goroutines to complete before returning. Errors
// (including panics turned into errors) from individual goroutines are
// joined into the returned error. The function returns nil only when every
// fn returned nil.
//
// Unlike Dispatch, this helper is synchronous: callers wait for completion
// because they typically need the side effects (e.g. each fn writes a
// per-subtask result through a repository). Panic recovery still goes
// through errutil.Handle so unexpected failures are visible in Sentry.
func RunParallel(ctx context.Context, fns ...func(ctx context.Context) error) error {
	if len(fns) == 0 {
		return nil
	}

	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		errs []error
	)
	wg.Add(len(fns))

	for _, fn := range fns {
		fn := fn
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					stack := debug.Stack()
					panicErr := goerr.New("panic in async.RunParallel handler",
						goerr.V("recover", r),
						goerr.V("stack", string(stack)))
					errutil.Handle(ctx, panicErr)
					mu.Lock()
					errs = append(errs, panicErr)
					mu.Unlock()
				}
			}()

			if err := fn(ctx); err != nil {
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if len(errs) == 0 {
		return nil
	}
	return errors.Join(errs...)
}
