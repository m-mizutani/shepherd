package async

import (
	"context"
	"runtime/debug"
	"sync"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/shepherd/pkg/utils/errutil"
	"github.com/m-mizutani/shepherd/pkg/utils/logging"
)

var wg sync.WaitGroup

func Dispatch(ctx context.Context, handler func(ctx context.Context) error) {
	newCtx := newBackgroundContext(ctx)

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() {
			if r := recover(); r != nil {
				stack := debug.Stack()
				errutil.Handle(newCtx, goerr.New("panic in async handler",
					goerr.V("recover", r),
					goerr.V("stack", string(stack))))
			}
		}()

		if err := handler(newCtx); err != nil {
			errutil.Handle(newCtx, err)
		}
	}()
}

func Wait() {
	wg.Wait()
}

func newBackgroundContext(ctx context.Context) context.Context {
	newCtx := context.Background()
	newCtx = logging.With(newCtx, logging.From(ctx))
	return newCtx
}
