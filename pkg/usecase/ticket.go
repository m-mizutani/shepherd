package usecase

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/shepherd/pkg/domain/interfaces"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
	"github.com/m-mizutani/shepherd/pkg/domain/model/auth"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
	"github.com/m-mizutani/shepherd/pkg/utils/errutil"
	"github.com/m-mizutani/shepherd/pkg/utils/logging"
)

type StatusChangeNotifier interface {
	ReplyStatusChange(ctx context.Context, channelID, threadTS, oldStatusName, newStatusName string) error
}

type TicketUseCase struct {
	repo     interfaces.Repository
	registry *model.WorkspaceRegistry
	notifier StatusChangeNotifier
}

func NewTicketUseCase(repo interfaces.Repository, registry *model.WorkspaceRegistry, notifier StatusChangeNotifier) *TicketUseCase {
	return &TicketUseCase{
		repo:     repo,
		registry: registry,
		notifier: notifier,
	}
}

func (uc *TicketUseCase) Create(ctx context.Context, workspaceID string, title, description, statusID, assigneeID string, fields map[types.FieldID]model.FieldValue) (*model.Ticket, error) {
	entry, ok := uc.registry.Get(workspaceID)
	if !ok {
		return nil, goerr.New("workspace not found", goerr.V("workspace_id", workspaceID), goerr.Tag(errutil.TagNotFound))
	}

	if statusID == "" {
		statusID = entry.FieldSchema.TicketConfig.DefaultStatusID
	}

	now := time.Now()
	ticket := &model.Ticket{
		ID:          uuid.Must(uuid.NewV7()).String(),
		WorkspaceID: workspaceID,
		Title:       title,
		Description: description,
		StatusID:    statusID,
		AssigneeID:  assigneeID,
		FieldValues: fields,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	created, err := uc.repo.Ticket().Create(ctx, workspaceID, ticket)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to create ticket")
	}

	changedBy := changedByFromContext(ctx)
	history := &model.TicketHistory{
		ID:          uuid.Must(uuid.NewV7()).String(),
		NewStatusID: statusID,
		ChangedBy:   changedBy,
		Action:      "created",
		CreatedAt:   now,
	}
	if _, err := uc.repo.TicketHistory().Create(ctx, workspaceID, created.ID, history); err != nil {
		errutil.Handle(ctx, goerr.Wrap(err, "failed to create ticket history"))
	}

	return created, nil
}

func (uc *TicketUseCase) Get(ctx context.Context, workspaceID, ticketID string) (*model.Ticket, error) {
	ticket, err := uc.repo.Ticket().Get(ctx, workspaceID, ticketID)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to get ticket")
	}
	return ticket, nil
}

func (uc *TicketUseCase) List(ctx context.Context, workspaceID string, isClosed *bool, statusIDs []string) ([]*model.Ticket, error) {
	tickets, err := uc.repo.Ticket().List(ctx, workspaceID, statusIDs)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to list tickets")
	}

	if isClosed != nil {
		entry, ok := uc.registry.Get(workspaceID)
		if ok {
			filtered := tickets[:0]
			for _, t := range tickets {
				if entry.FieldSchema.IsClosedStatus(t.StatusID) == *isClosed {
					filtered = append(filtered, t)
				}
			}
			tickets = filtered
		}
	}

	return tickets, nil
}

func (uc *TicketUseCase) Update(ctx context.Context, workspaceID, ticketID string, title, description, statusID, assigneeID *string, fields map[types.FieldID]model.FieldValue) (*model.Ticket, error) {
	existing, err := uc.repo.Ticket().Get(ctx, workspaceID, ticketID)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to get ticket for update")
	}

	oldStatusID := existing.StatusID

	if title != nil {
		existing.Title = *title
	}
	if description != nil {
		existing.Description = *description
	}
	if statusID != nil {
		existing.StatusID = *statusID
	}
	if assigneeID != nil {
		existing.AssigneeID = *assigneeID
	}
	if fields != nil {
		if existing.FieldValues == nil {
			existing.FieldValues = make(map[types.FieldID]model.FieldValue)
		}
		for k, v := range fields {
			existing.FieldValues[k] = v
		}
	}
	existing.UpdatedAt = time.Now()

	updated, err := uc.repo.Ticket().Update(ctx, workspaceID, existing)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to update ticket")
	}

	if oldStatusID != updated.StatusID {
		changedBy := changedByFromContext(ctx)
		history := &model.TicketHistory{
			ID:          uuid.Must(uuid.NewV7()).String(),
			NewStatusID: updated.StatusID,
			OldStatusID: oldStatusID,
			ChangedBy:   changedBy,
			Action:      "changed",
			CreatedAt:   time.Now(),
		}
		if _, err := uc.repo.TicketHistory().Create(ctx, workspaceID, ticketID, history); err != nil {
			errutil.Handle(ctx, goerr.Wrap(err, "failed to create ticket history"))
		}

		uc.notifyStatusChange(ctx, workspaceID, updated, oldStatusID)
	}

	return updated, nil
}

func (uc *TicketUseCase) notifyStatusChange(ctx context.Context, workspaceID string, ticket *model.Ticket, oldStatusID string) {
	if uc.notifier == nil || ticket.SlackChannelID == "" || ticket.SlackThreadTS == "" {
		return
	}

	entry, ok := uc.registry.Get(workspaceID)
	if !ok {
		return
	}

	oldName := statusName(entry, oldStatusID)
	newName := statusName(entry, ticket.StatusID)

	logger := logging.From(ctx)
	if err := uc.notifier.ReplyStatusChange(ctx, ticket.SlackChannelID, ticket.SlackThreadTS, oldName, newName); err != nil {
		logger.Warn("failed to notify status change to slack",
			slog.String("ticket_id", ticket.ID),
			slog.Any("error", err),
		)
	}
}

func statusName(entry *model.WorkspaceEntry, statusID string) string {
	for _, s := range entry.FieldSchema.Statuses {
		if s.ID == statusID {
			return s.Name
		}
	}
	return statusID
}

func (uc *TicketUseCase) Delete(ctx context.Context, workspaceID, ticketID string) error {
	if err := uc.repo.Ticket().Delete(ctx, workspaceID, ticketID); err != nil {
		return goerr.Wrap(err, "failed to delete ticket")
	}
	return nil
}

func (uc *TicketUseCase) ListComments(ctx context.Context, workspaceID, ticketID string) ([]*model.Comment, error) {
	comments, err := uc.repo.Comment().List(ctx, workspaceID, ticketID)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to list comments")
	}
	return comments, nil
}

func (uc *TicketUseCase) ListHistory(ctx context.Context, workspaceID, ticketID string) ([]*model.TicketHistory, error) {
	histories, err := uc.repo.TicketHistory().List(ctx, workspaceID, ticketID)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to list ticket history")
	}
	return histories, nil
}

func changedByFromContext(ctx context.Context) string {
	token, err := auth.TokenFromContext(ctx)
	if err != nil || token == nil {
		return "system"
	}
	return token.Sub
}
