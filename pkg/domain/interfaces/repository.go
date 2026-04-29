package interfaces

import (
	"context"

	"github.com/m-mizutani/goerr/v2"
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
	Prompt() PromptRepository
	PutToken(ctx context.Context, token *auth.Token) error
	GetToken(ctx context.Context, tokenID auth.TokenID) (*auth.Token, error)
	DeleteToken(ctx context.Context, tokenID auth.TokenID) error
	Close() error
}

// ErrPromptVersionConflict is returned by PromptRepository.Append when the
// version the caller is trying to write does not equal the current highest
// version + 1 for the (workspace, promptID) pair. Controllers map this to
// HTTP 409.
var ErrPromptVersionConflict = goerr.New("prompt version conflict")

// ErrPromptVersionNotFound is returned when a specific Version is requested
// but does not exist for the (workspace, promptID) pair.
var ErrPromptVersionNotFound = goerr.New("prompt version not found")

// PromptRepository persists workspace-level prompt overrides as an
// append-only sequence of versions.
type PromptRepository interface {
	// Append atomically creates draft.Version for (ws, id), and only succeeds
	// when draft.Version equals the current highest version + 1 (or 1 when
	// no version exists yet). Otherwise it returns ErrPromptVersionConflict.
	//
	// The caller populates Version (the version they intend to write),
	// Content, UpdatedBy*, and UpdatedAt on draft. Concurrent writers that
	// pick the same Version are also rejected with ErrPromptVersionConflict.
	Append(ctx context.Context, ws types.WorkspaceID, id model.PromptID, draft *model.PromptVersion) (*model.PromptVersion, error)

	// GetCurrent returns the highest-Version override for the pair, or
	// (nil, nil) when no override exists yet.
	GetCurrent(ctx context.Context, ws types.WorkspaceID, id model.PromptID) (*model.PromptVersion, error)

	// GetVersion returns a specific version, or ErrPromptVersionNotFound.
	GetVersion(ctx context.Context, ws types.WorkspaceID, id model.PromptID, version int) (*model.PromptVersion, error)

	// List returns all versions for the pair, ordered by Version ascending
	// (oldest = v1 first).
	List(ctx context.Context, ws types.WorkspaceID, id model.PromptID) ([]*model.PromptVersion, error)
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
	// the assignees, and records the supplied history entry. Implementations
	// MUST execute the ticket update and the history append in a single atomic
	// step (Firestore RunTransaction or equivalent). The operation is
	// idempotent: when the ticket is already Triaged, the call is a no-op and
	// no additional history entry is recorded.
	//
	// assignees == nil leaves Ticket.AssigneeIDs untouched. A non-nil pointer
	// (even to an empty slice) replaces the assignee list wholesale.
	// history.ID may be empty; implementations must populate it with a
	// generated identifier.
	FinalizeTriage(ctx context.Context, workspaceID types.WorkspaceID, ticketID types.TicketID, assignees *[]types.SlackUserID, history *model.TicketHistory) error
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
