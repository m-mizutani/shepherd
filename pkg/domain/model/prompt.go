package model

import (
	"time"

	"github.com/m-mizutani/shepherd/pkg/domain/types"
)

// PromptID identifies a customizable prompt slot. The set is open-ended so
// new slots (Summarize, Reply Drafts, ...) can be added without schema
// changes.
type PromptID string

const (
	// PromptIDTriage is the planner system prompt used by the triage agent.
	PromptIDTriage PromptID = "triage"
)

// PromptVersion is one append-only revision of a workspace-level prompt
// override. Version is a 1-based monotonic integer assigned at write time;
// the highest-Version row for (WorkspaceID, PromptID) is the "current"
// revision. Older entries are never mutated or deleted.
type PromptVersion struct {
	WorkspaceID    types.WorkspaceID
	PromptID       PromptID
	Version        int
	Content        string
	UpdatedAt      time.Time
	UpdatedBy      string
	UpdatedByEmail string
	UpdatedBySub   string
}
