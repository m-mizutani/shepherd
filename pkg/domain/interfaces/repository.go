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
	Source() SourceRepository
	ToolSettings() ToolSettingsRepository
	PutToken(ctx context.Context, token *auth.Token) error
	GetToken(ctx context.Context, tokenID auth.TokenID) (*auth.Token, error)
	DeleteToken(ctx context.Context, tokenID auth.TokenID) error
	Close() error
}

type SourceRepository interface {
	Create(ctx context.Context, s *model.Source) (*model.Source, error)
	Get(ctx context.Context, ws types.WorkspaceID, id types.SourceID) (*model.Source, error)
	List(ctx context.Context, ws types.WorkspaceID) ([]*model.Source, error)
	ListByProvider(ctx context.Context, ws types.WorkspaceID, p types.SourceProvider) ([]*model.Source, error)
	UpdateDescription(ctx context.Context, ws types.WorkspaceID, id types.SourceID, description string) (*model.Source, error)
	Delete(ctx context.Context, ws types.WorkspaceID, id types.SourceID) error
}

type ToolSettingsRepository interface {
	// Get returns the workspace's recorded settings, or an empty (non-nil)
	// ToolSettings when nothing has been written yet. Callers apply factory
	// defaults to absent keys via ToolSettings.IsEnabled.
	Get(ctx context.Context, ws types.WorkspaceID) (*model.ToolSettings, error)
	Set(ctx context.Context, ws types.WorkspaceID, providerID string, enabled bool) error
}

type TicketRepository interface {
	Create(ctx context.Context, workspaceID types.WorkspaceID, t *model.Ticket) (*model.Ticket, error)
	Get(ctx context.Context, workspaceID types.WorkspaceID, id types.TicketID) (*model.Ticket, error)
	List(ctx context.Context, workspaceID types.WorkspaceID, statusIDs []types.StatusID) ([]*model.Ticket, error)
	Update(ctx context.Context, workspaceID types.WorkspaceID, t *model.Ticket) (*model.Ticket, error)
	Delete(ctx context.Context, workspaceID types.WorkspaceID, id types.TicketID) error
	GetBySlackThreadTS(ctx context.Context, workspaceID types.WorkspaceID, channelID types.SlackChannelID, threadTS types.SlackThreadTS) (*model.Ticket, error)

	// FinalizeTriage atomically marks the ticket as triaged, optionally updates
	// the assignee, and records the supplied history entry. Implementations
	// MUST execute the ticket update and the history append in a single atomic
	// step (Firestore RunTransaction or equivalent). The operation is
	// idempotent: when the ticket is already Triaged, the call is a no-op and
	// no additional history entry is recorded.
	//
	// assignee == nil leaves Ticket.AssigneeID untouched. history.ID may be
	// empty; implementations must populate it with a generated identifier.
	FinalizeTriage(ctx context.Context, workspaceID types.WorkspaceID, ticketID types.TicketID, assignee *types.SlackUserID, history *model.TicketHistory) error
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
