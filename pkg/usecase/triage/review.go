package triage

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"strings"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
	domainConfig "github.com/m-mizutani/shepherd/pkg/domain/model/config"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
	slackService "github.com/m-mizutani/shepherd/pkg/service/slack"
	"github.com/m-mizutani/shepherd/pkg/utils/async"
	"github.com/m-mizutani/shepherd/pkg/utils/i18n"
	"github.com/m-mizutani/shepherd/pkg/utils/logging"
	slackgo "github.com/slack-go/slack"
)

// HandleReviewEditOpen opens the Edit modal in response to a click on the
// review message's "Edit" button. Slack's trigger_id has a ~3-second TTL, so
// callers (the HTTP handler) MUST invoke this synchronously before
// acknowledging the interaction.
func (u *UseCase) HandleReviewEditOpen(ctx context.Context, ticketID types.TicketID, channelID, messageTS, triggerID string) error {
	logger := logging.From(ctx).With(slog.String("ticket_id", string(ticketID)))
	ctx = logging.With(ctx, logger)

	wsID, ticket, err := u.resolveTicket(ctx, channelID, ticketID)
	if err != nil {
		return goerr.Wrap(err, "resolve ticket for review edit")
	}
	if ticket == nil {
		return nil
	}
	if ticket.Triaged {
		u.notifyAlreadyFinalized(ctx, channelID, ticket.ReporterSlackUserID)
		return nil
	}

	plan, err := loadLatestTriagePlan(ctx, u.executor.historyRepo, wsID, ticketID)
	if err != nil {
		return goerr.Wrap(err, "load latest plan for review edit")
	}
	if plan == nil || plan.Kind != types.PlanComplete || plan.Complete == nil {
		u.notifyMissingProposal(ctx, channelID, ticket.ReporterSlackUserID)
		return nil
	}

	schema := u.workspaceSchema(wsID)
	view, err := slackService.BuildReviewEditModal(ctx, slackService.TriageReviewModalMetadata{
		TicketID: ticketID, ChannelID: channelID, MessageTS: messageTS,
	}, plan.Complete, schema, ticket.FieldValues)
	if err != nil {
		return goerr.Wrap(err, "build edit modal")
	}
	if _, err := u.executor.slack.OpenView(ctx, triggerID, view); err != nil {
		return goerr.Wrap(err, "open edit modal")
	}
	return nil
}

// HandleReviewReinvestigateOpen opens the Re-investigate instruction modal.
// Same trigger_id deadline rules as HandleReviewEditOpen.
func (u *UseCase) HandleReviewReinvestigateOpen(ctx context.Context, ticketID types.TicketID, channelID, messageTS, triggerID string) error {
	logger := logging.From(ctx).With(slog.String("ticket_id", string(ticketID)))
	ctx = logging.With(ctx, logger)

	_, ticket, err := u.resolveTicket(ctx, channelID, ticketID)
	if err != nil {
		return goerr.Wrap(err, "resolve ticket for reinvestigate")
	}
	if ticket == nil {
		return nil
	}
	if ticket.Triaged {
		u.notifyAlreadyFinalized(ctx, channelID, ticket.ReporterSlackUserID)
		return nil
	}

	view, err := slackService.BuildReviewReinvestigateModal(ctx, slackService.TriageReviewModalMetadata{
		TicketID: ticketID, ChannelID: channelID, MessageTS: messageTS,
	})
	if err != nil {
		return goerr.Wrap(err, "build reinvestigate modal")
	}
	if _, err := u.executor.slack.OpenView(ctx, triggerID, view); err != nil {
		return goerr.Wrap(err, "open reinvestigate modal")
	}
	return nil
}

// HandleReviewSubmit finalises the latest planner proposal as-is. Side effects
// in this order: persist the Complete onto the ticket, deactivate the buttons
// on the original review message via chat.update, and post the LLM-generated
// hand-off message as a new thread reply.
func (u *UseCase) HandleReviewSubmit(ctx context.Context, ticketID types.TicketID, channelID, messageTS, actorID string) error {
	logger := logging.From(ctx).With(slog.String("ticket_id", string(ticketID)))
	ctx = logging.With(ctx, logger)

	wsID, ticket, err := u.resolveTicket(ctx, channelID, ticketID)
	if err != nil {
		return goerr.Wrap(err, "resolve ticket for review submit")
	}
	if ticket == nil {
		return nil
	}
	if ticket.Triaged {
		u.notifyAlreadyFinalized(ctx, channelID, ticket.ReporterSlackUserID)
		return nil
	}

	plan, err := loadLatestTriagePlan(ctx, u.executor.historyRepo, wsID, ticketID)
	if err != nil {
		return goerr.Wrap(err, "load latest plan for review submit")
	}
	if plan == nil || plan.Kind != types.PlanComplete || plan.Complete == nil {
		u.notifyMissingProposal(ctx, channelID, ticket.ReporterSlackUserID)
		return nil
	}

	if err := u.executor.finalizeComplete(ctx, ticket, plan.Complete); err != nil {
		return goerr.Wrap(err, "finalize complete from review submit")
	}
	u.deactivateReviewMessage(ctx, ticket, channelID, messageTS, plan.Complete, slackService.ReviewActionedSubmitted, types.SlackUserID(actorID), u.workspaceSchema(wsID), ticket.FieldValues)
	return u.postHandoff(ctx, ticket, plan.Complete)
}

// ErrReviewFieldRequired marks a missing required custom-field input. The
// HTTP handler turns this into a Slack response_action: errors payload so the
// modal stays open for the user to fix.
var ErrReviewFieldRequired = errors.New("required field missing")

// ReviewFieldErrors carries per-block_id error messages destined for Slack's
// response_action: errors payload. The HTTP handler is responsible for the
// JSON shape; the usecase only owns the (block_id → message) map.
type ReviewFieldErrors map[string]string

// HandleReviewEditSubmit parses the Edit modal's view state, persists the
// edited field values to the ticket, and finalises the ticket using the
// edited title / summary / assignee / suggested_fields. Like HandleReviewSubmit,
// it deactivates the original review message and posts the LLM hand-off.
// Required-field validation failures return ErrReviewFieldRequired with a
// populated ReviewFieldErrors, which the HTTP handler should surface as
// Slack's response_action: errors.
func (u *UseCase) HandleReviewEditSubmit(ctx context.Context, ticketID types.TicketID, channelID, messageTS, actorID string, state *slackgo.ViewState) (ReviewFieldErrors, error) {
	logger := logging.From(ctx).With(slog.String("ticket_id", string(ticketID)))
	ctx = logging.With(ctx, logger)

	wsID, ticket, err := u.resolveTicket(ctx, channelID, ticketID)
	if err != nil {
		return nil, goerr.Wrap(err, "resolve ticket for edit submit")
	}
	if ticket == nil {
		return nil, nil
	}
	if ticket.Triaged {
		u.notifyAlreadyFinalized(ctx, channelID, ticket.ReporterSlackUserID)
		return nil, nil
	}

	plan, err := loadLatestTriagePlan(ctx, u.executor.historyRepo, wsID, ticketID)
	if err != nil {
		return nil, goerr.Wrap(err, "load latest plan for edit submit")
	}
	if plan == nil || plan.Kind != types.PlanComplete || plan.Complete == nil {
		u.notifyMissingProposal(ctx, channelID, ticket.ReporterSlackUserID)
		return nil, nil
	}

	schema := u.workspaceSchema(wsID)
	edited, fieldValues, fieldErrs := applyEditModalState(ctx, plan.Complete, schema, state)
	if len(fieldErrs) > 0 {
		return fieldErrs, ErrReviewFieldRequired
	}

	// Slack's view_submission has a hard 3-second deadline. Validation has
	// already happened above (it must run sync so we can return
	// response_action: errors), but the heavy tail — finalize + LLM
	// hand-off + chat.update + PostThreadBlocks — easily blows the deadline,
	// causing Slack to render "We had some trouble connecting." even though
	// the work would have eventually succeeded. Push the tail into
	// async.Dispatch so the modal closes promptly.
	wsIDCopy := wsID
	editedCopy := edited
	fvCopy := fieldValues
	channelCopy := channelID
	messageTSCopy := messageTS
	actorCopy := actorID
	async.Dispatch(ctx, func(ctx context.Context) error {
		// Re-load the ticket inside the async tail because we may have left
		// the originating request context (and to avoid mutating the caller's
		// pointer concurrently with anything else).
		t, err := u.executor.repo.Ticket().Get(ctx, wsIDCopy, ticketID)
		if err != nil {
			return goerr.Wrap(err, "reload ticket for edit submit tail")
		}
		if t == nil || t.Triaged {
			// Lost a race with another finalisation; nothing to do.
			return nil
		}
		if len(fvCopy) > 0 {
			t.FieldValues = fvCopy
			if _, err := u.executor.repo.Ticket().Update(ctx, wsIDCopy, t); err != nil {
				return goerr.Wrap(err, "persist edited field values")
			}
		}
		if err := u.executor.finalizeComplete(ctx, t, editedCopy); err != nil {
			return goerr.Wrap(err, "finalize complete from edit submit")
		}
		u.deactivateReviewMessage(ctx, t, channelCopy, messageTSCopy, editedCopy, slackService.ReviewActionedSubmitted, types.SlackUserID(actorCopy), u.workspaceSchema(wsIDCopy), t.FieldValues)
		return u.postHandoff(ctx, t, editedCopy)
	})
	return nil, nil
}

// HandleReviewReinvestigate appends the user's instruction to the planner's
// gollem history, deactivates the buttons on the original review message
// (so users land on the freshly posted re-investigation banner / next review
// message instead), and re-dispatches the planner.
func (u *UseCase) HandleReviewReinvestigate(ctx context.Context, ticketID types.TicketID, channelID, messageTS, actorID string, state *slackgo.ViewState) error {
	logger := logging.From(ctx).With(slog.String("ticket_id", string(ticketID)))
	ctx = logging.With(ctx, logger)

	wsID, ticket, err := u.resolveTicket(ctx, channelID, ticketID)
	if err != nil {
		return goerr.Wrap(err, "resolve ticket for reinvestigate")
	}
	if ticket == nil {
		return nil
	}
	if ticket.Triaged {
		u.notifyAlreadyFinalized(ctx, channelID, ticket.ReporterSlackUserID)
		return nil
	}

	instruction := extractInstruction(state)
	if strings.TrimSpace(instruction) == "" {
		// Empty input is not actionable; no-op silently. Slack's modal would
		// normally surface a required-field error, but the input is optional
		// from the modal's perspective.
		return nil
	}

	if err := appendUserMessage(ctx, u.executor.historyRepo, wsID, ticketID, instruction); err != nil {
		return goerr.Wrap(err, "append reinvestigate instruction")
	}

	// Deactivate the original review message's buttons so the click cannot be
	// repeated while the planner is busy. The latest Complete proposal in
	// history is what the buttons referenced; reuse it for the body so the
	// rendered content stays accurate.
	if plan, err := loadLatestTriagePlan(ctx, u.executor.historyRepo, wsID, ticketID); err == nil && plan != nil && plan.Complete != nil {
		u.deactivateReviewMessage(ctx, ticket, channelID, messageTS, plan.Complete, slackService.ReviewActionedReinvestigate, types.SlackUserID(actorID), u.workspaceSchema(wsID), ticket.FieldValues)
	}

	blocks := slackService.BuildReviewReinvestigatingBlocks(ctx, u.ticketRef(ticket), instruction)
	if _, err := u.executor.slack.PostThreadBlocks(ctx, channelID, string(ticket.SlackThreadTS), blocks); err != nil {
		// Best effort: log and continue. The planner restart is the load-bearing
		// effect; the follow-up message is informational.
		logger.Warn("failed to post reinvestigating follow-up", slog.String("error", err.Error()))
	}

	tkID := ticketID
	async.Dispatch(ctx, func(ctx context.Context) error {
		return u.executor.run(ctx, wsID, tkID)
	})
	return nil
}

// postHandoff posts the LLM-generated assignee call-to-action as a fresh
// thread reply. Failures here do not roll back the (already persisted)
// finalize; we surface them to errutil via the caller's return.
func (u *UseCase) postHandoff(ctx context.Context, ticket *model.Ticket, comp *model.Complete) error {
	message := u.executor.generateHandoffMessage(ctx, comp)
	blocks := slackService.BuildHandoffMessageBlocks(ctx, u.ticketRef(ticket), message)
	if _, err := u.executor.slack.PostThreadBlocks(ctx, string(ticket.SlackChannelID), string(ticket.SlackThreadTS), blocks); err != nil {
		return goerr.Wrap(err, "post triage hand-off message")
	}
	return nil
}

// deactivateReviewMessage rewrites the original review message in place so
// its buttons disappear once anyone has actioned it. Failures are logged and
// swallowed: the message-update failing is annoying but not load-bearing — the
// idempotency check (Triaged flag) still protects against duplicate finalises.
func (u *UseCase) deactivateReviewMessage(ctx context.Context, ticket *model.Ticket, channelID, messageTS string, comp *model.Complete, kind slackService.ReviewActionedKind, actor types.SlackUserID, schema *domainConfig.FieldSchema, fieldValues map[string]model.FieldValue) {
	if messageTS == "" {
		return
	}
	blocks := slackService.BuildReviewActionedBlocks(ctx, u.ticketRef(ticket), comp, kind, actor, schema, fieldValues)
	if err := u.executor.slack.UpdateMessage(ctx, channelID, messageTS, blocks); err != nil {
		logging.From(ctx).Warn("failed to deactivate review message",
			slog.String("error", err.Error()),
			slog.String("channel_id", channelID),
			slog.String("message_ts", messageTS))
	}
}

func (u *UseCase) notifyAlreadyFinalized(ctx context.Context, channelID string, userID types.SlackUserID) {
	if err := u.executor.slack.PostEphemeral(ctx, channelID, string(userID),
		i18n.From(ctx).T(i18n.MsgTriageReviewAlreadyFinalized)); err != nil {
		logging.From(ctx).Warn("failed to send already-finalized ephemeral",
			slog.String("error", err.Error()))
	}
}

func (u *UseCase) notifyMissingProposal(ctx context.Context, channelID string, userID types.SlackUserID) {
	if err := u.executor.slack.PostEphemeral(ctx, channelID, string(userID),
		i18n.From(ctx).T(i18n.MsgTriageReviewMissingProposal)); err != nil {
		logging.From(ctx).Warn("failed to send missing-proposal ephemeral",
			slog.String("error", err.Error()))
	}
}

// workspaceSchema returns the workspace's FieldSchema, or nil when the
// registry implementation doesn't expose one (e.g. in some tests). Code paths
// that need the schema treat nil as "no custom fields".
func (u *UseCase) workspaceSchema(wsID types.WorkspaceID) *domainConfig.FieldSchema {
	type schemaProvider interface {
		WorkspaceSchema(types.WorkspaceID) *domainConfig.FieldSchema
	}
	if sp, ok := u.registry.(schemaProvider); ok {
		return sp.WorkspaceSchema(wsID)
	}
	return nil
}

// applyEditModalState merges a view-state snapshot into the planner's
// proposal, producing the edited *Complete plus the parsed FieldValues that
// should be persisted on the ticket. Unknown / blank fields fall through to
// the existing values from comp / current; required-field violations are
// returned as a populated ReviewFieldErrors.
func applyEditModalState(ctx context.Context, base *model.Complete, schema *domainConfig.FieldSchema, state *slackgo.ViewState) (*model.Complete, map[string]model.FieldValue, ReviewFieldErrors) {
	loc := i18n.From(ctx)
	out := *base // shallow copy; we replace pointer / slice fields below as needed.

	if state != nil {
		if v, ok := lookupAction(state, slackService.TriageReviewTitleBlockID, slackService.TriageReviewTitleActionID); ok {
			if s := strings.TrimSpace(v.Value); s != "" {
				out.Title = s
			}
		}
		if v, ok := lookupAction(state, slackService.TriageReviewSummaryBlockID, slackService.TriageReviewSummaryActionID); ok {
			if s := strings.TrimSpace(v.Value); s != "" {
				out.Summary = s
			}
		}
		if v, ok := lookupAction(state, slackService.TriageReviewAssigneeBlockID, slackService.TriageReviewAssigneeActionID); ok {
			if u := v.SelectedUser; u != "" {
				uid := types.SlackUserID(u)
				out.Assignee = model.AssigneeDecision{
					Kind:      types.AssigneeAssigned,
					UserID:    &uid,
					Reasoning: base.Assignee.Reasoning,
				}
			} else {
				// Cleared selection: reset to unassigned, preserving reasoning.
				out.Assignee = model.AssigneeDecision{
					Kind:      types.AssigneeUnassigned,
					Reasoning: base.Assignee.Reasoning,
				}
			}
		}
	}

	if schema == nil || len(schema.Fields) == 0 {
		return &out, nil, nil
	}

	fieldValues := make(map[string]model.FieldValue, len(schema.Fields))
	suggested := make(map[string]any, len(schema.Fields))
	var errs ReviewFieldErrors

	for _, f := range schema.Fields {
		var act *slackgo.BlockAction
		if state != nil {
			// action_id mirrors the per-field encoding used in
			// buildFieldInputBlock: "field_value::<id>".
			actionID := slackService.TriageReviewFieldValueAction + "_" + f.ID
			if v, ok := lookupAction(state, f.ID, actionID); ok {
				act = v
			}
		}
		val, raw, ok := parseFieldValue(f, act)
		if !ok {
			if f.Required {
				if errs == nil {
					errs = ReviewFieldErrors{}
				}
				errs[f.ID] = loc.T(i18n.MsgTriageReviewFieldRequiredError)
			}
			continue
		}
		fieldValues[f.ID] = val
		if raw != "" {
			suggested[f.ID] = raw
		}
	}

	if len(errs) > 0 {
		return nil, nil, errs
	}

	if len(suggested) > 0 {
		out.SuggestedFields = suggested
	}
	return &out, fieldValues, nil
}

func parseFieldValue(f domainConfig.FieldDefinition, act *slackgo.BlockAction) (model.FieldValue, string, bool) {
	if act == nil {
		return model.FieldValue{}, "", false
	}
	switch f.Type {
	case types.FieldTypeText, types.FieldTypeURL:
		s := strings.TrimSpace(act.Value)
		if s == "" {
			return model.FieldValue{}, "", false
		}
		return model.FieldValue{FieldID: types.FieldID(f.ID), Type: f.Type, Value: s}, s, true
	case types.FieldTypeNumber:
		s := strings.TrimSpace(act.Value)
		if s == "" {
			return model.FieldValue{}, "", false
		}
		// Number inputs round-trip as strings on the wire; we store the
		// canonical numeric form so downstream consumers can rely on it.
		if n, err := strconv.ParseFloat(s, 64); err == nil {
			return model.FieldValue{FieldID: types.FieldID(f.ID), Type: f.Type, Value: n}, s, true
		}
		return model.FieldValue{FieldID: types.FieldID(f.ID), Type: f.Type, Value: s}, s, true
	case types.FieldTypeDate:
		if act.SelectedDate == "" {
			return model.FieldValue{}, "", false
		}
		return model.FieldValue{FieldID: types.FieldID(f.ID), Type: f.Type, Value: act.SelectedDate}, act.SelectedDate, true
	case types.FieldTypeSelect:
		opt := act.SelectedOption.Value
		if opt == "" {
			return model.FieldValue{}, "", false
		}
		return model.FieldValue{FieldID: types.FieldID(f.ID), Type: f.Type, Value: opt}, opt, true
	case types.FieldTypeMultiSelect:
		ids := make([]string, 0, len(act.SelectedOptions))
		for _, o := range act.SelectedOptions {
			if o.Value != "" {
				ids = append(ids, o.Value)
			}
		}
		if len(ids) == 0 {
			return model.FieldValue{}, "", false
		}
		return model.FieldValue{FieldID: types.FieldID(f.ID), Type: f.Type, Value: ids}, strings.Join(ids, ","), true
	case types.FieldTypeUser:
		if act.SelectedUser == "" {
			return model.FieldValue{}, "", false
		}
		return model.FieldValue{FieldID: types.FieldID(f.ID), Type: f.Type, Value: act.SelectedUser}, act.SelectedUser, true
	case types.FieldTypeMultiUser:
		users := make([]string, 0, len(act.SelectedUsers))
		for _, u := range act.SelectedUsers {
			if u != "" {
				users = append(users, u)
			}
		}
		if len(users) == 0 {
			return model.FieldValue{}, "", false
		}
		return model.FieldValue{FieldID: types.FieldID(f.ID), Type: f.Type, Value: users}, strings.Join(users, ","), true
	}
	return model.FieldValue{}, "", false
}

func lookupAction(state *slackgo.ViewState, blockID, actionID string) (*slackgo.BlockAction, bool) {
	if state == nil {
		return nil, false
	}
	block, ok := state.Values[blockID]
	if !ok {
		return nil, false
	}
	act, ok := block[actionID]
	if !ok {
		return nil, false
	}
	return &act, true
}

func extractInstruction(state *slackgo.ViewState) string {
	if v, ok := lookupAction(state, slackService.TriageReviewInstructionBlock, slackService.TriageReviewInstructionAction); ok {
		return v.Value
	}
	return ""
}
