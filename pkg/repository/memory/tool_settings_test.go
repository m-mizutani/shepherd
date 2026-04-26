package memory_test

import (
	"context"
	"testing"

	"github.com/m-mizutani/gt"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
	"github.com/m-mizutani/shepherd/pkg/repository/memory"
)

func TestToolSettings_GetReturnsEmptyByDefault(t *testing.T) {
	repo := memory.New().ToolSettings()
	got, err := repo.Get(context.Background(), types.WorkspaceID("ws-x"))
	gt.NoError(t, err)
	gt.Equal(t, got.WorkspaceID, types.WorkspaceID("ws-x"))
	gt.Equal(t, len(got.Enabled), 0)
}

func TestToolSettings_SetThenGet(t *testing.T) {
	repo := memory.New().ToolSettings()
	ctx := context.Background()
	ws := types.WorkspaceID("ws-1")

	gt.NoError(t, repo.Set(ctx, ws, "notion", true))
	gt.NoError(t, repo.Set(ctx, ws, "slack", false))

	got, err := repo.Get(ctx, ws)
	gt.NoError(t, err)
	gt.True(t, got.Enabled["notion"])
	gt.False(t, got.Enabled["slack"])

	// IsEnabled fallback applies factory default for unknown key
	gt.True(t, got.IsEnabled("missing", true))
	gt.False(t, got.IsEnabled("missing", false))
	gt.True(t, got.IsEnabled("notion", false))  // override wins
	gt.False(t, got.IsEnabled("slack", true))   // override wins
}
