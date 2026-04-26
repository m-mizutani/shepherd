package notion

import (
	"context"

	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
	notionsvc "github.com/m-mizutani/shepherd/pkg/service/notion"
)

// notionAPI is the union of every method the in-package tools call. Tests
// pass a fake implementing this superset.
type notionAPI interface {
	Search(ctx context.Context, query string, opts notionsvc.SearchOptions) ([]*notionsvc.SearchHit, error)
	GetPageMarkdown(ctx context.Context, pageID string) (*notionsvc.PageMarkdown, error)
	RetrievePage(ctx context.Context, pageID string) (*notionsvc.PageMeta, error)
	QueryDatabase(ctx context.Context, dbID string, opts notionsvc.QueryDatabaseOptions) (*notionsvc.QueryDatabaseResult, error)
}

type guardLike interface {
	Authorize(ctx context.Context, ws types.WorkspaceID, t types.NotionObjectType, id string) error
}

// BuildToolsForTest constructs the tool slice from arbitrary fakes. Used by
// notion_test.go so tests do not need to spin up an HTTP server.
func BuildToolsForTest(api notionAPI, guard guardLike) []gollem.Tool {
	return []gollem.Tool{
		newSearchTool(api, guard),
		newGetPageTool(api, guard),
		newQueryDatabaseTool(api, guard),
	}
}

// SetTokenForTest binds the --notion-token value into the factory without
// going through urfave/cli. Used only by tests.
func SetTokenForTest(f *Factory, token string) { f.token = token }
