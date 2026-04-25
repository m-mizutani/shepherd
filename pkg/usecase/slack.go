package usecase

import (
	"context"
	"log/slog"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/shepherd/pkg/domain/interfaces"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
	slackService "github.com/m-mizutani/shepherd/pkg/service/slack"
	"github.com/m-mizutani/shepherd/pkg/usecase/prompt"
	"github.com/m-mizutani/shepherd/pkg/utils/errutil"
	"github.com/m-mizutani/shepherd/pkg/utils/logging"
)

type SlackUseCase struct {
	repo      interfaces.Repository
	registry  *model.WorkspaceRegistry
	slack     *slackService.Client
	baseURL   string
	llm       gollem.LLMClient
	userCache sync.Map
}

func NewSlackUseCase(repo interfaces.Repository, registry *model.WorkspaceRegistry, slack *slackService.Client, baseURL string, llm gollem.LLMClient) *SlackUseCase {
	return &SlackUseCase{
		repo:     repo,
		registry: registry,
		slack:    slack,
		baseURL:  baseURL,
		llm:      llm,
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
		slog.String("workspace_id", string(wsID)),
		slog.String("channel_id", channelID),
		slog.String("user_id", userID),
		slog.String("message_ts", messageTS),
	)

	existing, err := uc.repo.Ticket().GetBySlackThreadTS(ctx, wsID, types.SlackChannelID(channelID), types.SlackThreadTS(messageTS))
	if err != nil {
		return goerr.Wrap(err, "failed to check duplicate ticket")
	}
	if existing != nil {
		logger.Debug("slack message ignored: ticket already exists",
			slog.String("ticket_id", string(existing.ID)),
			slog.String("message_ts", messageTS),
		)
		return nil
	}

	now := time.Now()
	ticket := &model.Ticket{
		ID:                  types.TicketID(uuid.Must(uuid.NewV7()).String()),
		WorkspaceID:         wsID,
		Title:               truncate(text, 200),
		Description:         text,
		InitialMessage:      text,
		StatusID:            entry.FieldSchema.TicketConfig.DefaultStatusID,
		ReporterSlackUserID: types.SlackUserID(userID),
		SlackChannelID:      types.SlackChannelID(channelID),
		SlackThreadTS:       types.SlackThreadTS(messageTS),
		FieldValues:         make(map[string]model.FieldValue),
		CreatedAt:           now,
		UpdatedAt:           now,
	}

	created, err := uc.repo.Ticket().Create(ctx, wsID, ticket)
	if err != nil {
		return goerr.Wrap(err, "failed to create ticket from slack message")
	}

	history := &model.TicketHistory{
		ID:          uuid.Must(uuid.NewV7()).String(),
		NewStatusID: created.StatusID,
		ChangedBy:   types.SlackUserID(userID),
		Action:      "created",
		CreatedAt:   now,
	}
	if _, err := uc.repo.TicketHistory().Create(ctx, wsID, created.ID, history); err != nil {
		errutil.Handle(ctx, goerr.Wrap(err, "failed to create ticket history"))
	}

	logger.Info("ticket created from slack message",
		slog.String("workspace_id", string(wsID)),
		slog.String("ticket_id", string(created.ID)),
		slog.Int64("seq_num", created.SeqNum),
		slog.String("channel_id", channelID),
	)

	ticketURL, _ := url.JoinPath(uc.baseURL, "ws", string(wsID), "tickets", string(created.ID))
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

	ticket, err := uc.repo.Ticket().GetBySlackThreadTS(ctx, wsID, types.SlackChannelID(channelID), types.SlackThreadTS(threadTS))
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

	existing, err := uc.repo.Comment().GetBySlackTS(ctx, wsID, ticket.ID, types.SlackThreadTS(messageTS))
	if err != nil {
		return goerr.Wrap(err, "failed to check duplicate comment")
	}
	if existing != nil {
		logger.Debug("slack thread reply ignored: comment already exists",
			slog.String("ticket_id", string(ticket.ID)),
			slog.String("message_ts", messageTS),
		)
		return nil
	}

	comment := &model.Comment{
		ID:          types.CommentID(uuid.Must(uuid.NewV7()).String()),
		TicketID:    ticket.ID,
		SlackUserID: types.SlackUserID(userID),
		IsBot:       isBot,
		Body:        text,
		SlackTS:     types.SlackThreadTS(messageTS),
		CreatedAt:   time.Now(),
	}

	if _, err := uc.repo.Comment().Create(ctx, wsID, ticket.ID, comment); err != nil {
		return goerr.Wrap(err, "failed to create comment from slack thread")
	}

	logger.Debug("comment created from slack thread reply",
		slog.String("ticket_id", string(ticket.ID)),
		slog.String("comment_id", string(comment.ID)),
	)

	return nil
}

// HandleAppMention responds to an app_mention event by feeding the ticket
// thread context to the configured LLM and posting the reply back to Slack.
// When the LLM is not configured, or when the mentioned thread does not map to
// a known ticket, the call is a debug-logged no-op.
func (uc *SlackUseCase) HandleAppMention(ctx context.Context, channelID, userID, text, messageTS, threadTS string) error {
	logger := logging.From(ctx)

	if uc.llm == nil {
		logger.Debug("app_mention ignored: LLM not configured",
			slog.String("channel_id", channelID),
		)
		return nil
	}

	rootTS := threadTS
	if rootTS == "" {
		rootTS = messageTS
	}

	entry, ok := uc.registry.GetBySlackChannelID(channelID)
	if !ok {
		logger.Debug("app_mention ignored: channel not mapped to workspace",
			slog.String("channel_id", channelID),
		)
		return nil
	}
	wsID := entry.Workspace.ID

	ticket, err := uc.repo.Ticket().GetBySlackThreadTS(ctx, wsID, types.SlackChannelID(channelID), types.SlackThreadTS(rootTS))
	if err != nil {
		return goerr.Wrap(err, "failed to find ticket for app_mention")
	}
	if ticket == nil {
		logger.Debug("app_mention ignored: no ticket for thread",
			slog.String("channel_id", channelID),
			slog.String("thread_ts", rootTS),
		)
		return nil
	}

	comments, err := uc.repo.Comment().List(ctx, wsID, ticket.ID)
	if err != nil {
		return goerr.Wrap(err, "failed to list comments for app_mention")
	}

	mentionAuthor := uc.resolveDisplayName(ctx, userID)
	rendered, err := prompt.RenderMention(prompt.MentionInput{
		Title:          ticket.Title,
		Description:    ticket.Description,
		InitialMessage: ticket.InitialMessage,
		Comments:       uc.buildMentionComments(ctx, comments),
		MentionAuthor:  mentionAuthor,
		Mention:        stripMentionTokens(text),
	})
	if err != nil {
		return goerr.Wrap(err, "failed to render mention prompt")
	}

	session, err := uc.llm.NewSession(ctx)
	if err != nil {
		return goerr.Wrap(err, "failed to start LLM session")
	}
	resp, err := session.Generate(ctx, []gollem.Input{gollem.Text(rendered)})
	if err != nil {
		return goerr.Wrap(err, "failed to generate LLM response")
	}
	if resp == nil || len(resp.Texts) == 0 {
		logger.Debug("app_mention: LLM returned no text",
			slog.String("ticket_id", string(ticket.ID)),
		)
		return nil
	}

	reply := strings.TrimSpace(strings.Join(resp.Texts, "\n"))
	if reply == "" {
		return nil
	}

	if err := uc.slack.ReplyThread(ctx, channelID, rootTS, reply); err != nil {
		return goerr.Wrap(err, "failed to reply to app_mention")
	}

	logger.Info("app_mention answered",
		slog.String("workspace_id", string(wsID)),
		slog.String("ticket_id", string(ticket.ID)),
		slog.String("channel_id", channelID),
	)
	return nil
}

func (uc *SlackUseCase) buildMentionComments(ctx context.Context, comments []*model.Comment) []prompt.MentionComment {
	out := make([]prompt.MentionComment, 0, len(comments))
	for _, c := range comments {
		role := "user"
		if c.IsBot {
			role = "bot"
		}
		out = append(out, prompt.MentionComment{
			Author: uc.resolveDisplayName(ctx, string(c.SlackUserID)),
			Role:   role,
			Body:   c.Body,
		})
	}
	return out
}

func (uc *SlackUseCase) resolveDisplayName(ctx context.Context, userID string) string {
	if userID == "" {
		return "unknown"
	}
	info, err := uc.GetUserInfo(ctx, userID)
	if err != nil || info == nil || info.Name == "" {
		return userID
	}
	return info.Name
}

// stripMentionTokens removes Slack mention tokens like "<@U12345>" from the
// raw event text so the LLM only sees the human-written content.
func stripMentionTokens(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		if s[i] == '<' {
			if end := strings.IndexByte(s[i:], '>'); end > 0 {
				token := s[i : i+end+1]
				if strings.HasPrefix(token, "<@") || strings.HasPrefix(token, "<!") {
					i += end + 1
					continue
				}
			}
		}
		b.WriteByte(s[i])
		i++
	}
	return strings.TrimSpace(b.String())
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

	ticket, err := uc.repo.Ticket().GetBySlackThreadTS(ctx, wsID, types.SlackChannelID(channelID), types.SlackThreadTS(messageTS))
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
		slog.String("ticket_id", string(ticket.ID)),
	)

	return nil
}

func (uc *SlackUseCase) ListUsers(ctx context.Context) ([]*slackService.UserInfo, error) {
	return uc.slack.ListUsers(ctx)
}

type userInfoCacheEntry struct {
	info      *slackService.UserInfo
	expiresAt time.Time
}

func (uc *SlackUseCase) GetUserInfo(ctx context.Context, userID string) (*slackService.UserInfo, error) {
	if v, ok := uc.userCache.Load(userID); ok {
		entry := v.(*userInfoCacheEntry)
		if time.Now().Before(entry.expiresAt) {
			return entry.info, nil
		}
		uc.userCache.Delete(userID)
	}

	info, err := uc.slack.GetUserInfo(ctx, userID)
	if err != nil {
		return nil, err
	}

	uc.userCache.Store(userID, &userInfoCacheEntry{
		info:      info,
		expiresAt: time.Now().Add(3 * time.Minute),
	})
	return info, nil
}

func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
