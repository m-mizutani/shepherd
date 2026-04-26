package source_test

import (
	"context"
	"errors"
	"testing"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/gt"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
	"github.com/m-mizutani/shepherd/pkg/repository/memory"
	"github.com/m-mizutani/shepherd/pkg/service/notion"
	"github.com/m-mizutani/shepherd/pkg/usecase/source"
)

type fakeNotion struct {
	pages     map[string]*notion.PageMeta
	databases map[string]*notion.DatabaseMeta
	pageErr   map[string]error
	dbErr     map[string]error
}

func (f *fakeNotion) RetrievePage(_ context.Context, id string) (*notion.PageMeta, error) {
	if e, ok := f.pageErr[id]; ok {
		return nil, e
	}
	if p, ok := f.pages[id]; ok {
		return p, nil
	}
	return nil, goerr.Wrap(notion.ErrNotFound, "page not found in fake")
}

func (f *fakeNotion) RetrieveDatabase(_ context.Context, id string) (*notion.DatabaseMeta, error) {
	if e, ok := f.dbErr[id]; ok {
		return nil, e
	}
	if d, ok := f.databases[id]; ok {
		return d, nil
	}
	return nil, goerr.Wrap(notion.ErrNotFound, "database not found in fake")
}

const (
	pageA      = "abcdef0123456789abcdef0123456789"
	pageB      = "11112222333344445555666677778888"
	pageOrphan = "99998888777766665555444433332222"
	dbA        = "aaaa1111bbbb2222cccc3333dddd4444"
)

func TestUseCase_VerifyAndCreate(t *testing.T) {
	ctx := context.Background()
	repo := memory.New().Source()
	fk := &fakeNotion{
		pages: map[string]*notion.PageMeta{
			pageA: {ID: pageA, Title: "Page A", URL: "https://www.notion.so/x/" + pageA},
		},
	}
	uc := source.New(repo, fk, nil)

	src, err := uc.CreateNotionSource(ctx, "ws-1", "https://www.notion.so/x/Page-A-"+pageA, "user1")
	gt.NoError(t, err)
	gt.Equal(t, src.Notion.Title, "Page A")
	gt.Equal(t, src.Notion.ObjectID, pageA)

	// Duplicate
	_, err = uc.CreateNotionSource(ctx, "ws-1", "https://www.notion.so/x/Page-A-"+pageA, "user1")
	gt.True(t, errors.Is(err, source.ErrDuplicate))
}

func TestUseCase_Verify_NotionForbidden(t *testing.T) {
	uc := source.New(memory.New().Source(), &fakeNotion{
		pageErr: map[string]error{pageA: goerr.Wrap(notion.ErrForbidden, "denied")},
	}, nil)
	_, err := uc.VerifyNotionTarget(context.Background(), "https://www.notion.so/x/"+pageA)
	gt.True(t, errors.Is(err, source.ErrNotionForbidden))
}

func TestUseCase_Verify_NotionNotFound(t *testing.T) {
	uc := source.New(memory.New().Source(), &fakeNotion{
		pageErr: map[string]error{pageA: goerr.Wrap(notion.ErrNotFound, "missing")},
	}, nil)
	_, err := uc.VerifyNotionTarget(context.Background(), "https://www.notion.so/x/"+pageA)
	gt.True(t, errors.Is(err, source.ErrNotionNotFound))
}

func TestUseCase_Verify_InvalidURL(t *testing.T) {
	uc := source.New(memory.New().Source(), &fakeNotion{}, nil)
	_, err := uc.VerifyNotionTarget(context.Background(), "https://example.com/foo")
	gt.True(t, errors.Is(err, source.ErrInvalidURL))
}

func TestGuard_DirectMatch(t *testing.T) {
	ctx := context.Background()
	repo := memory.New().Source()
	_, err := repo.Create(ctx, &model.Source{
		WorkspaceID: "ws-1",
		Provider:    types.SourceProviderNotion,
		Notion:      &model.NotionSource{ObjectType: types.NotionObjectPage, ObjectID: pageA},
	})
	gt.NoError(t, err)

	g := source.NewNotionGuard(repo, &fakeNotion{})
	gt.NoError(t, g.Authorize(ctx, "ws-1", types.NotionObjectPage, pageA))
}

func TestGuard_ParentWalkAllows(t *testing.T) {
	ctx := context.Background()
	repo := memory.New().Source()
	_, _ = repo.Create(ctx, &model.Source{
		WorkspaceID: "ws-1",
		Provider:    types.SourceProviderNotion,
		Notion:      &model.NotionSource{ObjectType: types.NotionObjectPage, ObjectID: pageA},
	})

	// pageB's parent is pageA → allowed via 1-hop walk.
	fk := &fakeNotion{
		pages: map[string]*notion.PageMeta{
			pageB: {ID: pageB, ParentType: "page_id", ParentID: pageA},
		},
	}
	g := source.NewNotionGuard(repo, fk)
	gt.NoError(t, g.Authorize(ctx, "ws-1", types.NotionObjectPage, pageB))
}

func TestGuard_OutOfScope(t *testing.T) {
	ctx := context.Background()
	repo := memory.New().Source()
	_, _ = repo.Create(ctx, &model.Source{
		WorkspaceID: "ws-1",
		Provider:    types.SourceProviderNotion,
		Notion:      &model.NotionSource{ObjectType: types.NotionObjectPage, ObjectID: pageA},
	})

	// pageOrphan has no parent — never reaches pageA.
	fk := &fakeNotion{
		pages: map[string]*notion.PageMeta{
			pageOrphan: {ID: pageOrphan},
		},
	}
	g := source.NewNotionGuard(repo, fk)
	err := g.Authorize(ctx, "ws-1", types.NotionObjectPage, pageOrphan)
	gt.True(t, errors.Is(err, source.ErrOutOfScope))
}

func TestGuard_DatabaseRoot(t *testing.T) {
	ctx := context.Background()
	repo := memory.New().Source()
	_, _ = repo.Create(ctx, &model.Source{
		WorkspaceID: "ws-1",
		Provider:    types.SourceProviderNotion,
		Notion:      &model.NotionSource{ObjectType: types.NotionObjectDatabase, ObjectID: dbA},
	})

	// pageB lives directly inside dbA.
	fk := &fakeNotion{
		pages: map[string]*notion.PageMeta{
			pageB: {ID: pageB, ParentType: "database_id", ParentID: dbA},
		},
		databases: map[string]*notion.DatabaseMeta{
			dbA: {ID: dbA},
		},
	}
	g := source.NewNotionGuard(repo, fk)
	gt.NoError(t, g.Authorize(ctx, "ws-1", types.NotionObjectPage, pageB))
}
