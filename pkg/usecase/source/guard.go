package source

import (
	"context"
	"errors"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/shepherd/pkg/domain/interfaces"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
	"github.com/m-mizutani/shepherd/pkg/service/notion"
)

// ErrOutOfScope signals that a Notion object is not reachable from any
// registered Source for the workspace. Returned by NotionGuard.Authorize.
var ErrOutOfScope = errors.New("notion: object is outside this workspace's allowed sources")

// maxParentWalk caps the depth we'll climb the Notion parent chain looking for
// a registered Source. Notion nests page-in-page rarely beyond a few hops; we
// keep this small to bound API usage.
const maxParentWalk = 6

// NotionGuard answers "is this Notion object inside one of the workspace's
// registered roots?" It uses the SourceRepository for the allowed root list
// and the Notion API to walk parent links when no direct match exists.
type NotionGuard struct {
	repo  interfaces.SourceRepository
	pages NotionParentResolver
}

// NotionParentResolver is the subset of the Notion client used to walk parents.
type NotionParentResolver interface {
	RetrievePage(ctx context.Context, pageID string) (*notion.PageMeta, error)
	RetrieveDatabase(ctx context.Context, dbID string) (*notion.DatabaseMeta, error)
}

func NewNotionGuard(repo interfaces.SourceRepository, resolver NotionParentResolver) *NotionGuard {
	return &NotionGuard{repo: repo, pages: resolver}
}

// AllowedRoots returns the page and database IDs registered as Source for ws.
func (g *NotionGuard) AllowedRoots(ctx context.Context, ws types.WorkspaceID) (pageIDs, dbIDs []string, err error) {
	srcs, err := g.repo.ListByProvider(ctx, ws, types.SourceProviderNotion)
	if err != nil {
		return nil, nil, goerr.Wrap(err, "failed to list notion sources")
	}
	for _, s := range srcs {
		if s.Notion == nil {
			continue
		}
		switch s.Notion.ObjectType {
		case types.NotionObjectPage:
			pageIDs = append(pageIDs, s.Notion.ObjectID)
		case types.NotionObjectDatabase:
			dbIDs = append(dbIDs, s.Notion.ObjectID)
		}
	}
	return pageIDs, dbIDs, nil
}

// Authorize is a one-shot convenience wrapper around NewWalker → Authorize.
// Tools that issue many checks in a row (notion_search, notion_get_page's
// recursion, notion_query_database's row checks) MUST use NewWalker instead so
// AllowedRoots is fetched once and parent-walk results are memoized within the
// call's lifetime.
func (g *NotionGuard) Authorize(ctx context.Context, ws types.WorkspaceID, objectType types.NotionObjectType, objectID string) error {
	w, err := g.NewWalker(ctx, ws)
	if err != nil {
		return err
	}
	return w.Authorize(ctx, objectType, objectID)
}

// Walker is a request-scoped Authorize cache. Holds the workspace's allowed
// roots (one repo read at construction) plus a memo of parent-walk decisions
// for the duration of a single tool call. Not safe for concurrent use across
// goroutines.
type Walker struct {
	g       *NotionGuard
	ws      types.WorkspaceID
	rootSet map[string]struct{}
	// parent[id] = canonical parent (type+id) discovered during this walk.
	// Lets a search batch share parent-chain work across hits.
	parent map[string]parentEntry
	// decided[id] = "is this id in scope?" cached terminal result.
	decided map[string]bool
}

type parentEntry struct {
	t  types.NotionObjectType
	id string
}

// NewWalker fetches AllowedRoots once for the workspace and returns a Walker
// whose Authorize calls share that root set + a parent-walk memo.
func (g *NotionGuard) NewWalker(ctx context.Context, ws types.WorkspaceID) (*Walker, error) {
	pageRoots, dbRoots, err := g.AllowedRoots(ctx, ws)
	if err != nil {
		return nil, err
	}
	rs := make(map[string]struct{}, len(pageRoots)+len(dbRoots))
	for _, p := range pageRoots {
		rs[p] = struct{}{}
	}
	for _, d := range dbRoots {
		rs[d] = struct{}{}
	}
	return &Walker{
		g:       g,
		ws:      ws,
		rootSet: rs,
		parent:  map[string]parentEntry{},
		decided: map[string]bool{},
	}, nil
}

// Authorize returns nil when the object is reachable from a registered Source
// for the walker's workspace. Walks parents up to maxParentWalk hops, sharing
// memoized lookups across calls on this walker.
func (w *Walker) Authorize(ctx context.Context, objectType types.NotionObjectType, objectID string) error {
	id, err := notion.NormalizeID(objectID)
	if err != nil {
		return goerr.Wrap(err, "invalid notion id")
	}
	if ok, cached := w.decided[id]; cached {
		if ok {
			return nil
		}
		return goerr.Wrap(ErrOutOfScope, "notion object out of allowed sources",
			goerr.V("workspace_id", string(w.ws)),
			goerr.V("object_id", objectID))
	}

	visited := []string{id}
	currentType := objectType
	currentID := id
	for range maxParentWalk {
		if _, ok := w.rootSet[currentID]; ok {
			for _, v := range visited {
				w.decided[v] = true
			}
			return nil
		}
		parentType, parentID, err := w.parentLookup(ctx, currentType, currentID)
		if err != nil {
			return err
		}
		if parentID == "" {
			break
		}
		visited = append(visited, parentID)
		currentType = parentType
		currentID = parentID
	}

	for _, v := range visited {
		w.decided[v] = false
	}
	return goerr.Wrap(ErrOutOfScope, "notion object out of allowed sources",
		goerr.V("workspace_id", string(w.ws)),
		goerr.V("object_id", objectID))
}

// parentLookup memoizes parentOf for the lifetime of the walker — so a sweep
// over many search hits sharing a common ancestor incurs at most one
// RetrievePage per ancestor.
func (w *Walker) parentLookup(ctx context.Context, t types.NotionObjectType, id string) (types.NotionObjectType, string, error) {
	if pe, ok := w.parent[id]; ok {
		return pe.t, pe.id, nil
	}
	pt, pid, err := w.g.parentOf(ctx, t, id)
	if err != nil {
		return "", "", err
	}
	w.parent[id] = parentEntry{t: pt, id: pid}
	return pt, pid, nil
}

func (g *NotionGuard) parentOf(ctx context.Context, t types.NotionObjectType, id string) (types.NotionObjectType, string, error) {
	switch t {
	case types.NotionObjectPage:
		pm, err := g.pages.RetrievePage(ctx, id)
		if err != nil {
			return "", "", goerr.Wrap(err, "failed to retrieve page for parent walk")
		}
		return mapParentType(pm.ParentType), normalizeOrEmpty(pm.ParentID), nil
	case types.NotionObjectDatabase:
		dm, err := g.pages.RetrieveDatabase(ctx, id)
		if err != nil {
			return "", "", goerr.Wrap(err, "failed to retrieve database for parent walk")
		}
		return mapParentType(dm.ParentType), normalizeOrEmpty(dm.ParentID), nil
	}
	return "", "", nil
}

func mapParentType(s string) types.NotionObjectType {
	switch s {
	case "page_id":
		return types.NotionObjectPage
	case "database_id":
		return types.NotionObjectDatabase
	}
	return ""
}

func normalizeOrEmpty(s string) string {
	if s == "" {
		return ""
	}
	if id, err := notion.NormalizeID(s); err == nil {
		return id
	}
	return ""
}
