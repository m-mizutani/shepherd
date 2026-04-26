package model

import (
	"time"

	"github.com/m-mizutani/shepherd/pkg/domain/types"
)

// ToolSettings stores per-workspace enable/disable flags for tool factories
// keyed by ToolFactory.ID(). Missing keys mean "use the factory's
// DefaultEnabled() value" — Enabled is therefore an override map, not an
// authoritative list.
type ToolSettings struct {
	WorkspaceID types.WorkspaceID
	Enabled     map[string]bool
	UpdatedAt   time.Time
}

// IsEnabled returns the effective on/off state for providerID, falling back to
// defaultEnabled when no override is recorded.
func (s *ToolSettings) IsEnabled(providerID string, defaultEnabled bool) bool {
	if s == nil || s.Enabled == nil {
		return defaultEnabled
	}
	v, ok := s.Enabled[providerID]
	if !ok {
		return defaultEnabled
	}
	return v
}
