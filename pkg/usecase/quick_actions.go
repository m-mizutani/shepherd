package usecase

import (
	"context"
	"log/slog"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/shepherd/pkg/domain/interfaces"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
	"github.com/m-mizutani/shepherd/pkg/utils/logging"
)

// QuickActionsUseCase orchestrates the inline assignee / status changes a
// user makes through the Slack quick-actions menu. It resolves the
// underlying ticket from channel + thread_ts (the menu is always posted
// as a thread reply, so block_actions carry the ticket's thread_ts in
// their cb.Message.ThreadTimestamp), then funnels the mutation through
// the same TicketUseCase.Update entry point used by the HTTP API. That
// guarantees the change-notification path fires once per logical update,
// regardless of which surface drove it.
type QuickActionsUseCase struct {
	repo     interfaces.Repository
	registry *model.WorkspaceRegistry
	ticketUC *TicketUseCase
}

func NewQuickActionsUseCase(repo interfaces.Repository, registry *model.WorkspaceRegistry, ticketUC *TicketUseCase) *QuickActionsUseCase {
	return &QuickActionsUseCase{
		repo:     repo,
		registry: registry,
		ticketUC: ticketUC,
	}
}

// HandleAssigneeChange applies the user's multi_users_select choice to
// the ticket identified by (channelID, threadTS). userIDs may be empty
// when the user cleared the field. Unmapped channels / unknown threads
// are debug-logged no-ops so re-deliveries from Slack do not error.
func (uc *QuickActionsUseCase) HandleAssigneeChange(ctx context.Context, channelID, threadTS string, userIDs []string) error {
	ticket, wsID, ok, err := uc.resolveTicket(ctx, channelID, threadTS)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}

	assignees := make([]types.SlackUserID, 0, len(userIDs))
	for _, id := range userIDs {
		if id == "" {
			continue
		}
		assignees = append(assignees, types.SlackUserID(id))
	}

	if _, err := uc.ticketUC.Update(ctx, wsID, ticket.ID, nil, nil, nil, &assignees, nil, nil); err != nil {
		return goerr.Wrap(err, "failed to update assignees from quick actions",
			goerr.V("ticket_id", string(ticket.ID)),
		)
	}
	return nil
}

// HandleStatusChange applies the user's static_select choice to the
// ticket identified by (channelID, threadTS). statusID == "" is treated
// as "no change" (defensive — Slack usually never delivers an empty
// option from a non-optional select). Unmapped channels / unknown
// threads are debug-logged no-ops.
func (uc *QuickActionsUseCase) HandleStatusChange(ctx context.Context, channelID, threadTS, statusID string) error {
	if statusID == "" {
		return nil
	}
	ticket, wsID, ok, err := uc.resolveTicket(ctx, channelID, threadTS)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}

	sid := types.StatusID(statusID)
	if _, err := uc.ticketUC.Update(ctx, wsID, ticket.ID, nil, nil, &sid, nil, nil, nil); err != nil {
		return goerr.Wrap(err, "failed to update status from quick actions",
			goerr.V("ticket_id", string(ticket.ID)),
		)
	}
	return nil
}

// resolveTicket maps channelID + threadTS to the underlying ticket via
// the workspace registry. When the channel is unmapped or no ticket
// exists for the thread, ok==false and the caller should treat the call
// as a no-op (re-delivered or stale block_actions).
func (uc *QuickActionsUseCase) resolveTicket(ctx context.Context, channelID, threadTS string) (*model.Ticket, types.WorkspaceID, bool, error) {
	logger := logging.From(ctx)

	entry, ok := uc.registry.GetBySlackChannelID(channelID)
	if !ok {
		logger.Debug("quick action ignored: channel not mapped",
			slog.String("channel_id", channelID),
		)
		return nil, "", false, nil
	}
	wsID := entry.Workspace.ID

	ticket, err := uc.repo.Ticket().GetBySlackThreadTS(ctx, wsID, types.SlackChannelID(channelID), types.SlackThreadTS(threadTS))
	if err != nil {
		return nil, wsID, false, goerr.Wrap(err, "failed to find ticket for quick action",
			goerr.V("channel_id", channelID),
			goerr.V("thread_ts", threadTS),
		)
	}
	if ticket == nil {
		logger.Debug("quick action ignored: no ticket for thread",
			slog.String("channel_id", channelID),
			slog.String("thread_ts", threadTS),
		)
		return nil, wsID, false, nil
	}
	return ticket, wsID, true, nil
}
