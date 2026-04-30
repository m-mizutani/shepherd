package slack_test

import (
	"context"
	"testing"

	"github.com/m-mizutani/gt"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
	tslack "github.com/m-mizutani/shepherd/pkg/tool/slack"
)

func TestFactory_Prompt(t *testing.T) {
	// renderPrompt is workspace-agnostic, so passing nil svc + empty ws is fine.
	f := tslack.New(nil)
	got, err := f.Prompt(context.Background(), types.WorkspaceID(""))
	gt.NoError(t, err)
	gt.S(t, got).Contains("slack_search_messages")
	gt.S(t, got).Contains("slack_get_thread")
	gt.S(t, got).Contains("slack_get_channel_history")
	gt.S(t, got).Contains("slack_get_user_info")
	gt.S(t, got).Contains("from:@user")
}
