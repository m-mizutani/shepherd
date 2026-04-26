// Package slack contains gollem.Tool implementations that read Slack data
// (search, thread, channel history, user info). The tools delegate API calls
// to a SlackTooler interface so tests can substitute a fake.
package slack

import (
	"context"

	"github.com/m-mizutani/gollem"
	slackService "github.com/m-mizutani/shepherd/pkg/service/slack"
	"github.com/m-mizutani/shepherd/pkg/tool"
	"github.com/urfave/cli/v3"
)

// SlackTooler is the subset of the Slack service used by the tools in this
// package. Production code passes *slackService.Client directly.
type SlackTooler interface {
	SearchMessages(ctx context.Context, query string, count int, sort string) ([]*slackService.SearchMatch, error)
	GetThreadMessages(ctx context.Context, channelID, threadTS string, limit int) ([]*slackService.Message, error)
	GetChannelHistory(ctx context.Context, channelID, oldest, latest string, limit int) ([]*slackService.Message, error)
	GetUserInfo(ctx context.Context, userID string) (*slackService.UserInfo, error)
}

// Factory implements tool.ToolFactory for Slack tools.
type Factory struct {
	svc   SlackTooler
	tools []gollem.Tool
}

// New constructs a Factory. svc may be nil — Available() then reports false.
func New(svc SlackTooler) *Factory { return &Factory{svc: svc} }

func (f *Factory) ID() tool.ProviderID { return tool.ProviderSlack }
func (f *Factory) Flags() []cli.Flag   { return nil }

func (f *Factory) Init(_ context.Context) error {
	if f.svc == nil {
		return nil
	}
	f.tools = []gollem.Tool{
		newSearchMessagesTool(f.svc),
		newGetThreadTool(f.svc),
		newGetChannelHistoryTool(f.svc),
		newGetUserInfoTool(f.svc),
	}
	return nil
}

func (f *Factory) Available() bool      { return f.svc != nil }
func (f *Factory) Tools() []gollem.Tool { return f.tools }
func (f *Factory) DefaultEnabled() bool { return true }
