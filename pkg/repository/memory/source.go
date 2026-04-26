package memory

import (
	"context"
	"sync"

	"github.com/google/uuid"
	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/shepherd/pkg/domain/interfaces"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
)

type SourceRepo struct {
	mu      sync.RWMutex
	sources map[types.WorkspaceID]map[types.SourceID]*model.Source
}

func newSourceRepo() *SourceRepo {
	return &SourceRepo{sources: make(map[types.WorkspaceID]map[types.SourceID]*model.Source)}
}

var _ interfaces.SourceRepository = (*SourceRepo)(nil)

func (r *SourceRepo) Create(ctx context.Context, s *model.Source) (*model.Source, error) {
	if s == nil {
		return nil, goerr.New("source is nil")
	}
	if s.WorkspaceID == "" {
		return nil, goerr.New("source workspace_id is required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.sources[s.WorkspaceID] == nil {
		r.sources[s.WorkspaceID] = make(map[types.SourceID]*model.Source)
	}
	if s.ID == "" {
		s.ID = types.SourceID(uuid.Must(uuid.NewV7()).String())
	}
	copied := cloneSource(s)
	r.sources[s.WorkspaceID][s.ID] = copied
	return cloneSource(copied), nil
}

func (r *SourceRepo) Get(ctx context.Context, ws types.WorkspaceID, id types.SourceID) (*model.Source, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	bucket := r.sources[ws]
	src, ok := bucket[id]
	if !ok {
		return nil, goerr.New("source not found",
			goerr.V("workspace_id", string(ws)),
			goerr.V("source_id", string(id)))
	}
	return cloneSource(src), nil
}

func (r *SourceRepo) List(ctx context.Context, ws types.WorkspaceID) ([]*model.Source, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	bucket := r.sources[ws]
	out := make([]*model.Source, 0, len(bucket))
	for _, s := range bucket {
		out = append(out, cloneSource(s))
	}
	return out, nil
}

func (r *SourceRepo) ListByProvider(ctx context.Context, ws types.WorkspaceID, p types.SourceProvider) ([]*model.Source, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	bucket := r.sources[ws]
	out := make([]*model.Source, 0, len(bucket))
	for _, s := range bucket {
		if s.Provider == p {
			out = append(out, cloneSource(s))
		}
	}
	return out, nil
}

func (r *SourceRepo) UpdateDescription(ctx context.Context, ws types.WorkspaceID, id types.SourceID, description string) (*model.Source, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	bucket := r.sources[ws]
	src, ok := bucket[id]
	if !ok {
		return nil, goerr.New("source not found",
			goerr.V("workspace_id", string(ws)),
			goerr.V("source_id", string(id)))
	}
	src.Description = description
	return cloneSource(src), nil
}

func (r *SourceRepo) Delete(ctx context.Context, ws types.WorkspaceID, id types.SourceID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	bucket := r.sources[ws]
	if _, ok := bucket[id]; !ok {
		return goerr.New("source not found",
			goerr.V("workspace_id", string(ws)),
			goerr.V("source_id", string(id)))
	}
	delete(bucket, id)
	return nil
}

func cloneSource(s *model.Source) *model.Source {
	if s == nil {
		return nil
	}
	c := *s
	if s.Notion != nil {
		n := *s.Notion
		c.Notion = &n
	}
	return &c
}
