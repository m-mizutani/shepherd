package async

import (
	"context"
	"runtime/debug"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/shepherd/pkg/utils/errutil"
	"github.com/m-mizutani/shepherd/pkg/utils/logging"
)

func Dispatch(ctx context.Context, handler func(ctx context.Context) error) {
	newCtx := newBackgroundContext(ctx)

	go func() {
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

func newBackgroundContext(ctx context.Context) context.Context {
	newCtx := context.Background()
	newCtx = logging.With(newCtx, logging.From(ctx))
	return newCtx
}
