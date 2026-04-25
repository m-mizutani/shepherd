package model

import (
	"time"

	"github.com/m-mizutani/shepherd/pkg/domain/types"
)

type Ticket struct {
	ID                  string
	WorkspaceID         string
	SeqNum              int64
	Title               string
	Description         string
	InitialMessage      string
	StatusID            string
	AssigneeID          string
	ReporterSlackUserID string
	SlackChannelID      string
	SlackThreadTS       string
	FieldValues         map[types.FieldID]FieldValue
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type FieldValue struct {
	FieldID types.FieldID
	Type    types.FieldType
	Value   any
}

type Comment struct {
	ID          string
	TicketID    string
	SlackUserID string
	IsBot       bool
	Body        string
	SlackTS     string
	CreatedAt   time.Time
}

type TicketHistory struct {
	ID          string
	NewStatusID string
	OldStatusID string
	ChangedBy   string
	Action      string
	CreatedAt   time.Time
}
