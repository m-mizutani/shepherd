package notion

import (
	"context"

	"github.com/m-mizutani/gollem"
	notionsvc "github.com/m-mizutani/shepherd/pkg/service/notion"
	"github.com/m-mizutani/shepherd/pkg/usecase/source"
)

// notionAPI is the union of every method the in-package tools call. Tests
// pass a fake implementing this superset.
type notionAPI interface {
	Search(ctx context.Context, query string, opts notionsvc.SearchOptions) ([]*notionsvc.SearchHit, error)
	GetPageMarkdown(ctx context.Context, pageID string) (*notionsvc.PageMarkdown, error)
	RetrievePage(ctx context.Context, pageID string) (*notionsvc.PageMeta, error)
	QueryDatabase(ctx context.Context, dbID string, opts notionsvc.QueryDatabaseOptions) (*notionsvc.QueryDatabaseResult, error)
}

// BuildToolsForTest constructs the tool slice from arbitrary fakes. Used by
// notion_test.go so tests do not need to spin up an HTTP server.
func BuildToolsForTest(api notionAPI, guard *source.NotionGuard) []gollem.Tool {
	auth := &guardAdapter{g: guard}
	return []gollem.Tool{
		newSearchTool(api, auth),
		newGetPageTool(api, auth),
		newQueryDatabaseTool(api, auth),
	}
}

// SetTokenForTest binds the --notion-token value into the factory without
// going through urfave/cli. Used only by tests.
func SetTokenForTest(f *Factory, token string) { f.token = token }
