package interfaces

import (
	"context"

	"github.com/m-mizutani/shepherd/pkg/domain/model"
	"github.com/m-mizutani/shepherd/pkg/domain/model/auth"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
)

type Repository interface {
	Ticket() TicketRepository
	Comment() CommentRepository
	TicketHistory() TicketHistoryRepository
	PutToken(ctx context.Context, token *auth.Token) error
	GetToken(ctx context.Context, tokenID auth.TokenID) (*auth.Token, error)
	DeleteToken(ctx context.Context, tokenID auth.TokenID) error
	Close() error
}

type TicketRepository interface {
	Create(ctx context.Context, workspaceID types.WorkspaceID, t *model.Ticket) (*model.Ticket, error)
	Get(ctx context.Context, workspaceID types.WorkspaceID, id types.TicketID) (*model.Ticket, error)
	List(ctx context.Context, workspaceID types.WorkspaceID, statusIDs []types.StatusID) ([]*model.Ticket, error)
	Update(ctx context.Context, workspaceID types.WorkspaceID, t *model.Ticket) (*model.Ticket, error)
	Delete(ctx context.Context, workspaceID types.WorkspaceID, id types.TicketID) error
	GetBySlackThreadTS(ctx context.Context, workspaceID types.WorkspaceID, channelID types.SlackChannelID, threadTS types.SlackThreadTS) (*model.Ticket, error)
}

type CommentRepository interface {
	Create(ctx context.Context, workspaceID types.WorkspaceID, ticketID types.TicketID, c *model.Comment) (*model.Comment, error)
	List(ctx context.Context, workspaceID types.WorkspaceID, ticketID types.TicketID) ([]*model.Comment, error)
	GetBySlackTS(ctx context.Context, workspaceID types.WorkspaceID, ticketID types.TicketID, slackTS types.SlackThreadTS) (*model.Comment, error)
}

type TicketHistoryRepository interface {
	Create(ctx context.Context, workspaceID types.WorkspaceID, ticketID types.TicketID, h *model.TicketHistory) (*model.TicketHistory, error)
	List(ctx context.Context, workspaceID types.WorkspaceID, ticketID types.TicketID) ([]*model.TicketHistory, error)
}
