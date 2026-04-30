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
	slackService "github.com/m-mizutani/shepherd/pkg/service/slack"
	"github.com/m-mizutani/shepherd/pkg/utils/errutil"
	"github.com/m-mizutani/shepherd/pkg/utils/logging"
)

// TicketChangeNotifier delivers a context-block notification about a ticket
// mutation back to the originating Slack thread. Implementations are
// responsible for rendering both status and assignee blocks in a single
// message when both are present in the TicketChange payload.
type TicketChangeNotifier interface {
	NotifyTicketChange(ctx context.Context, channelID, threadTS string, change slackService.TicketChange) error
}

type TicketUseCase struct {
	repo     interfaces.Repository
	registry *model.WorkspaceRegistry
	notifier TicketChangeNotifier
}

func NewTicketUseCase(repo interfaces.Repository, registry *model.WorkspaceRegistry, notifier TicketChangeNotifier) *TicketUseCase {
	return &TicketUseCase{
		repo:     repo,
		registry: registry,
		notifier: notifier,
	}
}

func (uc *TicketUseCase) Create(ctx context.Context, workspaceID types.WorkspaceID, title, description string, statusID types.StatusID, assigneeIDs []types.SlackUserID, fields map[string]model.FieldValue) (*model.Ticket, error) {
	entry, ok := uc.registry.Get(workspaceID)
	if !ok {
		return nil, goerr.New("workspace not found", goerr.V("workspace_id", workspaceID), goerr.Tag(errutil.TagNotFound))
	}

	if statusID == "" {
		statusID = entry.FieldSchema.TicketConfig.DefaultStatusID
	}

	now := time.Now()
	ticket := &model.Ticket{
		ID:          types.TicketID(uuid.Must(uuid.NewV7()).String()),
		WorkspaceID: workspaceID,
		Title:       title,
		Description: description,
		StatusID:    statusID,
		AssigneeIDs: append([]types.SlackUserID(nil), assigneeIDs...),
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

func (uc *TicketUseCase) Get(ctx context.Context, workspaceID types.WorkspaceID, ticketID types.TicketID) (*model.Ticket, error) {
	ticket, err := uc.repo.Ticket().Get(ctx, workspaceID, ticketID)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to get ticket")
	}
	return ticket, nil
}

func (uc *TicketUseCase) List(ctx context.Context, workspaceID types.WorkspaceID, isClosed *bool, statusIDs []types.StatusID) ([]*model.Ticket, error) {
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

func (uc *TicketUseCase) Update(ctx context.Context, workspaceID types.WorkspaceID, ticketID types.TicketID, title, description *string, statusID *types.StatusID, assigneeIDs *[]types.SlackUserID, fields map[string]model.FieldValue) (*model.Ticket, error) {
	existing, err := uc.repo.Ticket().Get(ctx, workspaceID, ticketID)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to get ticket for update")
	}

	oldStatusID := existing.StatusID
	oldAssigneeIDs := append([]types.SlackUserID(nil), existing.AssigneeIDs...)

	if title != nil {
		existing.Title = *title
	}
	if description != nil {
		existing.Description = *description
	}
	if statusID != nil {
		existing.StatusID = *statusID
	}
	if assigneeIDs != nil {
		existing.AssigneeIDs = append([]types.SlackUserID(nil), (*assigneeIDs)...)
	}
	if fields != nil {
		if existing.FieldValues == nil {
			existing.FieldValues = make(map[string]model.FieldValue)
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

	statusChanged := oldStatusID != updated.StatusID
	assigneeChanged := !sameSlackUserIDSet(oldAssigneeIDs, updated.AssigneeIDs)

	if statusChanged {
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
	}

	if statusChanged || assigneeChanged {
		uc.notifyTicketChange(ctx, workspaceID, updated, oldStatusID, oldAssigneeIDs, statusChanged, assigneeChanged)
	}

	return updated, nil
}

func (uc *TicketUseCase) notifyTicketChange(ctx context.Context, workspaceID types.WorkspaceID, ticket *model.Ticket, oldStatusID types.StatusID, oldAssigneeIDs []types.SlackUserID, statusChanged, assigneeChanged bool) {
	if uc.notifier == nil || ticket.SlackChannelID == "" || ticket.SlackThreadTS == "" {
		return
	}

	entry, ok := uc.registry.Get(workspaceID)
	if !ok {
		return
	}

	change := slackService.TicketChange{
		StatusChanged:   statusChanged,
		AssigneeChanged: assigneeChanged,
	}
	if statusChanged {
		change.OldStatusName = statusName(entry, oldStatusID)
		change.NewStatusName = statusName(entry, ticket.StatusID)
	}
	if assigneeChanged {
		change.OldAssigneeIDs = toUserIDStrings(oldAssigneeIDs)
		change.NewAssigneeIDs = toUserIDStrings(ticket.AssigneeIDs)
	}

	logger := logging.From(ctx)
	if err := uc.notifier.NotifyTicketChange(ctx, string(ticket.SlackChannelID), string(ticket.SlackThreadTS), change); err != nil {
		logger.Warn("failed to notify ticket change to slack",
			slog.String("ticket_id", string(ticket.ID)),
			slog.Any("error", err),
		)
	}
}

func toUserIDStrings(ids []types.SlackUserID) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, string(id))
	}
	return out
}

// sameSlackUserIDSet treats assignee lists as unordered sets — order in
// AssigneeIDs is not meaningful, so a reorder alone must not trigger a
// "changed" notification.
func sameSlackUserIDSet(a, b []types.SlackUserID) bool {
	if len(a) != len(b) {
		return false
	}
	seen := make(map[types.SlackUserID]int, len(a))
	for _, id := range a {
		seen[id]++
	}
	for _, id := range b {
		seen[id]--
		if seen[id] < 0 {
			return false
		}
	}
	return true
}

func statusName(entry *model.WorkspaceEntry, statusID types.StatusID) string {
	for _, s := range entry.FieldSchema.Statuses {
		if s.ID == statusID {
			return s.Name
		}
	}
	return string(statusID)
}

func (uc *TicketUseCase) Delete(ctx context.Context, workspaceID types.WorkspaceID, ticketID types.TicketID) error {
	if err := uc.repo.Ticket().Delete(ctx, workspaceID, ticketID); err != nil {
		return goerr.Wrap(err, "failed to delete ticket")
	}
	return nil
}

func (uc *TicketUseCase) ListComments(ctx context.Context, workspaceID types.WorkspaceID, ticketID types.TicketID) ([]*model.Comment, error) {
	comments, err := uc.repo.Comment().List(ctx, workspaceID, ticketID)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to list comments")
	}
	return comments, nil
}

func (uc *TicketUseCase) ListHistory(ctx context.Context, workspaceID types.WorkspaceID, ticketID types.TicketID) ([]*model.TicketHistory, error) {
	histories, err := uc.repo.TicketHistory().List(ctx, workspaceID, ticketID)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to list ticket history")
	}
	return histories, nil
}

func changedByFromContext(ctx context.Context) types.SlackUserID {
	token, err := auth.TokenFromContext(ctx)
	if err != nil || token == nil {
		return "system"
	}
	return types.SlackUserID(token.Sub)
}
