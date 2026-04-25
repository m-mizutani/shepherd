// Package ticket exposes gollem.Tool implementations for reading tickets,
// their comments, and their status history out of the workspace repository.
// All tools require a workspace ID bound to the context via
// types.ContextWithWorkspace.
package ticket

import (
	"context"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/shepherd/pkg/domain/interfaces"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
)

// Deps bundles the repository tools depend on.
type Deps struct {
	Repo interfaces.Repository
}

// Tools returns every gollem.Tool exported from this package.
func Tools(d Deps) []gollem.Tool {
	return []gollem.Tool{
		newSearchTool(d.Repo),
		newGetTool(d.Repo),
		newGetCommentsTool(d.Repo),
		newGetHistoryTool(d.Repo),
	}
}

// workspaceFromCtx returns the active workspace ID, erroring out cleanly when
// the caller forgot to bind one. The error message is structured for the LLM.
func workspaceFromCtx(ctx context.Context) (types.WorkspaceID, error) {
	id, ok := types.WorkspaceFromContext(ctx)
	if !ok || id == "" {
		return "", goerr.New("no active workspace bound to context")
	}
	return id, nil
}
