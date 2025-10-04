package async

import (
	"context"
	"runtime/debug"

	"github.com/m-mizutani/ctxlog"
)

// Dispatch executes a handler function asynchronously with proper context and panic recovery
//
// Parameters:
//   - ctx: Original context (values will be preserved, but cancellation won't affect the async handler)
//   - handler: Function to execute asynchronously
//
// Behavior:
//   - Creates a new background context with preserved logger
//   - Executes handler in a new goroutine
//   - Recovers from panics and logs them
//   - Logs errors returned by handler
func Dispatch(ctx context.Context, handler func(ctx context.Context) error) {
	newCtx := newBackgroundContext(ctx)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				stack := debug.Stack()
				logger := ctxlog.From(newCtx)
				logger.Error("panic in async handler",
					"recover", r,
					"stack", string(stack))
			}
		}()

		if err := handler(newCtx); err != nil {
			logger := ctxlog.From(newCtx)
			logger.Error("error in async handler", "error", err)
		}
	}()
}

// newBackgroundContext creates a new background context preserving important values
//
// Preserved values:
//   - ctxlog logger
//
// Returns: New context.Background() with preserved values
func newBackgroundContext(ctx context.Context) context.Context {
	newCtx := context.Background()
	newCtx = ctxlog.With(newCtx, ctxlog.From(ctx))
	return newCtx
}
