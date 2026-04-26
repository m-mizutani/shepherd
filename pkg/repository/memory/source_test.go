package memory_test

import (
	"context"
	"testing"

	"github.com/m-mizutani/gt"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
	"github.com/m-mizutani/shepherd/pkg/repository/memory"
)

func TestSourceRepo_CRUD(t *testing.T) {
	repo := memory.New().Source()
	ctx := context.Background()
	ws := types.WorkspaceID("ws-1")

	created, err := repo.Create(ctx, &model.Source{
		WorkspaceID: ws,
		Provider:    types.SourceProviderNotion,
		Notion: &model.NotionSource{
			ObjectType: types.NotionObjectPage,
			ObjectID:   "abc",
			URL:        "https://www.notion.so/abc",
			Title:      "Demo",
		},
	})
	gt.NoError(t, err)
	gt.NotEqual(t, created.ID, types.SourceID(""))

	got, err := repo.Get(ctx, ws, created.ID)
	gt.NoError(t, err)
	gt.Equal(t, got.Notion.Title, "Demo")

	all, err := repo.List(ctx, ws)
	gt.NoError(t, err)
	gt.Equal(t, len(all), 1)

	notion, err := repo.ListByProvider(ctx, ws, types.SourceProviderNotion)
	gt.NoError(t, err)
	gt.Equal(t, len(notion), 1)

	none, err := repo.ListByProvider(ctx, ws, types.SourceProvider("nope"))
	gt.NoError(t, err)
	gt.Equal(t, len(none), 0)

	gt.NoError(t, repo.Delete(ctx, ws, created.ID))
	_, err = repo.Get(ctx, ws, created.ID)
	gt.Error(t, err)
}

func TestSourceRepo_IsolatedByWorkspace(t *testing.T) {
	repo := memory.New().Source()
	ctx := context.Background()

	a, err := repo.Create(ctx, &model.Source{WorkspaceID: "ws-a", Provider: types.SourceProviderNotion})
	gt.NoError(t, err)
	_, err = repo.Create(ctx, &model.Source{WorkspaceID: "ws-b", Provider: types.SourceProviderNotion})
	gt.NoError(t, err)

	listA, _ := repo.List(ctx, "ws-a")
	listB, _ := repo.List(ctx, "ws-b")
	gt.Equal(t, len(listA), 1)
	gt.Equal(t, len(listB), 1)
	gt.Equal(t, listA[0].ID, a.ID)
}
