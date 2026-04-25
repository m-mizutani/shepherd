package agentstore

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/gollem/trace"
	"github.com/m-mizutani/shepherd/pkg/utils/errutil"
)

// TraceRepository implements gollem/trace.Repository on top of a Backend.
// One trace is persisted per Execute call as "trace/v1/{traceID}.json".
//
// Save errors are logged via errutil but always returned as nil so that trace
// persistence failures cannot interrupt agent execution.
type TraceRepository struct {
	backend Backend
}

var _ trace.Repository = (*TraceRepository)(nil)

// NewTraceRepository wraps the given Backend as a trace.Repository.
func NewTraceRepository(backend Backend) *TraceRepository {
	return &TraceRepository{backend: backend}
}

func traceKey(traceID string) string {
	return fmt.Sprintf("%s%s.json", tracePrefix, traceID)
}

// Save implements trace.Repository.
func (r *TraceRepository) Save(ctx context.Context, t *trace.Trace) error {
	if t == nil {
		return nil
	}
	if t.TraceID == "" {
		errutil.Handle(ctx, goerr.New("trace.TraceID must not be empty"))
		return nil
	}

	w, err := r.backend.Put(ctx, traceKey(t.TraceID))
	if err != nil {
		errutil.Handle(ctx, goerr.Wrap(err, "failed to open trace writer",
			goerr.V("trace_id", t.TraceID)))
		return nil
	}

	if err := json.NewEncoder(w).Encode(t); err != nil {
		// Discard the partial payload via Abort (see Writer doc in
		// backend.go) so neither GCS nor the local FS keeps a truncated blob.
		w.Abort(err)
		errutil.Handle(ctx, goerr.Wrap(err, "failed to encode trace",
			goerr.V("trace_id", t.TraceID)))
		return nil
	}
	if err := w.Close(); err != nil {
		errutil.Handle(ctx, goerr.Wrap(err, "failed to flush trace",
			goerr.V("trace_id", t.TraceID)))
	}
	return nil
}
