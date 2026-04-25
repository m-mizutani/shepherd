package interfaces

import (
	"context"

	"github.com/m-mizutani/shepherd/pkg/domain/model"
	"github.com/m-mizutani/shepherd/pkg/domain/model/auth"
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
	Create(ctx context.Context, workspaceID string, t *model.Ticket) (*model.Ticket, error)
	Get(ctx context.Context, workspaceID string, id string) (*model.Ticket, error)
	List(ctx context.Context, workspaceID string, statusIDs []string) ([]*model.Ticket, error)
	Update(ctx context.Context, workspaceID string, t *model.Ticket) (*model.Ticket, error)
	Delete(ctx context.Context, workspaceID string, id string) error
	GetBySlackThreadTS(ctx context.Context, workspaceID string, channelID, threadTS string) (*model.Ticket, error)
}

type CommentRepository interface {
	Create(ctx context.Context, workspaceID string, ticketID string, c *model.Comment) (*model.Comment, error)
	List(ctx context.Context, workspaceID string, ticketID string) ([]*model.Comment, error)
	GetBySlackTS(ctx context.Context, workspaceID string, ticketID string, slackTS string) (*model.Comment, error)
}

type TicketHistoryRepository interface {
	Create(ctx context.Context, workspaceID string, ticketID string, h *model.TicketHistory) (*model.TicketHistory, error)
	List(ctx context.Context, workspaceID string, ticketID string) ([]*model.TicketHistory, error)
}

