package msg_test

import (
	"context"
	"testing"

	"github.com/m-mizutani/gt"
	"github.com/m-mizutani/shepherd/pkg/utils/msg"
)

func TestWithAndEmit(t *testing.T) {
	var notifies, traces, warns []string

	ctx := msg.With(context.Background(),
		func(ctx context.Context, m string) { notifies = append(notifies, m) },
		func(ctx context.Context, m string) { traces = append(traces, m) },
		func(ctx context.Context, m string) { warns = append(warns, m) },
	)
	msg.Notify(ctx, "hi %s", "there")
	msg.Trace(ctx, "trace %d", 1)
	msg.Warn(ctx, "warn %v", true)

	gt.Equal(t, notifies, []string{"hi there"})
	gt.Equal(t, traces, []string{"trace 1"})
	gt.Equal(t, warns, []string{"warn true"})
}

func TestEmit_NoCallback_NoPanic(t *testing.T) {
	// no With(): Notify/Trace/Warn should fall back to debug log silently.
	msg.Notify(context.Background(), "no-op")
	msg.Trace(context.Background(), "no-op")
	msg.Warn(context.Background(), "no-op")
}

func TestFuncs_Returns(t *testing.T) {
	calls := 0
	ctx := msg.With(context.Background(),
		nil,
		func(ctx context.Context, m string) { calls++ },
		nil,
	)
	notify, trace, warn := msg.Funcs(ctx)
	gt.True(t, notify == nil)
	gt.True(t, trace != nil)
	gt.True(t, warn == nil)
	trace(ctx, "x")
	gt.N(t, calls).Equal(1)
}

func TestCopyTo(t *testing.T) {
	calls := 0
	src := msg.With(context.Background(),
		func(ctx context.Context, m string) { calls++ },
		nil, nil,
	)
	dst := msg.CopyTo(context.Background(), src)
	msg.Notify(dst, "x")
	gt.N(t, calls).Equal(1)
}
