package async_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/m-mizutani/ctxlog"
	"github.com/m-mizutani/gt"
	"github.com/m-mizutani/shepherd/pkg/utils/async"
)

// safeBuffer is a thread-safe buffer for concurrent logging
type safeBuffer struct {
	b bytes.Buffer
	m sync.Mutex
}

func (sb *safeBuffer) Write(p []byte) (int, error) {
	sb.m.Lock()
	defer sb.m.Unlock()
	return sb.b.Write(p)
}

func (sb *safeBuffer) String() string {
	sb.m.Lock()
	defer sb.m.Unlock()
	return sb.b.String()
}

// syncHandler is a slog.Handler that signals when a log is written
type syncHandler struct {
	handler slog.Handler
	done    chan struct{}
}

func newSyncHandler(buf *safeBuffer) *syncHandler {
	return &syncHandler{
		handler: slog.NewTextHandler(buf, &slog.HandlerOptions{
			Level: slog.LevelError,
		}),
		done: make(chan struct{}, 1),
	}
}

func (h *syncHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.handler.Enabled(ctx, level)
}

func (h *syncHandler) Handle(ctx context.Context, r slog.Record) error {
	err := h.handler.Handle(ctx, r)
	select {
	case h.done <- struct{}{}:
	default:
	}
	return err
}

func (h *syncHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &syncHandler{
		handler: h.handler.WithAttrs(attrs),
		done:    h.done,
	}
}

func (h *syncHandler) WithGroup(name string) slog.Handler {
	return &syncHandler{
		handler: h.handler.WithGroup(name),
		done:    h.done,
	}
}

func TestDispatch(t *testing.T) {
	t.Run("executes handler asynchronously", func(t *testing.T) {
		ctx := context.Background()
		var wg sync.WaitGroup
		executed := false

		wg.Add(1)
		async.Dispatch(ctx, func(ctx context.Context) error {
			defer wg.Done()
			executed = true
			return nil
		})

		wg.Wait()
		gt.True(t, executed)
	})

	t.Run("handles errors without crashing", func(t *testing.T) {
		ctx := context.Background()
		var wg sync.WaitGroup

		wg.Add(1)
		async.Dispatch(ctx, func(ctx context.Context) error {
			defer wg.Done()
			return errors.New("test error")
		})

		wg.Wait()
		// Test passes if no panic occurs
	})

	t.Run("recovers from panic", func(t *testing.T) {
		ctx := context.Background()
		done := make(chan bool, 1)

		async.Dispatch(ctx, func(ctx context.Context) error {
			defer func() {
				done <- true
			}()
			panic("test panic")
		})

		select {
		case <-done:
			// Test passes if panic was recovered
		case <-time.After(1 * time.Second):
			t.Fatal("handler did not complete within timeout")
		}
	})

	t.Run("recovers from panic with stack trace", func(t *testing.T) {
		logBuf := &safeBuffer{}
		handler := newSyncHandler(logBuf)
		logger := slog.New(handler)

		ctx := context.Background()
		ctx = ctxlog.With(ctx, logger)

		done := make(chan bool, 1)

		async.Dispatch(ctx, func(ctx context.Context) error {
			defer func() {
				done <- true
			}()
			panic("test panic with stack")
		})

		select {
		case <-done:
			// Wait for log to be written
			select {
			case <-handler.done:
				// Log has been written
			case <-time.After(1 * time.Second):
				t.Fatal("log was not written within timeout")
			}

			logOutput := logBuf.String()

			// Check that panic message is logged
			gt.True(t, strings.Contains(logOutput, "panic in async handler"))
			gt.True(t, strings.Contains(logOutput, "test panic with stack"))

			// Check that stack trace is logged
			gt.True(t, strings.Contains(logOutput, "goroutine"))
			gt.True(t, strings.Contains(logOutput, "dispatch_test.go"))
		case <-time.After(1 * time.Second):
			t.Fatal("handler did not complete within timeout")
		}
	})

	t.Run("preserves context values", func(t *testing.T) {
		ctx := context.Background()

		// Set context values
		logger := slog.Default()
		ctx = ctxlog.With(ctx, logger)

		var wg sync.WaitGroup
		wg.Add(1)

		async.Dispatch(ctx, func(newCtx context.Context) error {
			defer wg.Done()

			// Check preserved values
			gt.NotNil(t, ctxlog.From(newCtx))

			return nil
		})

		wg.Wait()
	})

	t.Run("creates new background context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		var wg sync.WaitGroup
		wg.Add(1)

		async.Dispatch(ctx, func(newCtx context.Context) error {
			defer wg.Done()

			// Cancel original context
			cancel()

			// New context should not be affected
			select {
			case <-newCtx.Done():
				t.Error("new context was cancelled")
			default:
				// Expected: context is not cancelled
			}

			return nil
		})

		wg.Wait()
	})
}
