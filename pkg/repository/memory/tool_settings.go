package memory

import (
	"context"
	"sync"
	"time"

	"github.com/m-mizutani/shepherd/pkg/domain/interfaces"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
)

type ToolSettingsRepo struct {
	mu       sync.RWMutex
	settings map[types.WorkspaceID]*model.ToolSettings
}

func newToolSettingsRepo() *ToolSettingsRepo {
	return &ToolSettingsRepo{settings: make(map[types.WorkspaceID]*model.ToolSettings)}
}

var _ interfaces.ToolSettingsRepository = (*ToolSettingsRepo)(nil)

func (r *ToolSettingsRepo) Get(ctx context.Context, ws types.WorkspaceID) (*model.ToolSettings, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if s, ok := r.settings[ws]; ok {
		return cloneToolSettings(s), nil
	}
	return &model.ToolSettings{
		WorkspaceID: ws,
		Enabled:     map[string]bool{},
	}, nil
}

func (r *ToolSettingsRepo) Set(ctx context.Context, ws types.WorkspaceID, providerID string, enabled bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.settings[ws]
	if !ok {
		s = &model.ToolSettings{
			WorkspaceID: ws,
			Enabled:     map[string]bool{},
		}
		r.settings[ws] = s
	}
	s.Enabled[providerID] = enabled
	s.UpdatedAt = time.Now()
	return nil
}

func cloneToolSettings(s *model.ToolSettings) *model.ToolSettings {
	if s == nil {
		return nil
	}
	c := *s
	c.Enabled = make(map[string]bool, len(s.Enabled))
	for k, v := range s.Enabled {
		c.Enabled[k] = v
	}
	return &c
}
