package meta

import (
	"context"
	"time"

	"github.com/m-mizutani/gollem"
)

type currentTimeTool struct {
	now func() time.Time
}

func newCurrentTimeTool(now func() time.Time) gollem.Tool {
	return &currentTimeTool{now: now}
}

func (t *currentTimeTool) Spec() gollem.ToolSpec {
	return gollem.ToolSpec{
		Name:        "current_time",
		Description: "Return the current server time. Useful for resolving relative date phrases like 'last week' before issuing other tool calls.",
		Parameters:  map[string]*gollem.Parameter{},
	}
}

func (t *currentTimeTool) Run(_ context.Context, _ map[string]any) (map[string]any, error) {
	now := t.now().UTC()
	return map[string]any{
		"rfc3339": now.Format(time.RFC3339),
		"unix":    now.Unix(),
	}, nil
}
