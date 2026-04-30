package ticket

import (
	"context"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/shepherd/pkg/domain/interfaces"
	argsutil "github.com/m-mizutani/shepherd/pkg/tool/internal/args"
	"github.com/m-mizutani/shepherd/pkg/tool/internal/format"
)

type getTool struct {
	repo interfaces.Repository
}

func newGetTool(r interfaces.Repository) gollem.Tool {
	return &getTool{repo: r}
}

func (t *getTool) Spec() gollem.ToolSpec {
	return gollem.ToolSpec{
		Name: "ticket_get",
		Description: "Fetch a single ticket from the active workspace. " +
			"Args: provide exactly one of `ticket_id` (UUID) or `seq_num` (workspace-scoped sequence number, the human-friendly ID surfaced in Slack). " +
			"Returns `{ found: bool }` when not found; otherwise `{ found: true, ticket: {id, seq_num, title, description, status_id, status_name, assignees, reporter, slack_channel_id, slack_thread_ts, fields, created_at, updated_at} }`.",
		Parameters: map[string]*gollem.Parameter{
			"ticket_id": {
				Type:        gollem.TypeString,
				Description: "Ticket UUID (the 'id' field).",
			},
			"seq_num": {
				Type:        gollem.TypeInteger,
				Description: "Workspace-scoped sequence number (the human-friendly ID surfaced in Slack).",
			},
		},
	}
}

func (t *getTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	wsID, err := workspaceFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	ticketID, _ := argsutil.String(args, "ticket_id", false)
	seqNum, hasSeq := argsutil.Int64(args, "seq_num")

	if ticketID == "" && !hasSeq {
		return nil, goerr.New("provide either ticket_id or seq_num")
	}
	if ticketID != "" && hasSeq {
		return nil, goerr.New("provide only one of ticket_id or seq_num")
	}

	tickets, err := t.repo.Ticket().List(ctx, wsID, nil)
	if err != nil {
		return nil, goerr.Wrap(err, "ticket_get list failed")
	}
	for _, tk := range tickets {
		if ticketID != "" && string(tk.ID) == ticketID {
			return map[string]any{"found": true, "ticket": format.Ticket(tk, "")}, nil
		}
		if hasSeq && tk.SeqNum == seqNum {
			return map[string]any{"found": true, "ticket": format.Ticket(tk, "")}, nil
		}
	}
	return map[string]any{"found": false}, nil
}
