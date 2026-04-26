package tool_test

import (
	"context"
	"testing"

	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/gt"
	"github.com/m-mizutani/shepherd/pkg/tool"
)

type filterStubTool struct{ name string }

func (s *filterStubTool) Spec() gollem.ToolSpec {
	return gollem.ToolSpec{Name: s.name, Description: "stub", Parameters: nil}
}
func (s *filterStubTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	return nil, nil
}

func TestFilterByName(t *testing.T) {
	tools := []gollem.Tool{
		&filterStubTool{name: "alpha"},
		&filterStubTool{name: "beta"},
		&filterStubTool{name: "gamma"},
	}

	t.Run("subset", func(t *testing.T) {
		got := tool.FilterByName(tools, []string{"beta", "gamma"})
		gt.N(t, len(got)).Equal(2)
		gt.S(t, got[0].Spec().Name).Equal("beta")
		gt.S(t, got[1].Spec().Name).Equal("gamma")
	})

	t.Run("preserves tool order, ignores name order", func(t *testing.T) {
		got := tool.FilterByName(tools, []string{"gamma", "alpha"})
		gt.N(t, len(got)).Equal(2)
		gt.S(t, got[0].Spec().Name).Equal("alpha")
		gt.S(t, got[1].Spec().Name).Equal("gamma")
	})

	t.Run("unknown names are ignored", func(t *testing.T) {
		got := tool.FilterByName(tools, []string{"alpha", "missing"})
		gt.N(t, len(got)).Equal(1)
		gt.S(t, got[0].Spec().Name).Equal("alpha")
	})

	t.Run("empty inputs", func(t *testing.T) {
		gt.N(t, len(tool.FilterByName(nil, []string{"x"}))).Equal(0)
		gt.N(t, len(tool.FilterByName(tools, nil))).Equal(0)
	})
}
