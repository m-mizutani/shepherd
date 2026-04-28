package memory

import (
	"context"
	"sync"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/shepherd/pkg/domain/interfaces"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
)

type promptKey struct {
	ws types.WorkspaceID
	id model.PromptID
}

type PromptRepo struct {
	mu       sync.Mutex
	versions map[promptKey][]*model.PromptVersion
}

func newPromptRepo() *PromptRepo {
	return &PromptRepo{versions: make(map[promptKey][]*model.PromptVersion)}
}

var _ interfaces.PromptRepository = (*PromptRepo)(nil)

func (r *PromptRepo) Append(ctx context.Context, ws types.WorkspaceID, id model.PromptID, draft *model.PromptVersion) (*model.PromptVersion, error) {
	if draft == nil {
		return nil, goerr.New("draft is nil")
	}
	if draft.Version < 1 {
		return nil, goerr.New("draft.Version must be >= 1",
			goerr.V("version", draft.Version))
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	key := promptKey{ws: ws, id: id}
	current := len(r.versions[key])
	if draft.Version != current+1 {
		return nil, goerr.Wrap(interfaces.ErrPromptVersionConflict,
			"version is not exactly current+1",
			goerr.V("workspace_id", string(ws)),
			goerr.V("prompt_id", string(id)),
			goerr.V("version", draft.Version),
			goerr.V("current_version", current))
	}

	stored := *draft
	stored.WorkspaceID = ws
	stored.PromptID = id
	r.versions[key] = append(r.versions[key], &stored)

	out := stored
	return &out, nil
}

func (r *PromptRepo) GetCurrent(ctx context.Context, ws types.WorkspaceID, id model.PromptID) (*model.PromptVersion, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	list := r.versions[promptKey{ws: ws, id: id}]
	if len(list) == 0 {
		return nil, nil
	}
	v := *list[len(list)-1]
	return &v, nil
}

func (r *PromptRepo) GetVersion(ctx context.Context, ws types.WorkspaceID, id model.PromptID, version int) (*model.PromptVersion, error) {
	if version < 1 {
		return nil, goerr.Wrap(interfaces.ErrPromptVersionNotFound,
			"version must be >= 1",
			goerr.V("version", version))
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	list := r.versions[promptKey{ws: ws, id: id}]
	if version > len(list) {
		return nil, goerr.Wrap(interfaces.ErrPromptVersionNotFound,
			"version is beyond current",
			goerr.V("workspace_id", string(ws)),
			goerr.V("prompt_id", string(id)),
			goerr.V("version", version),
			goerr.V("current_version", len(list)))
	}
	v := *list[version-1]
	return &v, nil
}

func (r *PromptRepo) List(ctx context.Context, ws types.WorkspaceID, id model.PromptID) ([]*model.PromptVersion, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	src := r.versions[promptKey{ws: ws, id: id}]
	out := make([]*model.PromptVersion, 0, len(src))
	for _, v := range src {
		c := *v
		out = append(out, &c)
	}
	return out, nil
}
