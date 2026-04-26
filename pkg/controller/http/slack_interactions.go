package http

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
	slackService "github.com/m-mizutani/shepherd/pkg/service/slack"
	"github.com/m-mizutani/shepherd/pkg/usecase/triage"
	"github.com/m-mizutani/shepherd/pkg/utils/errutil"
	"github.com/m-mizutani/shepherd/pkg/utils/logging"
	slackgo "github.com/slack-go/slack"
)

// TriageInteractionsUC is the slim surface the interactions endpoint needs
// from the triage usecase. Defined as an interface so tests can substitute
// a fake without instantiating the full executor.
type TriageInteractionsUC interface {
	HandleSubmit(ctx context.Context, sub triage.Submission) error
	SubmitInvalid(ctx context.Context, channelID, messageTS string)
	WorkspaceForChannel(ctx context.Context, channelID string) (types.WorkspaceID, bool)
	TicketByID(ctx context.Context, workspaceID types.WorkspaceID, ticketID types.TicketID) (*model.Ticket, error)
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
			if cb.ActionCallback.BlockActions[i].ActionID == slackService.TriageSubmitActionID {
				triageAction = cb.ActionCallback.BlockActions[i]
				break
			}
		}
		if triageAction == nil {
			logger.Debug("slack interaction ignored: no triage submit action")
			w.WriteHeader(http.StatusOK)
			return
		}

		ticketID := types.TicketID(triageAction.Value)
		channelID := cb.Channel.ID
		messageTS := cb.Message.Timestamp

		// Acknowledge Slack as early as possible. The actual work runs
		// synchronously below since it's bounded (LLM is not invoked here)
		// and we want the response to fully reflect the new state.
		w.WriteHeader(http.StatusOK)

		wsID, ok := uc.WorkspaceForChannel(ctx, channelID)
		if !ok {
			logger.Debug("triage submit ignored: channel not mapped",
				slog.String("channel", channelID),
			)
			uc.SubmitInvalid(ctx, channelID, messageTS)
			return
		}

		ticket, err := uc.TicketByID(ctx, wsID, ticketID)
		if err != nil || ticket == nil {
			logger.Debug("triage submit invalidated: ticket not found",
				slog.String("ticket_id", string(ticketID)),
			)
			uc.SubmitInvalid(ctx, channelID, messageTS)
			return
		}

		sub := triage.Submission{
			WorkspaceID: wsID,
			TicketID:    ticketID,
			ChannelID:   channelID,
			MessageTS:   messageTS,
			State:       cb.BlockActionState,
		}
		if err := uc.HandleSubmit(ctx, sub); err != nil {
			errutil.Handle(ctx, goerr.Wrap(err, "handle triage submit"))
		}
	}
}
