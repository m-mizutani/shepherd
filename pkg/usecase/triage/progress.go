package triage

import (
	"context"
	"sync"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
	slackService "github.com/m-mizutani/shepherd/pkg/service/slack"
	"github.com/m-mizutani/shepherd/pkg/utils/errutil"
	slackgo "github.com/slack-go/slack"
)

// progressMessage owns a Slack message that displays per-subtask progress
// rows. Concurrent subtasks call UpdateTrace / MarkDone / MarkFailed; the
// struct serialises updates through its mutex and re-renders the entire
// blocks payload via chat.update.
type progressMessage struct {
	slack         progressSlack
	channel       types.SlackChannelID
	threadTS      types.SlackThreadTS
	headerMessage string

	mu     sync.Mutex
	tsKnown bool
	ts      string
	rows    []slackService.SubtaskProgress
}

// progressSlack is the minimal Slack surface progressMessage depends on.
// Defined as an interface so tests can substitute a fake.
type progressSlack interface {
	PostThreadBlocks(ctx context.Context, channelID, threadTS string, blocks []slackgo.Block) (string, error)
	UpdateMessage(ctx context.Context, channelID, messageTS string, blocks []slackgo.Block) error
}

// newProgressMessage posts the initial progress message in the ticket thread
// with every subtask shown as queued, and returns a handle that can be used
// to mutate individual rows as work proceeds.
func newProgressMessage(ctx context.Context, slack progressSlack, channel types.SlackChannelID, threadTS types.SlackThreadTS, headerMessage string, subtasks []model.Subtask) (*progressMessage, error) {
	rows := make([]slackService.SubtaskProgress, 0, len(subtasks))
	for _, st := range subtasks {
		rows = append(rows, slackService.SubtaskProgress{
			ID:      st.ID,
			Request: st.Request,
			State:   slackService.SubtaskQueued,
		})
	}

	blocks := slackService.BuildProgressBlocks(ctx, headerMessage, rows)
	ts, err := slack.PostThreadBlocks(ctx, string(channel), string(threadTS), blocks)
	if err != nil {
		return nil, goerr.Wrap(err, "post initial progress message")
	}
	return &progressMessage{
		slack:         slack,
		channel:       channel,
		threadTS:      threadTS,
		headerMessage: headerMessage,
		tsKnown:       true,
		ts:            ts,
		rows:          rows,
	}, nil
}

// MessageTS returns the timestamp of the underlying Slack message. Useful in
// tests to assert the message exists.
func (p *progressMessage) MessageTS() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.ts
}

// UpdateTrace transitions the subtask to the running state and records the
// supplied trace text. Repeated calls overwrite the trace.
func (p *progressMessage) UpdateTrace(ctx context.Context, subtaskID types.SubtaskID, trace string) {
	p.mutate(ctx, func(row *slackService.SubtaskProgress) {
		row.State = slackService.SubtaskRunning
		row.Trace = trace
		row.Error = ""
	}, subtaskID)
}

// MarkDone transitions the subtask to the done state. The summary is not
// shown verbatim in the row (the row only carries a state icon + request),
// but the call signature carries it for symmetry with MarkFailed.
func (p *progressMessage) MarkDone(ctx context.Context, subtaskID types.SubtaskID, _ string) {
	p.mutate(ctx, func(row *slackService.SubtaskProgress) {
		row.State = slackService.SubtaskDone
		row.Trace = ""
		row.Error = ""
	}, subtaskID)
}

// MarkFailed transitions the subtask to the failed state, recording a short
// error summary that is shown next to the request.
func (p *progressMessage) MarkFailed(ctx context.Context, subtaskID types.SubtaskID, errSummary string) {
	p.mutate(ctx, func(row *slackService.SubtaskProgress) {
		row.State = slackService.SubtaskFailed
		row.Trace = ""
		row.Error = errSummary
	}, subtaskID)
}

func (p *progressMessage) mutate(ctx context.Context, fn func(*slackService.SubtaskProgress), subtaskID types.SubtaskID) {
	// Snapshot the rendered blocks under lock, then release before the
	// network call. Holding the mutex across UpdateMessage would serialise
	// every parallel subtask's progress update on Slack's API latency,
	// negating the whole point of running them in parallel.
	p.mu.Lock()
	for i := range p.rows {
		if p.rows[i].ID == subtaskID {
			fn(&p.rows[i])
			break
		}
	}
	if !p.tsKnown {
		p.mu.Unlock()
		return
	}
	blocks := slackService.BuildProgressBlocks(ctx, p.headerMessage, p.rows)
	channel := string(p.channel)
	ts := p.ts
	p.mu.Unlock()

	if err := p.slack.UpdateMessage(ctx, channel, ts, blocks); err != nil {
		// Slack update failures are non-fatal: the subtask still ran, and
		// the next mutate() call will repaint with the latest state. Route
		// the error through errutil so it surfaces in logs / Sentry.
		errutil.Handle(ctx, goerr.Wrap(err, "update progress message"))
	}
}
