package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/shepherd/pkg/domain/interfaces"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
	slackService "github.com/m-mizutani/shepherd/pkg/service/slack"
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
	entry, ok := uc.registry.GetBySlackChannelID(channelID)
	if !ok {
		return nil
	}

	wsID := entry.Workspace.ID

	existing, err := uc.repo.Ticket().GetBySlackThreadTS(ctx, wsID, channelID, messageTS)
	if err != nil {
		return goerr.Wrap(err, "failed to check duplicate ticket")
	}
	if existing != nil {
		return nil
	}

	now := time.Now()
	ticket := &model.Ticket{
		ID:                  uuid.Must(uuid.NewV7()).String(),
		WorkspaceID:         wsID,
		Title:               truncate(text, 200),
		Description:         text,
		StatusID:            entry.FieldSchema.TicketConfig.DefaultStatusID,
		ReporterSlackUserID: userID,
		SlackChannelID:      channelID,
		SlackThreadTS:       messageTS,
		FieldValues:         make(map[string]model.FieldValue),
		CreatedAt:           now,
		UpdatedAt:           now,
	}

	created, err := uc.repo.Ticket().Create(ctx, wsID, ticket)
	if err != nil {
		return goerr.Wrap(err, "failed to create ticket from slack message")
	}

	ticketURL := fmt.Sprintf("%s/ws/%s/tickets/%s", uc.baseURL, wsID, created.ID)
	if err := uc.slack.ReplyTicketCreated(ctx, channelID, messageTS, created.SeqNum, ticketURL); err != nil {
		return goerr.Wrap(err, "failed to reply ticket created")
	}

	return nil
}

func (uc *SlackUseCase) HandleThreadReply(ctx context.Context, channelID, threadTS, userID, text, messageTS string) error {
	entry, ok := uc.registry.GetBySlackChannelID(channelID)
	if !ok {
		return nil
	}

	wsID := entry.Workspace.ID

	ticket, err := uc.repo.Ticket().GetBySlackThreadTS(ctx, wsID, channelID, threadTS)
	if err != nil {
		return goerr.Wrap(err, "failed to find ticket by thread_ts")
	}
	if ticket == nil {
		return nil
	}

	existing, err := uc.repo.Comment().GetBySlackTS(ctx, wsID, ticket.ID, messageTS)
	if err != nil {
		return goerr.Wrap(err, "failed to check duplicate comment")
	}
	if existing != nil {
		return nil
	}

	comment := &model.Comment{
		ID:          uuid.Must(uuid.NewV7()).String(),
		TicketID:    ticket.ID,
		SlackUserID: userID,
		Body:        text,
		SlackTS:     messageTS,
		CreatedAt:   time.Now(),
	}

	if _, err := uc.repo.Comment().Create(ctx, wsID, ticket.ID, comment); err != nil {
		return goerr.Wrap(err, "failed to create comment from slack thread")
	}

	return nil
}

func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
