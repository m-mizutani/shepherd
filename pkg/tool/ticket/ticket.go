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
	"github.com/m-mizutani/shepherd/pkg/tool"
	"github.com/urfave/cli/v3"
)

// Factory implements tool.ToolFactory for ticket tools.
type Factory struct {
	repo     interfaces.Repository
	embedder interfaces.Embedder
	tools    []gollem.Tool
}

func New(repo interfaces.Repository, embedder interfaces.Embedder) *Factory {
	return &Factory{repo: repo, embedder: embedder}
}

func (f *Factory) ID() tool.ProviderID { return tool.ProviderTicket }
func (f *Factory) Flags() []cli.Flag   { return nil }

func (f *Factory) Init(_ context.Context) error {
	f.tools = []gollem.Tool{
		newSearchTool(f.repo, f.embedder),
		newGetTool(f.repo),
		newGetCommentsTool(f.repo),
		newGetHistoryTool(f.repo),
	}
	return nil
}

func (f *Factory) Available() bool      { return f.repo != nil }
func (f *Factory) Tools() []gollem.Tool { return f.tools }
func (f *Factory) DefaultEnabled() bool { return true }

// Prompt returns provider-level narrative for the ticket tools. The real
// content is rendered in prompt.go; this signature satisfies tool.ToolFactory.
func (f *Factory) Prompt(ctx context.Context, ws types.WorkspaceID) (string, error) {
	return renderPrompt()
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
