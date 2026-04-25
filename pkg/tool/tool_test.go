package tool_test

import (
	"context"
	"testing"

	"github.com/m-mizutani/gt"
	slackService "github.com/m-mizutani/shepherd/pkg/service/slack"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
	"github.com/m-mizutani/shepherd/pkg/repository/memory"
	"github.com/m-mizutani/shepherd/pkg/tool"
)

type stubSlack struct{}

func (stubSlack) SearchMessages(_ context.Context, _ string, _ int, _ string) ([]*slackService.SearchMatch, error) {
	return nil, nil
}
func (stubSlack) GetThreadMessages(_ context.Context, _, _ string, _ int) ([]*slackService.Message, error) {
	return nil, nil
}
func (stubSlack) GetChannelHistory(_ context.Context, _, _, _ string, _ int) ([]*slackService.Message, error) {
	return nil, nil
}
func (stubSlack) GetUserInfo(_ context.Context, _ string) (*slackService.UserInfo, error) {
	return nil, nil
}

func TestBuild(t *testing.T) {
	repo := memory.New()
	t.Cleanup(func() { _ = repo.Close() })

	tools := tool.Build(tool.Deps{
		Repo:     repo,
		Slack:    stubSlack{},
		Registry: model.NewWorkspaceRegistry(),
	})

	expected := map[string]bool{
		"slack_search_messages":     true,
		"slack_get_thread":          true,
		"slack_get_channel_history": true,
		"slack_get_user_info":       true,
		"ticket_search":             true,
		"ticket_get":                true,
		"ticket_get_comments":       true,
		"ticket_get_history":        true,
		"workspace_describe":        true,
		"current_time":              true,
	}
	gt.Equal(t, len(tools), len(expected))

	seen := map[string]bool{}
	for _, tl := range tools {
		spec := tl.Spec()
		gt.NoError(t, spec.Validate())
		gt.False(t, seen[spec.Name])
		seen[spec.Name] = true
		gt.True(t, expected[spec.Name])
	}
}
