package model

import (
	"github.com/m-mizutani/shepherd/pkg/domain/model/config"
)

type Workspace struct {
	ID   string
	Name string
}

type WorkspaceEntry struct {
	Workspace       Workspace
	FieldSchema     *config.FieldSchema
	SlackChannelID  string
}

type WorkspaceRegistry struct {
	entries map[string]*WorkspaceEntry
	order   []string
}

func NewWorkspaceRegistry() *WorkspaceRegistry {
	return &WorkspaceRegistry{
		entries: make(map[string]*WorkspaceEntry),
	}
}

func (r *WorkspaceRegistry) Register(entry *WorkspaceEntry) {
	id := entry.Workspace.ID
	if _, exists := r.entries[id]; !exists {
		r.order = append(r.order, id)
	}
	r.entries[id] = entry
}

func (r *WorkspaceRegistry) Get(id string) (*WorkspaceEntry, bool) {
	entry, ok := r.entries[id]
	return entry, ok
}

func (r *WorkspaceRegistry) List() []*WorkspaceEntry {
	result := make([]*WorkspaceEntry, 0, len(r.order))
	for _, id := range r.order {
		result = append(result, r.entries[id])
	}
	return result
}

func (r *WorkspaceRegistry) Workspaces() []Workspace {
	result := make([]Workspace, 0, len(r.order))
	for _, id := range r.order {
		result = append(result, r.entries[id].Workspace)
	}
	return result
}

func (r *WorkspaceRegistry) GetBySlackChannelID(channelID string) (*WorkspaceEntry, bool) {
	for _, entry := range r.entries {
		if entry.SlackChannelID == channelID {
			return entry, true
		}
	}
	return nil, false
}
