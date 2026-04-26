// Package msg provides context-scoped progress messaging callbacks. Tools and
// child agents emit user-visible status updates (Notify), detailed traces
// (Trace), or warnings (Warn) without knowing how those messages reach Slack.
// The triage usecase wires the actual delivery — typically a chat.update on a
// Slack progress message — by attaching the appropriate callbacks to the
// context before invoking child code. The pattern mirrors the warren `msg`
// package.
package msg

import (
	"context"
	"fmt"

	"github.com/m-mizutani/shepherd/pkg/utils/logging"
)

// NotifyFunc is invoked for user-visible status milestones (e.g. "starting
// investigation", "ready to summarise"). It typically posts or updates a
// Slack message in production.
type NotifyFunc func(ctx context.Context, msg string)

// TraceFunc is invoked for fine-grained progress traces (e.g. "querying
// channel #foo", "received 12 results"). triage uses this to update a per-
// subtask Slack context block.
type TraceFunc func(ctx context.Context, msg string)

// WarnFunc is invoked for non-fatal warnings (e.g. degraded results, partial
// failures).
type WarnFunc func(ctx context.Context, msg string)

type ctxNotifyKey struct{}
type ctxTraceKey struct{}
type ctxWarnKey struct{}

// With returns a context that carries the supplied callbacks. Any of the
// callbacks may be nil, in which case the corresponding emit (Notify / Trace
// / Warn) becomes a debug-log fallback.
func With(ctx context.Context, notify NotifyFunc, trace TraceFunc, warn WarnFunc) context.Context {
	ctx = context.WithValue(ctx, ctxNotifyKey{}, notify)
	ctx = context.WithValue(ctx, ctxTraceKey{}, trace)
	ctx = context.WithValue(ctx, ctxWarnKey{}, warn)
	return ctx
}

// CopyTo copies the registered callbacks from src onto dst. Useful when a
// goroutine builds a fresh context (e.g. via context.Background after a
// shutdown signal) but still wants to deliver progress to the original
// caller's display.
func CopyTo(dst context.Context, src context.Context) context.Context {
	dst = context.WithValue(dst, ctxNotifyKey{}, src.Value(ctxNotifyKey{}))
	dst = context.WithValue(dst, ctxTraceKey{}, src.Value(ctxTraceKey{}))
	dst = context.WithValue(dst, ctxWarnKey{}, src.Value(ctxWarnKey{}))
	return dst
}

// Funcs returns the currently registered callbacks, or nil for any callback
// that is not set. Used when a layer wants to wrap (rather than replace) the
// outer display, e.g. to persist a copy of every notify alongside the live
// Slack update.
func Funcs(ctx context.Context) (NotifyFunc, TraceFunc, WarnFunc) {
	var (
		n NotifyFunc
		t TraceFunc
		w WarnFunc
	)
	if v := ctx.Value(ctxNotifyKey{}); v != nil {
		if fn, ok := v.(NotifyFunc); ok {
			n = fn
		}
	}
	if v := ctx.Value(ctxTraceKey{}); v != nil {
		if fn, ok := v.(TraceFunc); ok {
			t = fn
		}
	}
	if v := ctx.Value(ctxWarnKey{}); v != nil {
		if fn, ok := v.(WarnFunc); ok {
			w = fn
		}
	}
	return n, t, w
}

// Notify delivers a user-visible status message via the registered callback,
// or falls back to a debug log if nothing is registered.
func Notify(ctx context.Context, format string, args ...any) {
	rendered := fmt.Sprintf(format, args...)
	if v := ctx.Value(ctxNotifyKey{}); v != nil {
		if fn, ok := v.(NotifyFunc); ok && fn != nil {
			fn(ctx, rendered)
			return
		}
	}
	logging.From(ctx).Debug("msg.Notify dropped (no callback)", "message", rendered)
}

// Trace delivers a detailed progress trace via the registered callback, or
// falls back to a debug log.
func Trace(ctx context.Context, format string, args ...any) {
	rendered := fmt.Sprintf(format, args...)
	if v := ctx.Value(ctxTraceKey{}); v != nil {
		if fn, ok := v.(TraceFunc); ok && fn != nil {
			fn(ctx, rendered)
			return
		}
	}
	logging.From(ctx).Debug("msg.Trace dropped (no callback)", "message", rendered)
}

// Warn delivers a non-fatal warning via the registered callback, or falls
// back to a debug log.
func Warn(ctx context.Context, format string, args ...any) {
	rendered := fmt.Sprintf(format, args...)
	if v := ctx.Value(ctxWarnKey{}); v != nil {
		if fn, ok := v.(WarnFunc); ok && fn != nil {
			fn(ctx, rendered)
			return
		}
	}
	logging.From(ctx).Debug("msg.Warn dropped (no callback)", "message", rendered)
}
