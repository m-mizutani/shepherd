package notion_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/gt"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
	"github.com/m-mizutani/shepherd/pkg/repository/memory"
	notionsvc "github.com/m-mizutani/shepherd/pkg/service/notion"
	tnotion "github.com/m-mizutani/shepherd/pkg/tool/notion"
	"github.com/m-mizutani/shepherd/pkg/usecase/source"
)

const (
	wsID  = types.WorkspaceID("ws-1")
	root  = "abcdef0123456789abcdef0123456789"
	child = "11112222333344445555666677778888"
	out   = "99998888777766665555444433332222"
	dbID  = "aaaa1111bbbb2222cccc3333dddd4444"
)

type fakeNotion struct {
	pages    map[string]*notionsvc.PageMeta
	bodies   map[string]*notionsvc.PageMarkdown
	dbs      map[string]*notionsvc.DatabaseMeta
	dbQuery  *notionsvc.QueryDatabaseResult
	hits     []*notionsvc.SearchHit
	bodyErr  map[string]error
}

func (f *fakeNotion) RetrievePage(_ context.Context, id string) (*notionsvc.PageMeta, error) {
	if p, ok := f.pages[id]; ok {
		return p, nil
	}
	return nil, goerr.Wrap(notionsvc.ErrNotFound, "page not in fake")
}
func (f *fakeNotion) RetrieveDatabase(_ context.Context, id string) (*notionsvc.DatabaseMeta, error) {
	if d, ok := f.dbs[id]; ok {
		return d, nil
	}
	return nil, goerr.Wrap(notionsvc.ErrNotFound, "db not in fake")
}
func (f *fakeNotion) GetPageMarkdown(_ context.Context, id string) (*notionsvc.PageMarkdown, error) {
	if e, ok := f.bodyErr[id]; ok {
		return nil, e
	}
	if b, ok := f.bodies[id]; ok {
		return b, nil
	}
	return nil, goerr.Wrap(notionsvc.ErrNotFound, "body not in fake")
}
func (f *fakeNotion) Search(_ context.Context, _ string, _ notionsvc.SearchOptions) ([]*notionsvc.SearchHit, error) {
	return f.hits, nil
}
func (f *fakeNotion) QueryDatabase(_ context.Context, _ string, _ notionsvc.QueryDatabaseOptions) (*notionsvc.QueryDatabaseResult, error) {
	return f.dbQuery, nil
}

// fakeAuthorizer wraps a real NotionGuard built on top of fakeNotion +
// memory source repo, so tests verify Guard logic too.
func newSetup(t *testing.T, fk *fakeNotion, roots ...rootSpec) (*memory.Repository, *source.NotionGuard) {
	t.Helper()
	repo := memory.New()
	t.Cleanup(func() { _ = repo.Close() })
	for _, r := range roots {
		_, err := repo.Source().Create(context.Background(), &model.Source{
			WorkspaceID: wsID,
			Provider:    types.SourceProviderNotion,
			Notion:      &model.NotionSource{ObjectType: r.t, ObjectID: r.id},
		})
		gt.NoError(t, err)
	}
	guard := source.NewNotionGuard(repo.Source(), fk)
	return repo, guard
}

type rootSpec struct {
	t  types.NotionObjectType
	id string
}

func runTool(t *testing.T, tools []gollem.Tool, name string, args map[string]any) (map[string]any, error) {
	t.Helper()
	ctx := types.ContextWithWorkspace(context.Background(), wsID)
	for _, tl := range tools {
		if tl.Spec().Name == name {
			return tl.Run(ctx, args)
		}
	}
	t.Fatalf("tool %q not found", name)
	return nil, nil
}

// buildTools mirrors what Factory.Init does, using interfaces directly so we
// can pass our fake without an HTTP client.
func TestSearch_FiltersToAllowedRoots(t *testing.T) {
	fk := &fakeNotion{
		hits: []*notionsvc.SearchHit{
			{ID: root, ObjectType: types.NotionObjectPage, Title: "in"},
			{ID: out, ObjectType: types.NotionObjectPage, Title: "out"},
		},
		pages: map[string]*notionsvc.PageMeta{
			out: {ID: out}, // no parent → out of scope
		},
	}
	_, guard := newSetup(t, fk, rootSpec{types.NotionObjectPage, root})
	tools := tnotion.BuildToolsForTest(fk, guard)

	res, err := runTool(t, tools, "notion_search", map[string]any{"query": "x"})
	gt.NoError(t, err)
	matches := res["matches"].([]map[string]any)
	gt.Equal(t, len(matches), 1)
	gt.Equal(t, matches[0]["id"], root)
}

func TestGetPage_AuthorizeDeniesOutOfScope(t *testing.T) {
	fk := &fakeNotion{
		pages: map[string]*notionsvc.PageMeta{
			out: {ID: out},
		},
	}
	_, guard := newSetup(t, fk, rootSpec{types.NotionObjectPage, root})
	tools := tnotion.BuildToolsForTest(fk, guard)

	_, err := runTool(t, tools, "notion_get_page", map[string]any{"page_id": out})
	gt.Error(t, err)
	gt.True(t, errors.Is(err, source.ErrOutOfScope))
}

func TestGetPage_RecursiveScopedAndCapped(t *testing.T) {
	fk := &fakeNotion{
		pages: map[string]*notionsvc.PageMeta{
			root: {ID: root, Title: "root"},
			child: {ID: child, ParentType: "page_id", ParentID: root},
		},
		bodies: map[string]*notionsvc.PageMarkdown{
			root:  {Markdown: "ROOT-BODY", ChildPageIDs: []string{child, out}},
			child: {Markdown: "CHILD-BODY"},
		},
	}
	_, guard := newSetup(t, fk, rootSpec{types.NotionObjectPage, root})
	tools := tnotion.BuildToolsForTest(fk, guard)

	res, err := runTool(t, tools, "notion_get_page", map[string]any{
		"page_id":   root,
		"recursive": true,
	})
	gt.NoError(t, err)
	gt.True(t, strings.Contains(res["markdown"].(string), "ROOT-BODY"))
	gt.True(t, strings.Contains(res["markdown"].(string), "CHILD-BODY"))
	skipped := res["skipped"].([]map[string]any)
	gt.Equal(t, len(skipped), 1) // out-of-scope child appears here
	gt.Equal(t, skipped[0]["id"], out)
	gt.Equal(t, skipped[0]["reason"], "out_of_scope")
}

func TestGetPage_MaxPagesTruncates(t *testing.T) {
	// max_pages=1 → after fetching root, queue has child but loop hits cap.
	fk := &fakeNotion{
		pages: map[string]*notionsvc.PageMeta{
			root:  {ID: root, Title: "root"},
			child: {ID: child, ParentType: "page_id", ParentID: root},
		},
		bodies: map[string]*notionsvc.PageMarkdown{
			root:  {Markdown: "R", ChildPageIDs: []string{child}},
			child: {Markdown: "C"},
		},
	}
	_, guard := newSetup(t, fk, rootSpec{types.NotionObjectPage, root})
	tools := tnotion.BuildToolsForTest(fk, guard)

	res, err := runTool(t, tools, "notion_get_page", map[string]any{
		"page_id":   root,
		"recursive": true,
		"max_pages": 1,
	})
	gt.NoError(t, err)
	gt.True(t, res["truncated"].(bool))
	fetched := res["fetched"].([]string)
	gt.Equal(t, len(fetched), 1)
}

func TestQueryDatabase_RequiresAllowedRoot(t *testing.T) {
	fk := &fakeNotion{
		dbs:     map[string]*notionsvc.DatabaseMeta{dbID: {ID: dbID}},
		dbQuery: &notionsvc.QueryDatabaseResult{Pages: []*notionsvc.PageMeta{{ID: root, Title: "row"}}},
		bodies: map[string]*notionsvc.PageMarkdown{
			root: {Markdown: "row body"},
		},
	}
	_, guard := newSetup(t, fk, rootSpec{types.NotionObjectDatabase, dbID})
	tools := tnotion.BuildToolsForTest(fk, guard)

	res, err := runTool(t, tools, "notion_query_database", map[string]any{
		"database_id":  dbID,
		"include_body": true,
	})
	gt.NoError(t, err)
	rows := res["rows"].([]map[string]any)
	gt.Equal(t, len(rows), 1)
	gt.Equal(t, rows[0]["markdown"], "row body")
}

func TestQueryDatabase_DeniesUnregistered(t *testing.T) {
	fk := &fakeNotion{
		dbs: map[string]*notionsvc.DatabaseMeta{dbID: {ID: dbID}},
	}
	_, guard := newSetup(t, fk /* no roots */)
	tools := tnotion.BuildToolsForTest(fk, guard)

	_, err := runTool(t, tools, "notion_query_database", map[string]any{
		"database_id": dbID,
	})
	gt.Error(t, err)
}

func TestFactory_AvailableFollowsToken(t *testing.T) {
	repo := memory.New()
	t.Cleanup(func() { _ = repo.Close() })

	t.Run("no token leaves factory inert", func(t *testing.T) {
		f := tnotion.New(repo.Source(), http.DefaultClient)
		gt.NoError(t, f.Init(context.Background()))
		gt.False(t, f.Available())
		gt.Equal(t, len(f.Tools()), 0)
		gt.Nil(t, f.Client())
	})

	t.Run("token enables tools", func(t *testing.T) {
		f := tnotion.New(repo.Source(), http.DefaultClient)
		// Bind the token field that --notion-token would normally fill.
		tnotion.SetTokenForTest(f, "secret_xx")
		gt.NoError(t, f.Init(context.Background()))
		gt.True(t, f.Available())
		gt.Equal(t, len(f.Tools()), 4) // search / get_page / query_database / list_sources
		gt.NotNil(t, f.Client())
	})
}
