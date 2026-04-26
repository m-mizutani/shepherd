package triage

import (
	"context"
	"strings"
	"sync"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
	"github.com/m-mizutani/shepherd/pkg/tool"
	"github.com/m-mizutani/shepherd/pkg/usecase/prompt"
	"github.com/m-mizutani/shepherd/pkg/utils/async"
	"github.com/m-mizutani/shepherd/pkg/utils/errutil"
	"github.com/m-mizutani/shepherd/pkg/utils/msg"
)

// runInvestigate executes the planner's Investigate decision: spin up a
// child gollem agent per subtask in parallel, surface progress through the
// per-subtask Slack context blocks, and feed the aggregated summaries back
// to the planner via the plan-level history.
//
// Each subtask gets its own gollem session ("{ws}/{ticket}/sub/{subtaskID}")
// so its tool-call traces stay isolated from the planner agent.
func (e *PlanExecutor) runInvestigate(ctx context.Context, ticket *model.Ticket, plan *model.TriagePlan, progress *progressMessage) error {
	inv := plan.Investigate
	if inv == nil {
		return goerr.New("runInvestigate called without Investigate payload")
	}

	allTools, err := e.catalog.For(ctx, ticket.WorkspaceID)
	if err != nil {
		return goerr.Wrap(err, "load workspace tool catalog",
			goerr.V("workspace", ticket.WorkspaceID))
	}

	var (
		mu        sync.Mutex
		summaries = make(map[types.SubtaskID]string, len(inv.Subtasks))
		failures  = make(map[types.SubtaskID]string, len(inv.Subtasks))
	)

	fns := make([]func(context.Context) error, 0, len(inv.Subtasks))
	for i := range inv.Subtasks {
		st := inv.Subtasks[i]
		fns = append(fns, func(ctx context.Context) error {
			return e.runSubtask(ctx, ticket, st, allTools, progress, &mu, summaries, failures)
		})
	}

	if err := async.RunParallel(ctx, fns...); err != nil {
		// async.RunParallel returns a joined error covering panics and any
		// per-subtask errors not swallowed by runSubtask. We still push the
		// partial results to the planner — losing one subtask should not
		// kill the iteration — but the error is surfaced via errutil so it
		// reaches logs and Sentry.
		errutil.Handle(ctx, goerr.Wrap(err, "investigate subtasks"))
	}

	// Feed aggregated results back as a user message so the planner sees them.
	contextMsg := formatInvestigationContext(inv.Subtasks, summaries, failures)
	if err := appendUserMessage(ctx, e.historyRepo, ticket.WorkspaceID, ticket.ID, contextMsg); err != nil {
		return goerr.Wrap(err, "append investigate result to plan history")
	}
	return nil
}

func (e *PlanExecutor) runSubtask(ctx context.Context, ticket *model.Ticket, st model.Subtask,
	allTools []gollem.Tool, progress *progressMessage,
	mu *sync.Mutex, summaries, failures map[types.SubtaskID]string) error {

	systemPrompt, err := prompt.RenderTriageSubtask(prompt.TriageSubtaskInput{
		Request:            st.Request,
		AcceptanceCriteria: st.AcceptanceCriteria,
	})
	if err != nil {
		return goerr.Wrap(err, "render triage_subtask prompt", goerr.V("subtask", st.ID))
	}

	// Wire the per-subtask trace callback so child tools can update the
	// matching Slack context block. We deliberately keep notify/warn nil for
	// MVP — only Trace is consumed by the progress UI.
	traceFn := func(ctx context.Context, m string) {
		if progress != nil {
			progress.UpdateTrace(ctx, st.ID, m)
		}
	}
	stCtx := msg.With(ctx, nil, traceFn, nil)

	if progress != nil {
		progress.UpdateTrace(stCtx, st.ID, "starting")
	}

	subTools := tool.FilterByName(allTools, st.AllowedTools)
	child := gollem.New(e.llm,
		gollem.WithSystemPrompt(systemPrompt),
		gollem.WithTools(subTools...),
		gollem.WithHistoryRepository(e.historyRepo, subtaskSessionID(ticket.WorkspaceID, ticket.ID, st.ID)),
	)
	resp, runErr := child.Execute(stCtx, gollem.Text(st.Request))

	mu.Lock()
	defer mu.Unlock()
	if runErr != nil {
		failures[st.ID] = runErr.Error()
		if progress != nil {
			progress.MarkFailed(ctx, st.ID, runErr.Error())
		}
		// Per-subtask failures are recorded into the summary; do not bubble
		// up so a single failed subtask does not abort the iteration.
		return nil
	}

	summary := ""
	if resp != nil {
		summary = strings.TrimSpace(strings.Join(resp.Texts, "\n"))
	}
	summaries[st.ID] = summary
	if progress != nil {
		progress.MarkDone(ctx, st.ID, summary)
	}
	return nil
}

func formatInvestigationContext(subtasks []model.Subtask, summaries, failures map[types.SubtaskID]string) string {
	var b strings.Builder
	b.WriteString("Investigate result:\n")
	for _, st := range subtasks {
		b.WriteString("\n- Subtask ")
		b.WriteString(string(st.ID))
		b.WriteString(" (")
		b.WriteString(st.Request)
		b.WriteString("):")
		if msg, ok := failures[st.ID]; ok {
			b.WriteString(" FAILED — ")
			b.WriteString(msg)
			continue
		}
		summary, ok := summaries[st.ID]
		if !ok || strings.TrimSpace(summary) == "" {
			b.WriteString(" (no summary returned)")
			continue
		}
		b.WriteString("\n")
		b.WriteString(indent(summary, "  "))
	}
	return b.String()
}

func indent(s, prefix string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
}
