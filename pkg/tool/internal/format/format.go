// Package format converts domain models to flat map[string]any structures
// suitable for returning from gollem.Tool.Run.
package format

import (
	"time"

	"github.com/m-mizutani/shepherd/pkg/domain/model"
	slackService "github.com/m-mizutani/shepherd/pkg/service/slack"
)

// Ticket renders a ticket as a JSON-friendly map. statusName is resolved by
// the caller against the workspace's field schema (empty string if unknown).
func Ticket(t *model.Ticket, statusName string) map[string]any {
	if t == nil {
		return nil
	}
	fields := make(map[string]any, len(t.FieldValues))
	for k, v := range t.FieldValues {
		fields[k] = v.Value
	}
	return map[string]any{
		"id":               string(t.ID),
		"seq_num":          t.SeqNum,
		"title":            t.Title,
		"description":      t.Description,
		"status_id":        string(t.StatusID),
		"status_name":      statusName,
		"assignee":         string(t.AssigneeID),
		"reporter":         string(t.ReporterSlackUserID),
		"slack_channel_id": string(t.SlackChannelID),
		"slack_thread_ts":  string(t.SlackThreadTS),
		"fields":           fields,
		"created_at":       t.CreatedAt.Format(time.RFC3339),
		"updated_at":       t.UpdatedAt.Format(time.RFC3339),
	}
}

// TicketSummary is a lighter form for search results, omitting verbose fields.
func TicketSummary(t *model.Ticket, statusName string) map[string]any {
	if t == nil {
		return nil
	}
	return map[string]any{
		"id":          string(t.ID),
		"seq_num":     t.SeqNum,
		"title":       t.Title,
		"status_id":   string(t.StatusID),
		"status_name": statusName,
		"updated_at":  t.UpdatedAt.Format(time.RFC3339),
	}
}

// Comment renders a single comment.
func Comment(c *model.Comment, authorName string) map[string]any {
	if c == nil {
		return nil
	}
	return map[string]any{
		"id":          string(c.ID),
		"author":      authorName,
		"author_id":   string(c.SlackUserID),
		"is_bot":      c.IsBot,
		"body":        c.Body,
		"slack_ts":    string(c.SlackTS),
		"created_at":  c.CreatedAt.Format(time.RFC3339),
	}
}

// History renders a status-change history entry.
func History(h *model.TicketHistory) map[string]any {
	if h == nil {
		return nil
	}
	return map[string]any{
		"id":             h.ID,
		"action":         h.Action,
		"old_status_id":  string(h.OldStatusID),
		"new_status_id":  string(h.NewStatusID),
		"changed_by":     string(h.ChangedBy),
		"created_at":     h.CreatedAt.Format(time.RFC3339),
	}
}

// SlackMessage renders a service-layer Slack message.
func SlackMessage(m *slackService.Message) map[string]any {
	if m == nil {
		return nil
	}
	return map[string]any{
		"user":      m.User,
		"text":      m.Text,
		"timestamp": m.Timestamp,
		"thread_ts": m.ThreadTS,
		"bot_id":    m.BotID,
	}
}

// SlackSearchMatch renders a search.messages hit.
func SlackSearchMatch(m *slackService.SearchMatch) map[string]any {
	if m == nil {
		return nil
	}
	return map[string]any{
		"channel_id":   m.ChannelID,
		"channel_name": m.ChannelName,
		"user":         m.User,
		"username":     m.Username,
		"text":         m.Text,
		"timestamp":    m.Timestamp,
		"permalink":    m.Permalink,
	}
}

