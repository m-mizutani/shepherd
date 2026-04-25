package format_test

import (
	"testing"
	"time"

	"github.com/m-mizutani/gt"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
	slackService "github.com/m-mizutani/shepherd/pkg/service/slack"
	"github.com/m-mizutani/shepherd/pkg/tool/internal/format"
)

func TestTicket(t *testing.T) {
	created := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	tk := &model.Ticket{
		ID:                  "tk-1",
		SeqNum:              42,
		Title:               "boom",
		Description:         "details",
		StatusID:            "open",
		AssigneeID:          "U-A",
		ReporterSlackUserID: "U-R",
		SlackChannelID:      "C-1",
		SlackThreadTS:       "1.0",
		FieldValues: map[string]model.FieldValue{
			"severity": {FieldID: "severity", Type: types.FieldTypeText, Value: "high"},
		},
		CreatedAt: created,
		UpdatedAt: created,
	}
	got := format.Ticket(tk, "Open")
	gt.Equal(t, got["id"], "tk-1")
	gt.Equal(t, got["seq_num"].(int64), int64(42))
	gt.Equal(t, got["status_name"], "Open")
	fields := got["fields"].(map[string]any)
	gt.Equal(t, fields["severity"], "high")
	gt.Equal(t, got["created_at"], "2026-01-02T03:04:05Z")
}

func TestTicketSummary(t *testing.T) {
	tk := &model.Ticket{ID: "tk-1", SeqNum: 7, Title: "x", StatusID: "open"}
	got := format.TicketSummary(tk, "Open")
	gt.Equal(t, got["seq_num"].(int64), int64(7))
	_, hasDesc := got["description"]
	gt.False(t, hasDesc)
}

func TestNilInputs(t *testing.T) {
	gt.Nil(t, format.Ticket(nil, ""))
	gt.Nil(t, format.Comment(nil, ""))
	gt.Nil(t, format.History(nil))
	gt.Nil(t, format.SlackMessage(nil))
	gt.Nil(t, format.SlackSearchMatch(nil))
}

func TestSlackMessage(t *testing.T) {
	m := &slackService.Message{User: "U", Text: "hi", Timestamp: "1.0", ThreadTS: "0.9", BotID: "B"}
	got := format.SlackMessage(m)
	gt.Equal(t, got["user"], "U")
	gt.Equal(t, got["text"], "hi")
	gt.Equal(t, got["thread_ts"], "0.9")
}
