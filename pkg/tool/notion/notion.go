// Package notion exposes gollem.Tool implementations for searching and
// reading Notion content, scoped to the workspace's registered Sources via a
// NotionGuard.
package notion

import (
	"context"
	"net/http"

	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/shepherd/pkg/domain/interfaces"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
	notionsvc "github.com/m-mizutani/shepherd/pkg/service/notion"
	"github.com/m-mizutani/shepherd/pkg/tool"
	"github.com/m-mizutani/shepherd/pkg/usecase/source"
	"github.com/urfave/cli/v3"
)

// Factory implements tool.ToolFactory for Notion tools. Construct with the
// repository and HTTP client only — the Notion API token is bound via Flags()
// and consumed in Init().
type Factory struct {
	sourceRepo interfaces.SourceRepository
	httpClient *http.Client

	token string

	client *notionsvc.Client
	guard  *source.NotionGuard
	tools  []gollem.Tool
}

func New(sourceRepo interfaces.SourceRepository, httpClient *http.Client) *Factory {
	return &Factory{sourceRepo: sourceRepo, httpClient: httpClient}
}

// SetDeps lets the CLI layer inject repo-derived dependencies after flags are
// collected but before Init runs. This is the rare case where a factory must
// be constructed before its dependencies exist (because Flags() needs to
// participate in cli.Command construction). Call once, before Init.
func (f *Factory) SetDeps(sourceRepo interfaces.SourceRepository, httpClient *http.Client) {
	f.sourceRepo = sourceRepo
	f.httpClient = httpClient
}

func (f *Factory) ID() tool.ProviderID { return tool.ProviderNotion }

func (f *Factory) Flags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:        "notion-token",
			Usage:       "Notion Internal Integration Secret. If unset, Notion tools are disabled.",
			Category:    "Notion",
			Sources:     cli.EnvVars("SHEPHERD_NOTION_TOKEN"),
			Destination: &f.token,
		},
	}
}

func (f *Factory) Init(_ context.Context) error {
	if f.token == "" {
		return nil
	}
	f.client = notionsvc.New(f.token, f.httpClient)
	f.guard = source.NewNotionGuard(f.sourceRepo, f.client)
	f.tools = append(buildTools(f.client, f.guard), newListSourcesTool(f.sourceRepo))
	return nil
}

func (f *Factory) Available() bool      { return f.client != nil }
func (f *Factory) Tools() []gollem.Tool { return f.tools }
func (f *Factory) DefaultEnabled() bool { return false }

// Prompt returns workspace-aware narrative for the Notion provider. The
// real implementation is provided in prompt.go; this signature satisfies
// tool.ToolFactory.
func (f *Factory) Prompt(ctx context.Context, ws types.WorkspaceID) (string, error) {
	return f.renderPrompt(ctx, ws)
}

// Client exposes the inner *notion.Client so the HTTP API layer can reuse it
// for Source verification. Returns nil when Init has not been called or token
// was unset.
func (f *Factory) Client() *notionsvc.Client { return f.client }

func buildTools(client *notionsvc.Client, guard *source.NotionGuard) []gollem.Tool {
	return buildToolsWithAuthorizer(client, &guardAdapter{g: guard})
}

func buildToolsWithAuthorizer(client *notionsvc.Client, auth authorizer) []gollem.Tool {
	return []gollem.Tool{
		newSearchTool(client, auth),
		newGetPageTool(client, auth),
		newQueryDatabaseTool(client, auth),
	}
}

// guardAdapter narrows *source.NotionGuard to the local authorizer interface.
// Necessary because Go interfaces are not covariant on return types — the
// concrete NewWalker returns *source.Walker, but tool consumers want the
// walkAuthorizer interface so test fakes can stand in.
type guardAdapter struct{ g *source.NotionGuard }

func (a *guardAdapter) NewWalker(ctx context.Context, ws types.WorkspaceID) (walkAuthorizer, error) {
	return a.g.NewWalker(ctx, ws)
}
