package http

import (
	"context"
	"encoding/json"
	"errors"
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
// workspace / ticket resolution — every method below takes only raw Slack
// payload fields and the usecase resolves ticket + workspace internally.
type TriageInteractionsUC interface {
	HandleSubmit(ctx context.Context, sub triage.Submission) error
	HandleRetry(ctx context.Context, ticketID types.TicketID, channelID, messageTS string) error

	// Review flow (new). Open methods are invoked SYNCHRONOUSLY because
	// Slack's trigger_id has a ~3-second TTL and views.open must be called
	// before that window closes.
	HandleReviewEditOpen(ctx context.Context, ticketID types.TicketID, channelID, messageTS, triggerID string) error
	HandleReviewReinvestigateOpen(ctx context.Context, ticketID types.TicketID, channelID, messageTS, triggerID string) error
	HandleReviewSubmit(ctx context.Context, ticketID types.TicketID, channelID, messageTS, actorID string) error
	HandleReviewEditSubmit(ctx context.Context, ticketID types.TicketID, channelID, messageTS, actorID string, state *slackgo.ViewState) (triage.ReviewFieldErrors, error)
	HandleReviewReinvestigate(ctx context.Context, ticketID types.TicketID, channelID, messageTS, actorID string, state *slackgo.ViewState) error
}

// QuickActionsInteractionsUC is the surface needed to react to the
// inline quick-actions menu (assignee + status selects). Resolution of
// the underlying ticket happens inside the usecase via channel + thread
// timestamp, so the handler does not touch the registry.
type QuickActionsInteractionsUC interface {
	HandleAssigneeChange(ctx context.Context, channelID, threadTS string, userIDs []string) error
	HandleStatusChange(ctx context.Context, channelID, threadTS, statusID string) error
}

// slackInteractionsHandler handles Slack Block Kit interactivity callbacks
// (block_actions / view_submission). Slack delivers these as
// application/x-www-form-urlencoded with a single "payload" field carrying
// the JSON. The signature middleware (shared with /event) has already
// validated the request body bytes by the time we run.
func slackInteractionsHandler(triageUC TriageInteractionsUC, quickUC QuickActionsInteractionsUC) http.HandlerFunc {
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

		switch cb.Type {
		case slackgo.InteractionTypeBlockActions:
			handleBlockActions(ctx, w, triageUC, quickUC, &cb)
		case slackgo.InteractionTypeViewSubmission:
			handleViewSubmission(ctx, w, triageUC, &cb)
		default:
			logger.Debug("slack interaction ignored: unsupported type",
				slog.String("type", string(cb.Type)),
			)
			w.WriteHeader(http.StatusOK)
		}
	}
}

func handleBlockActions(ctx context.Context, w http.ResponseWriter, triageUC TriageInteractionsUC, quickUC QuickActionsInteractionsUC, cb *slackgo.InteractionCallback) {
	logger := logging.From(ctx)

	// Quick-actions selects (assignee / status) and triage buttons can in
	// principle co-arrive in the same payload, though Slack's UI delivers
	// at most one action per click. Iterate once, classify, and dispatch
	// each match. Quick-actions resolve the underlying ticket from
	// channel + thread_ts (the menu lives as a thread reply on the
	// ticket's root message), so we forward those raw fields.
	channelID := cb.Channel.ID
	threadTS := cb.Message.ThreadTimestamp
	if threadTS == "" {
		threadTS = cb.Message.Timestamp
	}

	var triageAction *slackgo.BlockAction
	var quickAssignee *slackgo.BlockAction
	var quickStatus *slackgo.BlockAction

	for i := range cb.ActionCallback.BlockActions {
		ba := cb.ActionCallback.BlockActions[i]
		switch ba.ActionID {
		case slackService.TriageSubmitActionID,
			slackService.TriageRetryActionID,
			slackService.TriageReviewEditActionID,
			slackService.TriageReviewSubmitActionID,
			slackService.TriageReviewReinvestigateActionID:
			if triageAction == nil {
				triageAction = ba
			}
		case slackService.QuickActionsAssigneeActionID:
			quickAssignee = ba
		case slackService.QuickActionsStatusActionID:
			quickStatus = ba
		}
	}

	if triageAction == nil && quickAssignee == nil && quickStatus == nil {
		logger.Debug("slack interaction ignored: no recognised action")
		w.WriteHeader(http.StatusOK)
		return
	}

	if triageAction != nil {
		ticketID := types.TicketID(triageAction.Value)
		messageTS := cb.Message.Timestamp
		triggerID := cb.TriggerID
		actionID := triageAction.ActionID
		actorID := cb.User.ID
		state := cb.BlockActionState

		// Edit / Re-investigate must call OpenView synchronously to honor
		// Slack's trigger_id deadline (~3s). Submit-style buttons follow
		// the existing 200-then-async pattern.
		switch actionID {
		case slackService.TriageReviewEditActionID:
			if err := triageUC.HandleReviewEditOpen(ctx, ticketID, channelID, messageTS, triggerID); err != nil {
				errutil.Handle(ctx, goerr.Wrap(err, "review edit open"))
			}
			w.WriteHeader(http.StatusOK)
			return
		case slackService.TriageReviewReinvestigateActionID:
			if err := triageUC.HandleReviewReinvestigateOpen(ctx, ticketID, channelID, messageTS, triggerID); err != nil {
				errutil.Handle(ctx, goerr.Wrap(err, "review reinvestigate open"))
			}
			w.WriteHeader(http.StatusOK)
			return
		}

		w.WriteHeader(http.StatusOK)

		async.Dispatch(ctx, func(ctx context.Context) error {
			switch actionID {
			case slackService.TriageSubmitActionID:
				return triageUC.HandleSubmit(ctx, triage.Submission{
					TicketID:  ticketID,
					ChannelID: channelID,
					MessageTS: messageTS,
					State:     state,
				})
			case slackService.TriageRetryActionID:
				return triageUC.HandleRetry(ctx, ticketID, channelID, messageTS)
			case slackService.TriageReviewSubmitActionID:
				return triageUC.HandleReviewSubmit(ctx, ticketID, channelID, messageTS, actorID)
			}
			return nil
		})
		return
	}

	// Quick-actions path — ack first, then dispatch.
	w.WriteHeader(http.StatusOK)

	if quickUC == nil {
		logger.Debug("quick action ignored: usecase not configured")
		return
	}

	if quickAssignee != nil {
		userIDs := append([]string(nil), quickAssignee.SelectedUsers...)
		async.Dispatch(ctx, func(ctx context.Context) error {
			return quickUC.HandleAssigneeChange(ctx, channelID, threadTS, userIDs)
		})
	}
	if quickStatus != nil {
		statusID := ""
		if quickStatus.SelectedOption.Value != "" {
			statusID = quickStatus.SelectedOption.Value
		}
		async.Dispatch(ctx, func(ctx context.Context) error {
			return quickUC.HandleStatusChange(ctx, channelID, threadTS, statusID)
		})
	}
}

func handleViewSubmission(ctx context.Context, w http.ResponseWriter, uc TriageInteractionsUC, cb *slackgo.InteractionCallback) {
	logger := logging.From(ctx)

	switch cb.View.CallbackID {
	case slackService.TriageReviewEditModalCallbackID:
		meta, err := slackService.DecodeTriageReviewModalMetadata(cb.View.PrivateMetadata)
		if err != nil {
			errutil.HandleHTTP(ctx, w, goerr.Wrap(err, "decode edit modal metadata"), http.StatusBadRequest)
			return
		}
		// Edit submit must run inline rather than fire-and-forget because we
		// might need to return a response_action: errors payload synchronously.
		state := cb.View.State
		fieldErrs, err := uc.HandleReviewEditSubmit(ctx, meta.TicketID, meta.ChannelID, meta.MessageTS, cb.User.ID, state)
		if err != nil && errors.Is(err, triage.ErrReviewFieldRequired) {
			respondViewErrors(ctx, w, fieldErrs)
			return
		}
		if err != nil {
			errutil.HandleHTTP(ctx, w, goerr.Wrap(err, "handle review edit submit"), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	case slackService.TriageReviewReinvestigateModalCallbackID:
		meta, err := slackService.DecodeTriageReviewModalMetadata(cb.View.PrivateMetadata)
		if err != nil {
			errutil.HandleHTTP(ctx, w, goerr.Wrap(err, "decode reinvestigate modal metadata"), http.StatusBadRequest)
			return
		}
		// Acknowledge first; the planner re-dispatch and follow-up post are
		// done asynchronously inside HandleReviewReinvestigate via async.Dispatch.
		state := cb.View.State
		actor := cb.User.ID
		w.WriteHeader(http.StatusOK)
		async.Dispatch(ctx, func(ctx context.Context) error {
			return uc.HandleReviewReinvestigate(ctx, meta.TicketID, meta.ChannelID, meta.MessageTS, actor, state)
		})
	default:
		logger.Debug("slack view submission ignored: unknown callback",
			slog.String("callback_id", cb.View.CallbackID),
		)
		w.WriteHeader(http.StatusOK)
	}
}

// respondViewErrors writes Slack's view_submission `response_action: errors`
// payload so the modal stays open and renders inline error messages keyed by
// block_id.
func respondViewErrors(ctx context.Context, w http.ResponseWriter, fieldErrs triage.ReviewFieldErrors) {
	body := struct {
		ResponseAction string            `json:"response_action"`
		Errors         map[string]string `json:"errors"`
	}{
		ResponseAction: "errors",
		Errors:         fieldErrs,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(&body); err != nil {
		errutil.Handle(ctx, goerr.Wrap(err, "encode view errors response"))
	}
}
