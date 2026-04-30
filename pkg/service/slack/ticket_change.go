package slack

import (
	"context"
	"strings"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/shepherd/pkg/utils/errutil"
	"github.com/m-mizutani/shepherd/pkg/utils/i18n"
	slackgo "github.com/slack-go/slack"
)

// TicketChange describes a mutation that should be reflected back to the
// originating Slack thread as a single context-block notification. Either
// the status block, the assignee block, or both, may be set on a given
// payload — when both are set, the notifier renders them in one message
// so the thread reader sees one event per logical update rather than two.
type TicketChange struct {
	StatusChanged   bool
	OldStatusName   string
	NewStatusName   string
	AssigneeChanged bool
	OldAssigneeIDs  []string
	NewAssigneeIDs  []string
}

// NotifyTicketChange posts a context-block message summarising a status
// and/or assignee transition. It is a no-op when neither block is set,
// since callers gate on this themselves but the safety net keeps the
// helper composable.
func (c *Client) NotifyTicketChange(ctx context.Context, channelID, threadTS string, change TicketChange) error {
	if !change.StatusChanged && !change.AssigneeChanged {
		return nil
	}
	loc := i18n.From(ctx)

	lines := make([]string, 0, 2)
	if change.StatusChanged {
		lines = append(lines, loc.T(i18n.MsgStatusChange,
			"old", change.OldStatusName,
			"new", change.NewStatusName,
		))
	}
	if change.AssigneeChanged {
		lines = append(lines, loc.T(i18n.MsgAssigneeChange,
			"old", renderAssignees(ctx, change.OldAssigneeIDs),
			"new", renderAssignees(ctx, change.NewAssigneeIDs),
		))
	}
	text := strings.Join(lines, "\n")

	_, _, err := c.api.PostMessageContext(ctx, channelID,
		slackgo.MsgOptionTS(threadTS),
		slackgo.MsgOptionBlocks(
			slackgo.NewContextBlock("",
				slackgo.NewTextBlockObject("mrkdwn", text, false, false),
			),
		),
	)
	if err != nil {
		return goerr.Wrap(err, "failed to post ticket change notification",
			goerr.V("channel_id", channelID),
			goerr.V("thread_ts", threadTS),
			goerr.Tag(errutil.TagSlackError),
		)
	}
	return nil
}

// renderAssignees produces the mrkdwn fragment that represents an
// assignee list for the change notification. An empty list collapses to
// the localised "(unassigned)" marker so the transition stays readable
// even when one side is empty.
func renderAssignees(ctx context.Context, ids []string) string {
	if len(ids) == 0 {
		return i18n.From(ctx).T(i18n.MsgUnassigned)
	}
	mentions := make([]string, 0, len(ids))
	for _, id := range ids {
		if id == "" {
			continue
		}
		mentions = append(mentions, "<@"+id+">")
	}
	if len(mentions) == 0 {
		return i18n.From(ctx).T(i18n.MsgUnassigned)
	}
	return strings.Join(mentions, " ")
}
