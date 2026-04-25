package usecase

import (
	"context"
	"log/slog"
	"net/url"
	"time"

	"github.com/google/uuid"
	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/shepherd/pkg/domain/interfaces"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
	slackService "github.com/m-mizutani/shepherd/pkg/service/slack"
	"github.com/m-mizutani/shepherd/pkg/utils/logging"
)

type SlackUseCase struct {
	repo     interfaces.Repository
	registry *model.WorkspaceRegistry
	slack    *slackService.Client
	baseURL  string
}

func NewSlackUseCase(repo interfaces.Repository, registry *model.WorkspaceRegistry, slack *slackService.Client, baseURL string) *SlackUseCase {
	return &SlackUseCase{
		repo:     repo,
		registry: registry,
		slack:    slack,
		baseURL:  baseURL,
	}
}

func (uc *SlackUseCase) HandleNewMessage(ctx context.Context, channelID, userID, text, messageTS string) error {
	logger := logging.From(ctx)

	entry, ok := uc.registry.GetBySlackChannelID(channelID)
	if !ok {
		logger.Debug("slack message ignored: channel not mapped to workspace",
			slog.String("channel_id", channelID),
		)
		return nil
	}

	wsID := entry.Workspace.ID
	logger.Debug("handling new slack message",
		slog.String("workspace_id", wsID),
		slog.String("channel_id", channelID),
		slog.String("user_id", userID),
		slog.String("message_ts", messageTS),
	)

	existing, err := uc.repo.Ticket().GetBySlackThreadTS(ctx, wsID, channelID, messageTS)
	if err != nil {
		return goerr.Wrap(err, "failed to check duplicate ticket")
	}
	if existing != nil {
		logger.Debug("slack message ignored: ticket already exists",
			slog.String("ticket_id", existing.ID),
			slog.String("message_ts", messageTS),
		)
		return nil
	}

	now := time.Now()
	ticket := &model.Ticket{
		ID:                  uuid.Must(uuid.NewV7()).String(),
		WorkspaceID:         wsID,
		Title:               truncate(text, 200),
		Description:         text,
		InitialMessage:      text,
		StatusID:            entry.FieldSchema.TicketConfig.DefaultStatusID,
		ReporterSlackUserID: userID,
		SlackChannelID:      channelID,
		SlackThreadTS:       messageTS,
		FieldValues:         make(map[types.FieldID]model.FieldValue),
		CreatedAt:           now,
		UpdatedAt:           now,
	}

	created, err := uc.repo.Ticket().Create(ctx, wsID, ticket)
	if err != nil {
		return goerr.Wrap(err, "failed to create ticket from slack message")
	}

	logger.Info("ticket created from slack message",
		slog.String("workspace_id", wsID),
		slog.String("ticket_id", created.ID),
		slog.Int64("seq_num", created.SeqNum),
		slog.String("channel_id", channelID),
	)

	ticketURL, _ := url.JoinPath(uc.baseURL, "ws", wsID, "tickets", created.ID)
	if err := uc.slack.ReplyTicketCreated(ctx, channelID, messageTS, created.SeqNum, ticketURL); err != nil {
		return goerr.Wrap(err, "failed to reply ticket created")
	}

	return nil
}

func (uc *SlackUseCase) HandleThreadReply(ctx context.Context, channelID, threadTS, userID, text, messageTS string, isBot bool) error {
	logger := logging.From(ctx)

	entry, ok := uc.registry.GetBySlackChannelID(channelID)
	if !ok {
		logger.Debug("slack thread reply ignored: channel not mapped to workspace",
			slog.String("channel_id", channelID),
		)
		return nil
	}

	wsID := entry.Workspace.ID

	ticket, err := uc.repo.Ticket().GetBySlackThreadTS(ctx, wsID, channelID, threadTS)
	if err != nil {
		return goerr.Wrap(err, "failed to find ticket by thread_ts")
	}
	if ticket == nil {
		logger.Debug("slack thread reply ignored: no ticket for thread",
			slog.String("channel_id", channelID),
			slog.String("thread_ts", threadTS),
		)
		return nil
	}

	existing, err := uc.repo.Comment().GetBySlackTS(ctx, wsID, ticket.ID, messageTS)
	if err != nil {
		return goerr.Wrap(err, "failed to check duplicate comment")
	}
	if existing != nil {
		logger.Debug("slack thread reply ignored: comment already exists",
			slog.String("ticket_id", ticket.ID),
			slog.String("message_ts", messageTS),
		)
		return nil
	}

	comment := &model.Comment{
		ID:          uuid.Must(uuid.NewV7()).String(),
		TicketID:    ticket.ID,
		SlackUserID: userID,
		IsBot:       isBot,
		Body:        text,
		SlackTS:     messageTS,
		CreatedAt:   time.Now(),
	}

	if _, err := uc.repo.Comment().Create(ctx, wsID, ticket.ID, comment); err != nil {
		return goerr.Wrap(err, "failed to create comment from slack thread")
	}

	logger.Debug("comment created from slack thread reply",
		slog.String("ticket_id", ticket.ID),
		slog.String("comment_id", comment.ID),
	)

	return nil
}

func (uc *SlackUseCase) HandleMessageChanged(ctx context.Context, channelID, messageTS, newText string) error {
	logger := logging.From(ctx)

	entry, ok := uc.registry.GetBySlackChannelID(channelID)
	if !ok {
		logger.Debug("slack message_changed ignored: channel not mapped to workspace",
			slog.String("channel_id", channelID),
		)
		return nil
	}

	wsID := entry.Workspace.ID

	ticket, err := uc.repo.Ticket().GetBySlackThreadTS(ctx, wsID, channelID, messageTS)
	if err != nil {
		return goerr.Wrap(err, "failed to find ticket by thread_ts for message_changed")
	}
	if ticket == nil {
		logger.Debug("slack message_changed ignored: no ticket for message",
			slog.String("channel_id", channelID),
			slog.String("message_ts", messageTS),
		)
		return nil
	}

	ticket.InitialMessage = newText
	ticket.UpdatedAt = time.Now()
	if _, err := uc.repo.Ticket().Update(ctx, wsID, ticket); err != nil {
		return goerr.Wrap(err, "failed to update initial message")
	}

	logger.Debug("initial message updated from slack message_changed",
		slog.String("ticket_id", ticket.ID),
	)

	return nil
}

func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
