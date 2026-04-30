package ticket

import (
	"context"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/shepherd/pkg/domain/interfaces"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
	argsutil "github.com/m-mizutani/shepherd/pkg/tool/internal/args"
	"github.com/m-mizutani/shepherd/pkg/tool/internal/format"
)

type getHistoryTool struct {
	repo interfaces.Repository
}

func newGetHistoryTool(r interfaces.Repository) gollem.Tool {
	return &getHistoryTool{repo: r}
}

func (t *getHistoryTool) Spec() gollem.ToolSpec {
	return gollem.ToolSpec{
		Name: "ticket_get_history",
		Description: "Return the audit log of a ticket: creation, status transitions, and other lifecycle events. " +
			"Args: `ticket_id` (required, UUID). " +
			"Returns `{ history: [{id, action, old_status_id, new_status_id, changed_by, created_at}], count }`, oldest-first.",
		Parameters: map[string]*gollem.Parameter{
			"ticket_id": {
				Type:        gollem.TypeString,
				Description: "Ticket UUID.",
				Required:    true,
				MinLength:   argsutil.PtrInt(1),
			},
		},
	}
}

func (t *getHistoryTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	wsID, err := workspaceFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	ticketID, err := argsutil.String(args, "ticket_id", true)
	if err != nil {
		return nil, err
	}

	history, err := t.repo.TicketHistory().List(ctx, wsID, types.TicketID(ticketID))
	if err != nil {
		return nil, goerr.Wrap(err, "ticket_get_history failed",
			goerr.V("ticket_id", ticketID))
	}

	out := make([]map[string]any, 0, len(history))
	for _, h := range history {
		out = append(out, format.History(h))
	}
	return map[string]any{
		"history": out,
		"count":   len(out),
	}, nil
}
