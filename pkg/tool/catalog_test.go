package tool_test

import (
	"context"
	"testing"

	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/gt"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
	"github.com/m-mizutani/shepherd/pkg/repository/memory"
	"github.com/m-mizutani/shepherd/pkg/tool"
	"github.com/urfave/cli/v3"
)

type stubFactory struct {
	id        tool.ProviderID
	available bool
	def       bool
	tools     []gollem.Tool
}

func (f *stubFactory) ID() tool.ProviderID    { return f.id }
func (f *stubFactory) Flags() []cli.Flag      { return nil }
func (f *stubFactory) Init(context.Context) error {
	if f.tools == nil {
		f.tools = []gollem.Tool{stubTool(string(f.id) + "_only")}
	}
	return nil
}
func (f *stubFactory) Available() bool      { return f.available }
func (f *stubFactory) Tools() []gollem.Tool { return f.tools }
func (f *stubFactory) DefaultEnabled() bool { return f.def }

type stubTool string

func (s stubTool) Spec() gollem.ToolSpec               { return gollem.ToolSpec{Name: string(s)} }
func (s stubTool) Run(context.Context, map[string]any) (map[string]any, error) {
	return map[string]any{}, nil
}

func TestCatalog_GatingMatrix(t *testing.T) {
	ws := types.WorkspaceID("ws-1")
	repo := memory.New()
	t.Cleanup(func() { _ = repo.Close() })

	notion := &stubFactory{id: tool.ProviderNotion, available: true, def: false}
	slack := &stubFactory{id: tool.ProviderSlack, available: true, def: true}
	ticket := &stubFactory{id: tool.ProviderTicket, available: true, def: true}
	disabled := &stubFactory{id: "extra", available: false, def: true}

	for _, f := range []tool.ToolFactory{notion, slack, ticket, disabled} {
		gt.NoError(t, f.Init(context.Background()))
	}

	cat := tool.NewCatalog([]tool.ToolFactory{notion, slack, ticket, disabled}, repo.ToolSettings()).
		WithGate(tool.ProviderNotion, func(_ context.Context, _ types.WorkspaceID) (bool, error) {
			// Only enable notion when there is at least one Notion source.
			srcs, _ := repo.Source().ListByProvider(context.Background(), ws, types.SourceProviderNotion)
			return len(srcs) > 0, nil
		})

	t.Run("notion blocked by gate when no source", func(t *testing.T) {
		states, err := cat.States(context.Background(), ws)
		gt.NoError(t, err)
		byID := stateMap(states)
		gt.False(t, byID[tool.ProviderNotion].Enabled)
		gt.Equal(t, byID[tool.ProviderNotion].Reason, tool.ReasonGateBlocked)
		gt.True(t, byID[tool.ProviderSlack].Enabled)
		gt.True(t, byID[tool.ProviderTicket].Enabled)
		gt.False(t, byID["extra"].Enabled)
		gt.Equal(t, byID["extra"].Reason, tool.ReasonProviderUnavailable)
	})

	t.Run("workspace_disabled overrides default", func(t *testing.T) {
		gt.NoError(t, repo.ToolSettings().Set(context.Background(), ws, "slack", false))
		states, err := cat.States(context.Background(), ws)
		gt.NoError(t, err)
		byID := stateMap(states)
		gt.False(t, byID[tool.ProviderSlack].Enabled)
		gt.Equal(t, byID[tool.ProviderSlack].Reason, tool.ReasonWorkspaceDisabled)
	})

	t.Run("For returns only enabled tools", func(t *testing.T) {
		gt.NoError(t, repo.ToolSettings().Set(context.Background(), ws, "slack", true))
		tools, err := cat.For(context.Background(), ws)
		gt.NoError(t, err)
		// notion gated out, extra unavailable, slack+ticket enabled.
		gt.Equal(t, len(tools), 2)
		names := map[string]bool{}
		for _, tl := range tools {
			names[tl.Spec().Name] = true
		}
		gt.True(t, names["slack_only"])
		gt.True(t, names["ticket_only"])
	})
}

func stateMap(states []tool.State) map[tool.ProviderID]tool.State {
	out := make(map[tool.ProviderID]tool.State, len(states))
	for _, s := range states {
		out[s.ID] = s
	}
	return out
}
