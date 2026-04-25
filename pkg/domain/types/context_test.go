package types_test

import (
	"context"
	"testing"

	"github.com/m-mizutani/gt"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
)

func TestWorkspaceContext(t *testing.T) {
	t.Run("unbound returns false", func(t *testing.T) {
		_, ok := types.WorkspaceFromContext(context.Background())
		gt.False(t, ok)
	})

	t.Run("bound value round-trips", func(t *testing.T) {
		ctx := types.ContextWithWorkspace(context.Background(), types.WorkspaceID("ws-1"))
		got, ok := types.WorkspaceFromContext(ctx)
		gt.True(t, ok)
		gt.Equal(t, got, types.WorkspaceID("ws-1"))
	})
}
