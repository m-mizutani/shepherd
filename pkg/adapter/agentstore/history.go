package agentstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/shepherd/pkg/utils/errutil"
	"github.com/m-mizutani/shepherd/pkg/utils/logging"
)

// HistoryRepository implements gollem.HistoryRepository on top of a Backend.
//
// SessionID is treated as an opaque, slash-separated path component
// (e.g. "{workspaceID}/{ticketID}"). It is appended verbatim under
// "history/v1/" with a ".json" suffix.
//
// Save failures are logged via errutil but never returned, matching warren's
// behaviour: history persistence must not abort agent execution.
// Load failures (including history version mismatches) are also swallowed and
// produce a fresh session.
type HistoryRepository struct {
	backend Backend
}

var _ gollem.HistoryRepository = (*HistoryRepository)(nil)

// NewHistoryRepository wraps the given Backend as a gollem.HistoryRepository.
func NewHistoryRepository(backend Backend) *HistoryRepository {
	return &HistoryRepository{backend: backend}
}

func historyKey(sessionID string) string {
	return fmt.Sprintf("%s%s.json", historyPrefix, sessionID)
}

// Load implements gollem.HistoryRepository. Missing sessions and version
// mismatches are returned as (nil, nil) so the agent starts fresh.
func (r *HistoryRepository) Load(ctx context.Context, sessionID string) (*gollem.History, error) {
	if sessionID == "" {
		return nil, goerr.New("history sessionID must not be empty")
	}
	logger := logging.From(ctx)

	rc, err := r.backend.Get(ctx, historyKey(sessionID))
	if err != nil {
		errutil.Handle(ctx, goerr.Wrap(err, "failed to load agent history",
			goerr.V("session_id", sessionID)))
		return nil, nil
	}
	if rc == nil {
		return nil, nil
	}
	defer func() {
		if cerr := rc.Close(); cerr != nil {
			errutil.Handle(ctx, goerr.Wrap(cerr, "failed to close history reader",
				goerr.V("session_id", sessionID)))
		}
	}()

	var h gollem.History
	if err := json.NewDecoder(rc).Decode(&h); err != nil {
		if errors.Is(err, gollem.ErrHistoryVersionMismatch) {
			logger.Warn("agent history version mismatch; starting fresh session",
				slog.String("session_id", sessionID),
				logging.ErrAttr(err))
			return nil, nil
		}
		errutil.Handle(ctx, goerr.Wrap(err, "failed to decode agent history",
			goerr.V("session_id", sessionID)))
		return nil, nil
	}
	return &h, nil
}

// Save implements gollem.HistoryRepository. Errors are logged via errutil
// and never returned, so trace storage failures do not abort agent execution.
func (r *HistoryRepository) Save(ctx context.Context, sessionID string, history *gollem.History) error {
	if sessionID == "" {
		errutil.Handle(ctx, goerr.New("history sessionID must not be empty"))
		return nil
	}
	if history == nil {
		return nil
	}

	w, err := r.backend.Put(ctx, historyKey(sessionID))
	if err != nil {
		errutil.Handle(ctx, goerr.Wrap(err, "failed to open history writer",
			goerr.V("session_id", sessionID)))
		return nil
	}

	if err := json.NewEncoder(w).Encode(history); err != nil {
		// Tell the backend to discard the partial payload — for GCS this
		// cancels the in-flight upload, for the filesystem it deletes the
		// half-written file and closes the FD.
		w.Abort(err)
		errutil.Handle(ctx, goerr.Wrap(err, "failed to encode agent history",
			goerr.V("session_id", sessionID)))
		return nil
	}
	if err := w.Close(); err != nil {
		errutil.Handle(ctx, goerr.Wrap(err, "failed to flush agent history",
			goerr.V("session_id", sessionID)))
	}
	return nil
}
