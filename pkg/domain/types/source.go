package types

// SourceID identifies a registered Source within a workspace.
type SourceID string

func (id SourceID) String() string { return string(id) }

// SourceProvider names an integration that contributes a Source kind. The
// values double as ToolFactory provider IDs (see pkg/tool) so the WebUI can
// gate tools and sources by the same key.
type SourceProvider string

const (
	SourceProviderNotion SourceProvider = "notion"
)

func (p SourceProvider) String() string { return string(p) }

func (p SourceProvider) IsValid() bool {
	switch p {
	case SourceProviderNotion:
		return true
	}
	return false
}

// NotionObjectType is the Notion-side kind a Source points at.
type NotionObjectType string

const (
	NotionObjectPage     NotionObjectType = "page"
	NotionObjectDatabase NotionObjectType = "database"
)

func (t NotionObjectType) String() string { return string(t) }

func (t NotionObjectType) IsValid() bool {
	switch t {
	case NotionObjectPage, NotionObjectDatabase:
		return true
	}
	return false
}
