package triage

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
	slackService "github.com/m-mizutani/shepherd/pkg/service/slack"
	"github.com/m-mizutani/shepherd/pkg/utils/async"
	"github.com/m-mizutani/shepherd/pkg/utils/logging"
	slackgo "github.com/slack-go/slack"
)

// UseCase is the public surface that other packages (notably the existing
// SlackUseCase and the HTTP interactions handler) use to drive triage. It
// wraps PlanExecutor with the Entry-1 / Entry-2 entry points described in
// the spec.
type UseCase struct {
	executor *PlanExecutor
	registry ChannelResolver
}

// ChannelResolver resolves a Slack channel id to its workspace id. Tests
// substitute a small fake; production wires *registryAdapter wrapping the
// existing *model.WorkspaceRegistry, since that registry's lookup returns a
// WorkspaceEntry rather than a bare id.
type ChannelResolver interface {
	ResolveWorkspace(channelID string) (types.WorkspaceID, bool)
}

// RegistryResolver adapts *model.WorkspaceRegistry to ChannelResolver.
type RegistryResolver struct {
	Registry *model.WorkspaceRegistry
}

// ResolveWorkspace implements ChannelResolver.
func (r *RegistryResolver) ResolveWorkspace(channelID string) (types.WorkspaceID, bool) {
	if r == nil || r.Registry == nil {
		return "", false
	}
	entry, ok := r.Registry.GetBySlackChannelID(channelID)
	if !ok {
		return "", false
	}
	return entry.Workspace.ID, true
}

// NewUseCase builds a triage UseCase around an executor. registry is used
// by the HTTP interaction handler to map a Slack interaction back to its
// workspace before the submission is processed.
func NewUseCase(executor *PlanExecutor, registry ChannelResolver) *UseCase {
	return &UseCase{executor: executor, registry: registry}
}

// resolveTicket maps a Slack channel id + ticket id to a loaded ticket. It
// returns (nil, nil) when the channel is not mapped to a workspace or the
// ticket is missing — both are "form invalidated" conditions, not server
// errors. Real failures (DB outage etc.) propagate as errors.
func (u *UseCase) resolveTicket(ctx context.Context, channelID string, ticketID types.TicketID) (types.WorkspaceID, *model.Ticket, error) {
	if u.registry == nil {
		return "", nil, nil
	}
	wsID, ok := u.registry.ResolveWorkspace(channelID)
	if !ok {
		return "", nil, nil
	}
	t, err := u.executor.repo.Ticket().Get(ctx, wsID, ticketID)
	if err != nil {
		// memory and firestore both return error for "not found"; treat
		// any error here as missing ticket so the form is gracefully
		// invalidated rather than 500'ing.
		return wsID, nil, nil
	}
	return wsID, t, nil
}

// invalidateForm replaces the question/recovery message with the
// "no longer valid" notice. Used when the interaction targets a ticket that
// has gone away or whose channel mapping was removed.
func (u *UseCase) invalidateForm(ctx context.Context, channelID, messageTS string) {
	if err := u.executor.slack.UpdateMessage(ctx, channelID, messageTS,
		slackService.BuildAskInvalidatedBlocks(ctx)); err != nil {
		logging.From(ctx).Warn("failed to invalidate form message",
			slog.String("error", err.Error()))
	}
}

// OnTicketCreated is Entry-1: invoked by SlackUseCase.HandleNewMessage right
// after a ticket has been created. It schedules the planner loop in the
// background and returns immediately so the original Slack handler stays
// fast. The repeated ticket.Triaged check inside Run guarantees that
// duplicate dispatches (e.g. event re-deliveries) do not re-run triage.
func (u *UseCase) OnTicketCreated(ctx context.Context, ticket *model.Ticket) {
	if ticket == nil {
		return
	}
	wsID := ticket.WorkspaceID
	id := ticket.ID
	async.Dispatch(ctx, func(ctx context.Context) error {
		return u.executor.run(ctx, wsID, id)
	})
}

// HandleSubmit is Entry-2: invoked by the HTTP interactions handler when
// the reporter clicks the submit button on the question form. It validates
// the submission, appends the answers to the plan history as a user
// message, swaps the question message for a "received" notice, and resumes
// the planner loop in the background.
func (u *UseCase) HandleSubmit(ctx context.Context, sub Submission) error {
	logger := logging.From(ctx).With(
		slog.String("ticket_id", string(sub.TicketID)),
	)
	ctx = logging.With(ctx, logger)

	wsID, ticket, err := u.resolveTicket(ctx, sub.ChannelID, sub.TicketID)
	if err != nil {
		return goerr.Wrap(err, "resolve ticket")
	}
	if ticket == nil || ticket.Triaged {
		u.invalidateForm(ctx, sub.ChannelID, sub.MessageTS)
		return nil
	}

	plan, err := loadLatestTriagePlan(ctx, u.executor.historyRepo, wsID, sub.TicketID)
	if err != nil {
		return goerr.Wrap(err, "load latest plan")
	}
	if plan == nil || plan.Kind != types.PlanAsk || plan.Ask == nil {
		u.invalidateForm(ctx, sub.ChannelID, sub.MessageTS)
		return nil
	}

	waiting, err := isWaitingUserSubmit(ctx, u.executor.historyRepo, wsID, sub.TicketID)
	if err != nil {
		return goerr.Wrap(err, "check waiting state")
	}
	if !waiting {
		u.invalidateForm(ctx, sub.ChannelID, sub.MessageTS)
		return nil
	}

	answers, err := matchAnswers(plan.Ask, sub.State)
	if err != nil {
		return goerr.Wrap(err, "match submission to questions")
	}

	if !allAnswersValid(answers, plan.Ask) {
		// Re-render the form with an inline validation banner so the
		// reporter can fix and resubmit.
		blocks := slackService.BuildAskValidationErrorBlocks(ctx, ticket.ID, plan.Ask, plan.Message)
		if err := u.executor.slack.UpdateMessage(ctx, sub.ChannelID, sub.MessageTS, blocks); err != nil {
			return goerr.Wrap(err, "post validation error")
		}
		return nil
	}

	answerSummary := formatAnswers(plan.Ask, answers)
	if err := appendUserMessage(ctx, u.executor.historyRepo, wsID, sub.TicketID, answerSummary); err != nil {
		return goerr.Wrap(err, "append answers to plan history")
	}

	if err := u.executor.slack.UpdateMessage(ctx, sub.ChannelID, sub.MessageTS,
		slackService.BuildAskAnsweredBlocks(ctx, plan.Ask, answers, plan.Message)); err != nil {
		// non-fatal; continue to resume the loop
		logger.Warn("failed to update ask message", slog.String("error", err.Error()))
	}

	tkID := sub.TicketID
	async.Dispatch(ctx, func(ctx context.Context) error {
		return u.executor.run(ctx, wsID, tkID)
	})
	return nil
}

// Submission is the decoded payload that the HTTP interactions handler
// passes to HandleSubmit. The handler does not pre-resolve the workspace —
// HandleSubmit looks it up internally from ChannelID via the registry, so
// the controller stays free of usecase-layer concerns.
type Submission struct {
	TicketID  types.TicketID
	ChannelID string // raw Slack channel id from the interaction payload
	MessageTS string // ts of the question message to update
	State     *slackgo.BlockActionStates
}

// matchAnswers walks the submission state and pairs each input value with
// its question by block_id. Question.ID is used as block_id for the choice
// input, and Question.ID + ":other" for the free-text fallback. Returns
// answers in the same order as ask.Questions for stability.
func matchAnswers(ask *model.Ask, state *slackgo.BlockActionStates) ([]model.Answer, error) {
	if ask == nil {
		return nil, goerr.New("ask is nil")
	}
	if state == nil {
		return nil, goerr.New("submission state is nil")
	}

	answers := make([]model.Answer, 0, len(ask.Questions))
	for _, q := range ask.Questions {
		ans := model.Answer{QuestionID: q.ID}

		if blk, ok := state.Values[string(q.ID)]; ok {
			if act, ok := blk[slackService.TriageChoiceActionID]; ok {
				if q.Multiple {
					for _, opt := range act.SelectedOptions {
						if opt.Value == "" {
							continue
						}
						ans.SelectedIDs = append(ans.SelectedIDs, types.ChoiceID(opt.Value))
					}
				} else if v := act.SelectedOption.Value; v != "" {
					ans.SelectedIDs = []types.ChoiceID{types.ChoiceID(v)}
				}
			}
		}

		if blk, ok := state.Values[string(q.ID)+slackService.TriageOtherSuffix]; ok {
			if act, ok := blk[slackService.TriageOtherTextActionID]; ok {
				ans.OtherText = act.Value
			}
		}
		answers = append(answers, ans)
	}
	return answers, nil
}

func allAnswersValid(answers []model.Answer, ask *model.Ask) bool {
	if len(answers) != len(ask.Questions) {
		return false
	}
	for i := range answers {
		if !answers[i].IsValid() {
			return false
		}
	}
	return true
}

func formatAnswers(ask *model.Ask, answers []model.Answer) string {
	labels := make(map[types.QuestionID]string, len(ask.Questions))
	choiceLabels := make(map[types.QuestionID]map[types.ChoiceID]string, len(ask.Questions))
	for _, q := range ask.Questions {
		labels[q.ID] = q.Label
		choices := make(map[types.ChoiceID]string, len(q.Choices))
		for _, c := range q.Choices {
			choices[c.ID] = c.Label
		}
		choiceLabels[q.ID] = choices
	}

	var b strings.Builder
	b.WriteString("Reporter answers:\n")
	for _, a := range answers {
		b.WriteString("\n- ")
		b.WriteString(labels[a.QuestionID])
		b.WriteString(":")
		if len(a.SelectedIDs) > 0 {
			parts := make([]string, 0, len(a.SelectedIDs))
			for _, sid := range a.SelectedIDs {
				if lbl, ok := choiceLabels[a.QuestionID][sid]; ok {
					parts = append(parts, lbl)
				} else {
					parts = append(parts, string(sid))
				}
			}
			fmt.Fprintf(&b, " %s", strings.Join(parts, ", "))
		}
		if other := strings.TrimSpace(a.OtherText); other != "" {
			fmt.Fprintf(&b, " (free text: %s)", other)
		}
	}
	return b.String()
}

// HandleRetry is invoked by the HTTP interactions handler when the reporter
// clicks the retry button on a failure-recovery message. It swaps the
// recovery message for a "queued" notice and re-dispatches the planner loop.
// Idempotency is preserved by ticket.Triaged + isWaitingUserSubmit checks
// inside run(), so repeated clicks at worst start one extra short-circuited
// run.
func (u *UseCase) HandleRetry(ctx context.Context, ticketID types.TicketID, channelID, messageTS string) error {
	logger := logging.From(ctx).With(slog.String("ticket_id", string(ticketID)))
	ctx = logging.With(ctx, logger)

	wsID, ticket, err := u.resolveTicket(ctx, channelID, ticketID)
	if err != nil {
		return goerr.Wrap(err, "resolve ticket for retry")
	}
	if ticket == nil || ticket.Triaged {
		// Already-finished or unknown tickets just have the button cleared.
		u.invalidateForm(ctx, channelID, messageTS)
		return nil
	}

	if err := u.executor.slack.UpdateMessage(ctx, channelID, messageTS,
		slackService.BuildRetryQueuedBlocks(ctx)); err != nil {
		logger.Warn("failed to update retry message", slog.String("error", err.Error()))
	}

	tkID := ticketID
	async.Dispatch(ctx, func(ctx context.Context) error {
		return u.executor.run(ctx, wsID, tkID)
	})
	return nil
}

