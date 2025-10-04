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
		var logBuf bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{
			Level: slog.LevelError,
		}))

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
			// Wait a bit for log to be written
			time.Sleep(100 * time.Millisecond)

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
