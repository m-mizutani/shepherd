package model

// SourceInfo represents information extracted from source code events (release, push, etc.)
type SourceInfo struct {
	Owner     string            // Repository owner
	Repo      string            // Repository name
	CommitSHA string            // Commit SHA
	EventType string            // Event type: "release", "push", etc.
	Ref       string            // Git ref (branch/tag)
	Actor     string            // User who triggered the event
	Metadata  map[string]string // Event-specific metadata
}
