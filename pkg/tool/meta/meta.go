// Package meta exposes ambient tools that describe the workspace itself or
// expose runtime information (current time) to the LLM.
package meta

import (
	"context"
	"time"

	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
	"github.com/m-mizutani/shepherd/pkg/tool"
	"github.com/urfave/cli/v3"
)

// Factory implements tool.ToolFactory for meta tools.
type Factory struct {
	registry *model.WorkspaceRegistry
	now      func() time.Time
	tools    []gollem.Tool
}

// New constructs a Factory. now defaults to time.Now when nil.
func New(registry *model.WorkspaceRegistry, now func() time.Time) *Factory {
	if now == nil {
		now = time.Now
	}
	return &Factory{registry: registry, now: now}
}

func (f *Factory) ID() tool.ProviderID { return tool.ProviderMeta }
func (f *Factory) Flags() []cli.Flag   { return nil }

func (f *Factory) Init(_ context.Context) error {
	f.tools = []gollem.Tool{
		newWorkspaceDescribeTool(f.registry),
		newCurrentTimeTool(f.now),
	}
	return nil
}

func (f *Factory) Available() bool      { return true }
func (f *Factory) Tools() []gollem.Tool { return f.tools }
func (f *Factory) DefaultEnabled() bool { return true }

// Prompt returns provider-level narrative for meta tools. Static markdown
// rendered from prompt.go; this signature satisfies tool.ToolFactory.
func (f *Factory) Prompt(ctx context.Context, ws types.WorkspaceID) (string, error) {
	return renderPrompt()
}
