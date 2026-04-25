package usecase

import (
	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
	"github.com/m-mizutani/shepherd/pkg/domain/model/config"
	"github.com/m-mizutani/shepherd/pkg/utils/errutil"
)

type WorkspaceUseCase struct {
	registry *model.WorkspaceRegistry
}

func NewWorkspaceUseCase(registry *model.WorkspaceRegistry) *WorkspaceUseCase {
	return &WorkspaceUseCase{
		registry: registry,
	}
}

func (uc *WorkspaceUseCase) List() []model.Workspace {
	return uc.registry.Workspaces()
}

func (uc *WorkspaceUseCase) Get(workspaceID string) (*model.Workspace, error) {
	entry, ok := uc.registry.Get(workspaceID)
	if !ok {
		return nil, goerr.New("workspace not found", goerr.V("workspace_id", workspaceID), goerr.Tag(errutil.TagNotFound))
	}
	return &entry.Workspace, nil
}

func (uc *WorkspaceUseCase) GetConfig(workspaceID string) (*config.FieldSchema, error) {
	entry, ok := uc.registry.Get(workspaceID)
	if !ok {
		return nil, goerr.New("workspace not found", goerr.V("workspace_id", workspaceID), goerr.Tag(errutil.TagNotFound))
	}
	return entry.FieldSchema, nil
}
