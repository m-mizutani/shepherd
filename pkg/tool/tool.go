// Package tool builds the catalog of gollem.Tool implementations exposed to
// the LLM. Each subdirectory groups related tools (slack, ticket, meta) and
// returns gollem.Tool values that this package aggregates via Build.
package tool

import (
	"context"
	"time"

	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/shepherd/pkg/domain/interfaces"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
	slackService "github.com/m-mizutani/shepherd/pkg/service/slack"
	"github.com/m-mizutani/shepherd/pkg/tool/meta"
	"github.com/m-mizutani/shepherd/pkg/tool/slack"
	"github.com/m-mizutani/shepherd/pkg/tool/ticket"
)

// SlackTooler is the subset of the Slack service needed by tools. Mirrors the
// pattern in pkg/usecase/slack.go: declare only what we use so tests can
// substitute a fake.
type SlackTooler interface {
	SearchMessages(ctx context.Context, query string, count int, sort string) ([]*slackService.SearchMatch, error)
	GetThreadMessages(ctx context.Context, channelID, threadTS string, limit int) ([]*slackService.Message, error)
	GetChannelHistory(ctx context.Context, channelID, oldest, latest string, limit int) ([]*slackService.Message, error)
	GetUserInfo(ctx context.Context, userID string) (*slackService.UserInfo, error)
}

// Deps bundles the dependencies tools need.
type Deps struct {
	Repo     interfaces.Repository
	Slack    SlackTooler
	Registry *model.WorkspaceRegistry
	Now      func() time.Time
}

// Build returns the full catalog of tools assembled from the subpackages.
func Build(d Deps) []gollem.Tool {
	now := d.Now
	if now == nil {
		now = time.Now
	}

	tools := make([]gollem.Tool, 0, 12)
	tools = append(tools, slack.Tools(slack.Deps{Slack: d.Slack})...)
	tools = append(tools, ticket.Tools(ticket.Deps{Repo: d.Repo})...)
	tools = append(tools, meta.Tools(meta.Deps{Registry: d.Registry, Now: now})...)
	return tools
}
