package http

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
	slackService "github.com/m-mizutani/shepherd/pkg/service/slack"
	"github.com/m-mizutani/shepherd/pkg/usecase/triage"
	"github.com/m-mizutani/shepherd/pkg/utils/async"
	"github.com/m-mizutani/shepherd/pkg/utils/errutil"
	"github.com/m-mizutani/shepherd/pkg/utils/logging"
	slackgo "github.com/slack-go/slack"
)

// TriageInteractionsUC is the slim surface the interactions endpoint needs
// from the triage usecase. Defined as an interface so tests can substitute
// a fake without instantiating the full executor. The handler stays free of
// workspace / ticket resolution — those live behind HandleSubmit /
// HandleRetry, which take only the raw Slack payload fields.
type TriageInteractionsUC interface {
	HandleSubmit(ctx context.Context, sub triage.Submission) error
	HandleRetry(ctx context.Context, ticketID types.TicketID, channelID, messageTS string) error
}

// slackInteractionsHandler handles Slack Block Kit interactivity callbacks
// (block_actions / view_submission). Slack delivers these as
// application/x-www-form-urlencoded with a single "payload" field carrying
// the JSON. The signature middleware (shared with /event) has already
// validated the request body bytes by the time we run.
func slackInteractionsHandler(uc TriageInteractionsUC) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := logging.From(ctx)

		// Slack interaction payloads are well under 1 MiB; cap the body to
		// avoid unbounded ParseForm allocations (gosec G120).
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

		if err := r.ParseForm(); err != nil {
			errutil.HandleHTTP(ctx, w, goerr.Wrap(err, "parse interaction form"), http.StatusBadRequest)
			return
		}
		raw := r.PostForm.Get("payload")
		if raw == "" {
			errutil.HandleHTTP(ctx, w, goerr.New("missing payload"), http.StatusBadRequest)
			return
		}

		var cb slackgo.InteractionCallback
		if err := json.Unmarshal([]byte(raw), &cb); err != nil {
			errutil.HandleHTTP(ctx, w, goerr.Wrap(err, "decode interaction payload"), http.StatusBadRequest)
			return
		}

		// MVP: only block_actions are handled, and only the triage submit
		// button. Anything else is acknowledged with 200 so Slack stops
		// retrying without surfacing as an error.
		if cb.Type != slackgo.InteractionTypeBlockActions {
			logger.Debug("slack interaction ignored: not block_actions",
				slog.String("type", string(cb.Type)),
			)
			w.WriteHeader(http.StatusOK)
			return
		}

		var triageAction *slackgo.BlockAction
		for i := range cb.ActionCallback.BlockActions {
			id := cb.ActionCallback.BlockActions[i].ActionID
			if id == slackService.TriageSubmitActionID || id == slackService.TriageRetryActionID {
				triageAction = cb.ActionCallback.BlockActions[i]
				break
			}
		}
		if triageAction == nil {
			logger.Debug("slack interaction ignored: no triage action")
			w.WriteHeader(http.StatusOK)
			return
		}

		ticketID := types.TicketID(triageAction.Value)
		channelID := cb.Channel.ID
		messageTS := cb.Message.Timestamp
		actionID := triageAction.ActionID
		state := cb.BlockActionState

		// Acknowledge Slack immediately. Slack enforces a 3-second deadline
		// on the interaction response, so the only work we do synchronously
		// is parsing the payload (which has to happen before we can return —
		// the body is buffered in the request). Workspace resolution, ticket
		// loading, history checks, Slack API mutations, and the planner
		// re-dispatch all live behind HandleSubmit / HandleRetry; we hand
		// them off to async.Dispatch which provides a detached context,
		// panic recovery, and Sentry routing.
		w.WriteHeader(http.StatusOK)

		async.Dispatch(ctx, func(ctx context.Context) error {
			switch actionID {
			case slackService.TriageSubmitActionID:
				return uc.HandleSubmit(ctx, triage.Submission{
					TicketID:  ticketID,
					ChannelID: channelID,
					MessageTS: messageTS,
					State:     state,
				})
			case slackService.TriageRetryActionID:
				return uc.HandleRetry(ctx, ticketID, channelID, messageTS)
			}
			return nil
		})
	}
}
