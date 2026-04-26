package model

import (
	"time"

	"github.com/m-mizutani/shepherd/pkg/domain/types"
)

// Source is a per-workspace search target registered through the WebUI. The
// Provider field identifies which integration owns the entry; only the
// matching provider-specific sub-struct is non-nil.
type Source struct {
	ID          types.SourceID
	WorkspaceID types.WorkspaceID
	Provider    types.SourceProvider
	// Description is free-text the user adds to explain what this Source is.
	// Surfaced to the LLM (via list_sources) so it can pick the right Source
	// to search.
	Description string
	Notion      *NotionSource
	CreatedAt   time.Time
	CreatedBy   string
}

// NotionSource is the Notion-specific payload for a Source whose Provider is
// SourceProviderNotion. ObjectID is the canonical 32-hex (no hyphens) form;
// Title is best-effort metadata captured at registration time.
type NotionSource struct {
	ObjectType types.NotionObjectType
	ObjectID   string
	URL        string
	Title      string
}
