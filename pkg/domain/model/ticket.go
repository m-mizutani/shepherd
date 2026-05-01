package model

import (
	"time"

	"github.com/m-mizutani/shepherd/pkg/domain/types"
)

type Ticket struct {
	ID                  types.TicketID
	WorkspaceID         types.WorkspaceID
	SeqNum              int64
	Title               string
	Description         string
	InitialMessage      string
	StatusID            types.StatusID
	AssigneeIDs         []types.SlackUserID
	ReporterSlackUserID types.SlackUserID
	SlackChannelID      types.SlackChannelID
	SlackThreadTS       types.SlackThreadTS
	FieldValues         map[string]FieldValue
	Triaged             bool
	Conclusion          string
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type FieldValue struct {
	FieldID types.FieldID
	Type    types.FieldType
	Value   any
}

type Comment struct {
	ID          types.CommentID
	TicketID    types.TicketID
	SlackUserID types.SlackUserID
	IsBot       bool
	Body        string
	SlackTS     types.SlackThreadTS
	CreatedAt   time.Time
}

type TicketHistory struct {
	ID          string
	NewStatusID types.StatusID
	OldStatusID types.StatusID
	ChangedBy   types.SlackUserID
	Action      string
	CreatedAt   time.Time
}
