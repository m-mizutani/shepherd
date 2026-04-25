package ticket

import (
	"context"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/shepherd/pkg/domain/interfaces"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
	"github.com/m-mizutani/shepherd/pkg/tool/internal/clamp"
	"github.com/m-mizutani/shepherd/pkg/tool/internal/format"
)

const (
	commentsDefaultLimit = 50
	commentsMaxLimit     = 200
)

type getCommentsTool struct {
	repo interfaces.Repository
}

func newGetCommentsTool(r interfaces.Repository) gollem.Tool {
	return &getCommentsTool{repo: r}
}

func (t *getCommentsTool) Spec() gollem.ToolSpec {
	return gollem.ToolSpec{
		Name:        "ticket_get_comments",
		Description: "List comments attached to a ticket in the active workspace, oldest-first. Useful for retrieving the conversation history that accompanied a past ticket.",
		Parameters: map[string]*gollem.Parameter{
			"ticket_id": {
				Type:        gollem.TypeString,
				Description: "Ticket UUID.",
				Required:    true,
				MinLength:   ptrInt(1),
			},
			"limit": {
				Type:        gollem.TypeInteger,
				Description: "Maximum comments to return. Defaults to 50, capped at 200.",
			},
		},
	}
}

func (t *getCommentsTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	wsID, err := workspaceFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	ticketID, err := stringArg(args, "ticket_id", true)
	if err != nil {
		return nil, err
	}
	limit := clamp.Limit(intArg(args, "limit"), commentsDefaultLimit, commentsMaxLimit)

	comments, err := t.repo.Comment().List(ctx, wsID, types.TicketID(ticketID))
	if err != nil {
		return nil, goerr.Wrap(err, "ticket_get_comments failed",
			goerr.V("ticket_id", ticketID))
	}
	if len(comments) > limit {
		comments = comments[:limit]
	}

	out := make([]map[string]any, 0, len(comments))
	for _, c := range comments {
		out = append(out, format.Comment(c, string(c.SlackUserID)))
	}
	return map[string]any{
		"comments": out,
		"count":    len(out),
	}, nil
}
