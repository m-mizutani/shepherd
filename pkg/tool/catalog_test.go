package tool_test

import (
	"context"
	"errors"
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
	prompt    string
	promptErr error
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
func (f *stubFactory) Prompt(context.Context, types.WorkspaceID) (string, error) {
	return f.prompt, f.promptErr
}

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

func TestCatalog_ToolBriefing(t *testing.T) {
	ws := types.WorkspaceID("ws-briefing")
	repo := memory.New()
	t.Cleanup(func() { _ = repo.Close() })

	slack := &stubFactory{
		id:        tool.ProviderSlack,
		available: true,
		def:       true,
		tools: []gollem.Tool{
			describedTool{name: "slack_search_messages", desc: "Search Slack messages."},
			describedTool{name: "slack_get_thread", desc: "Read a Slack thread."},
		},
		prompt: "slack provider narrative",
	}
	ticket := &stubFactory{
		id:        tool.ProviderTicket,
		available: true,
		def:       true,
		tools:     []gollem.Tool{describedTool{name: "ticket_get", desc: "Fetch a ticket."}},
		prompt:    "ticket provider narrative",
	}
	notion := &stubFactory{
		id:        tool.ProviderNotion,
		available: true,
		def:       false, // not enabled by default → won't show up unless toggled on
		tools:     []gollem.Tool{describedTool{name: "notion_search", desc: "Search Notion."}},
		prompt:    "notion provider narrative",
	}
	flaky := &stubFactory{
		id:        tool.ProviderID("flaky"),
		available: true,
		def:       true,
		tools:     []gollem.Tool{describedTool{name: "flaky_op", desc: "Flaky tool."}},
		promptErr: errors.New("source repo unavailable"),
	}

	for _, f := range []tool.ToolFactory{slack, ticket, notion, flaky} {
		gt.NoError(t, f.Init(context.Background()))
	}

	cat := tool.NewCatalog(
		[]tool.ToolFactory{slack, ticket, notion, flaky},
		repo.ToolSettings(),
	)

	t.Run("returns only enabled providers in registration order", func(t *testing.T) {
		got, err := cat.ToolBriefing(context.Background(), ws)
		gt.NoError(t, err)
		// notion is not DefaultEnabled and never toggled on, so it must be absent.
		gt.Equal(t, len(got), 3)
		gt.Equal(t, got[0].ID, tool.ProviderSlack)
		gt.Equal(t, got[1].ID, tool.ProviderTicket)
		gt.Equal(t, got[2].ID, tool.ProviderID("flaky"))

		// slack briefing carries narrative + tool entries.
		gt.Equal(t, got[0].Description, "slack provider narrative")
		gt.Equal(t, len(got[0].Tools), 2)
		gt.Equal(t, got[0].Tools[0].Name, "slack_search_messages")
		gt.Equal(t, got[0].Tools[0].Description, "Search Slack messages.")
		gt.Equal(t, got[0].Tools[1].Name, "slack_get_thread")
	})

	t.Run("provider Prompt error blanks Description but keeps Tools", func(t *testing.T) {
		got, err := cat.ToolBriefing(context.Background(), ws)
		gt.NoError(t, err)
		flakyEntry := got[2]
		gt.Equal(t, flakyEntry.ID, tool.ProviderID("flaky"))
		gt.Equal(t, flakyEntry.Description, "")
		gt.Equal(t, len(flakyEntry.Tools), 1)
		gt.Equal(t, flakyEntry.Tools[0].Name, "flaky_op")
	})

	t.Run("toggling notion on surfaces it in the briefing", func(t *testing.T) {
		gt.NoError(t, repo.ToolSettings().Set(context.Background(), ws, "notion", true))
		got, err := cat.ToolBriefing(context.Background(), ws)
		gt.NoError(t, err)
		gt.Equal(t, len(got), 4)
		gt.Equal(t, got[2].ID, tool.ProviderNotion)
		gt.Equal(t, got[2].Description, "notion provider narrative")
		gt.Equal(t, len(got[2].Tools), 1)
		gt.Equal(t, got[2].Tools[0].Name, "notion_search")
	})

	t.Run("disabling everything yields empty briefing", func(t *testing.T) {
		gt.NoError(t, repo.ToolSettings().Set(context.Background(), ws, "slack", false))
		gt.NoError(t, repo.ToolSettings().Set(context.Background(), ws, "ticket", false))
		gt.NoError(t, repo.ToolSettings().Set(context.Background(), ws, "notion", false))
		gt.NoError(t, repo.ToolSettings().Set(context.Background(), ws, "flaky", false))
		got, err := cat.ToolBriefing(context.Background(), ws)
		gt.NoError(t, err)
		gt.Equal(t, len(got), 0)
	})
}

type describedTool struct {
	name string
	desc string
}

func (d describedTool) Spec() gollem.ToolSpec {
	return gollem.ToolSpec{Name: d.name, Description: d.desc}
}
func (d describedTool) Run(context.Context, map[string]any) (map[string]any, error) {
	return map[string]any{}, nil
}
