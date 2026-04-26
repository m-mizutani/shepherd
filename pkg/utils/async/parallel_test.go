package async_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/m-mizutani/gt"
	"github.com/m-mizutani/shepherd/pkg/utils/async"
)

func TestRunParallel_AllSuccess(t *testing.T) {
	var counter atomic.Int32
	err := async.RunParallel(context.Background(),
		func(ctx context.Context) error { counter.Add(1); return nil },
		func(ctx context.Context) error { counter.Add(1); return nil },
		func(ctx context.Context) error { counter.Add(1); return nil },
	)
	gt.NoError(t, err)
	gt.N(t, int(counter.Load())).Equal(3)
}

func TestRunParallel_PartialFailure(t *testing.T) {
	completed := atomic.Int32{}
	errSentinel := errors.New("first failed")
	err := async.RunParallel(context.Background(),
		func(ctx context.Context) error { return errSentinel },
		func(ctx context.Context) error { completed.Add(1); return nil },
		func(ctx context.Context) error { completed.Add(1); return nil },
	)
	gt.Error(t, err)
	gt.True(t, errors.Is(err, errSentinel))
	// other goroutines must complete despite one failing.
	gt.N(t, int(completed.Load())).Equal(2)
}

func TestRunParallel_PanicRecovered(t *testing.T) {
	completed := atomic.Int32{}
	err := async.RunParallel(context.Background(),
		func(ctx context.Context) error { panic("boom") },
		func(ctx context.Context) error { completed.Add(1); return nil },
	)
	gt.Error(t, err)
	gt.S(t, err.Error()).Contains("panic in async.RunParallel handler")
	gt.N(t, int(completed.Load())).Equal(1)
}

func TestRunParallel_NoFns(t *testing.T) {
	gt.NoError(t, async.RunParallel(context.Background()))
}
