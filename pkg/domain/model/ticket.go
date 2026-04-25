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
	StatusID            string
	AssigneeID          string
	ReporterSlackUserID string
	SlackChannelID      string
	SlackThreadTS       string
	FieldValues         map[string]FieldValue
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type FieldValue struct {
	FieldID string
	Type    types.FieldType
	Value   any
}

type Comment struct {
	ID          string
	TicketID    string
	SlackUserID string
	Body        string
	SlackTS     string
	CreatedAt   time.Time
}
