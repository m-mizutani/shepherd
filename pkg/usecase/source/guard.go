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

// Authorize returns nil when the object is reachable from a registered Source.
// Walks parents up to maxParentWalk hops via the Notion API.
func (g *NotionGuard) Authorize(ctx context.Context, ws types.WorkspaceID, objectType types.NotionObjectType, objectID string) error {
	id, err := notion.NormalizeID(objectID)
	if err != nil {
		return goerr.Wrap(err, "invalid notion id")
	}
	pageRoots, dbRoots, err := g.AllowedRoots(ctx, ws)
	if err != nil {
		return err
	}
	rootSet := make(map[string]struct{}, len(pageRoots)+len(dbRoots))
	for _, p := range pageRoots {
		rootSet[p] = struct{}{}
	}
	for _, d := range dbRoots {
		rootSet[d] = struct{}{}
	}

	currentType := objectType
	currentID := id
	for range maxParentWalk {
		if _, ok := rootSet[currentID]; ok {
			return nil
		}
		parentType, parentID, err := g.parentOf(ctx, currentType, currentID)
		if err != nil {
			return err
		}
		if parentID == "" {
			break
		}
		currentType = parentType
		currentID = parentID
	}
	return goerr.Wrap(ErrOutOfScope, "notion object out of allowed sources",
		goerr.V("workspace_id", string(ws)),
		goerr.V("object_id", objectID))
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
