package triage

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/shepherd/pkg/domain/interfaces"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
	slackService "github.com/m-mizutani/shepherd/pkg/service/slack"
	"github.com/m-mizutani/shepherd/pkg/tool"
	"github.com/m-mizutani/shepherd/pkg/utils/logging"
)

// Config holds tunable parameters for the triage executor. Values come from
// CLI flags / env vars at startup; defaults are applied via defaultConfig.
type Config struct {
	IterationCap int
}

// defaultConfig returns the default triage executor configuration.
func defaultConfig() Config {
	return Config{IterationCap: 10}
}

// PlanExecutor drives a single triage's planning loop. One executor instance
// is shared across all in-flight triages; per-ticket state lives entirely in
// the agent history (gollem) and the ticket fields.
type PlanExecutor struct {
	repo        interfaces.Repository
	historyRepo gollem.HistoryRepository
	llm         gollem.LLMClient
	slack       SlackTriageClient
	catalog     *tool.Catalog
	cfg         Config
}

// SlackTriageClient is the slim Slack surface the triage usecase actually
// uses. Defined as an interface so tests can substitute a fake without
// reaching the real Slack API. Implemented by *service/slack.Client.
type SlackTriageClient interface {
	progressSlack
	ReplyThread(ctx context.Context, channelID, threadTS, text string) error
	PostEphemeral(ctx context.Context, channelID, userID, text string) error
}

// NewPlanExecutor wires the executor with its dependencies. All fields are
// required.
func NewPlanExecutor(repo interfaces.Repository, historyRepo gollem.HistoryRepository,
	llm gollem.LLMClient, slack SlackTriageClient, catalog *tool.Catalog, cfg Config) *PlanExecutor {
	if cfg.IterationCap <= 0 {
		cfg.IterationCap = defaultConfig().IterationCap
	}
	return &PlanExecutor{
		repo:        repo,
		historyRepo: historyRepo,
		llm:         llm,
		slack:       slack,
		catalog:     catalog,
		cfg:         cfg,
	}
}

// Run drives the planner loop for a single ticket from whatever state the
// agent history is currently in. It is the single entry point invoked by
// async.Dispatch from both the new-ticket trigger (Entry-1) and the submit
// resume (Entry-2). The function returns nil on natural pauses
// (waiting_user_submit, done, aborted).
func (e *PlanExecutor) run(ctx context.Context, workspaceID types.WorkspaceID, ticketID types.TicketID) (retErr error) {
	logger := logging.From(ctx).With(
		slog.String("workspace_id", string(workspaceID)),
		slog.String("ticket_id", string(ticketID)),
	)
	ctx = logging.With(ctx, logger)

	ticket, err := e.repo.Ticket().Get(ctx, workspaceID, ticketID)
	if err != nil {
		return goerr.Wrap(err, "load ticket", goerr.V("ticket_id", ticketID))
	}
	if ticket.Triaged {
		logger.Debug("triage skipped: already triaged")
		return nil
	}

	// Recovery: any abnormal exit (returned error or panic) posts a failure
	// notice with a retry button to the ticket thread, so the reporter is not
	// left waiting silently. The deferred handler runs *after* the planner
	// loop has decided what to do, so legitimate completions (Complete /
	// Ask-pause / Abort) are unaffected — they return nil and skip the body.
	defer func() {
		if r := recover(); r != nil {
			retErr = goerr.New(fmt.Sprintf("panic in triage run: %v", r))
		}
		if retErr == nil {
			return
		}
		e.reportFailure(ctx, ticket, retErr)
	}()

	// Waiting for user submit? The submit handler will resume the loop.
	waiting, err := isWaitingUserSubmit(ctx, e.historyRepo, workspaceID, ticketID)
	if err != nil {
		return goerr.Wrap(err, "check waiting state")
	}
	if waiting {
		logger.Debug("triage paused: waiting for user submit")
		return nil
	}

	for {
		count, err := countPlannerTurns(ctx, e.historyRepo, workspaceID, ticketID)
		if err != nil {
			return goerr.Wrap(err, "count plan turns")
		}
		if count >= e.cfg.IterationCap {
			logger.Warn("triage aborted: iteration cap exceeded",
				slog.Int("count", count),
				slog.Int("cap", e.cfg.IterationCap),
			)
			return e.finalizeAbort(ctx, ticket, "iteration cap exceeded")
		}

		plan, err := e.llmPlan(ctx, ticket)
		if err != nil {
			return goerr.Wrap(err, "llmPlan")
		}

		switch plan.Kind {
		case types.PlanInvestigate:
			progress, perr := newProgressMessage(ctx, e.slack, ticket.SlackChannelID, ticket.SlackThreadTS, plan.Message, plan.Investigate.Subtasks)
			if perr != nil {
				return goerr.Wrap(perr, "post progress message")
			}
			if err := e.runInvestigate(ctx, ticket, plan, progress); err != nil {
				return goerr.Wrap(err, "investigate")
			}
			// Loop back: ask the LLM what to do next.
			continue

		case types.PlanAsk:
			if err := e.postAsk(ctx, ticket, plan); err != nil {
				return goerr.Wrap(err, "post ask")
			}
			logger.Info("triage paused: ask form posted")
			return nil

		case types.PlanComplete:
			if err := e.finalizeComplete(ctx, ticket, plan); err != nil {
				return goerr.Wrap(err, "finalize complete")
			}
			logger.Info("triage completed")
			return nil

		default:
			return goerr.New("unknown plan kind", goerr.V("kind", plan.Kind))
		}
	}
}

// reportFailure posts the failure-recovery message (error summary + retry
// button) to the ticket thread. Invoked from run()'s deferred handler when
// the planner exits abnormally; it must never propagate further errors —
// they are logged so the deferred path stays robust.
func (e *PlanExecutor) reportFailure(ctx context.Context, ticket *model.Ticket, runErr error) {
	logger := logging.From(ctx)
	logger.Error("triage run failed; posting recovery message",
		slog.String("error", runErr.Error()),
	)
	blocks := slackService.BuildFailedBlocks(ctx, ticket.ID, runErr.Error())
	if _, err := e.slack.PostThreadBlocks(ctx, string(ticket.SlackChannelID), string(ticket.SlackThreadTS), blocks); err != nil {
		logger.Error("failed to post triage failure recovery message",
			slog.String("error", err.Error()),
		)
	}
}

// postAsk renders and posts the question form. There is no Slack-side state
// to remember beyond what the message itself encodes (block_id = question
// id; action.value = ticket id) — re-deriving the questions on submit comes
// from the agent history's latest propose_ask tool call.
func (e *PlanExecutor) postAsk(ctx context.Context, ticket *model.Ticket, plan *model.TriagePlan) error {
	if plan.Ask == nil {
		return goerr.New("plan kind ask without payload")
	}
	blocks := buildAskBlocks(ctx, ticket, plan)
	if _, err := e.slack.PostThreadBlocks(ctx, string(ticket.SlackChannelID), string(ticket.SlackThreadTS), blocks); err != nil {
		return goerr.Wrap(err, "post ask form")
	}
	return nil
}
