package slack_test

import (
	"context"
	"testing"

	"github.com/m-mizutani/gt"
	"github.com/m-mizutani/shepherd/pkg/domain/model/config"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
	slackService "github.com/m-mizutani/shepherd/pkg/service/slack"
	slackgo "github.com/slack-go/slack"
)

func TestBuildQuickActionsBlocks_PreselectsCurrentValues(t *testing.T) {
	ref := slackService.TicketRef{
		ID:     "tkt-1",
		SeqNum: 42,
		Title:  "Login fails on staging",
		URL:    "https://example.test/t/42",
	}
	state := slackService.QuickActionsTicketState{
		StatusID:    "in-progress",
		AssigneeIDs: []types.SlackUserID{"U111", "U222"},
	}
	statuses := []config.StatusDef{
		{ID: "open", Name: "Open"},
		{ID: "in-progress", Name: "In Progress"},
		{ID: "resolved", Name: "Resolved"},
	}

	blocks := slackService.BuildQuickActionsBlocks(context.Background(), ref, state, statuses)
	// Header (ticket ref) + section header + assignee block + status block.
	gt.A(t, blocks).Length(4)

	assigneeBlock, ok := blocks[2].(*slackgo.SectionBlock)
	gt.V(t, ok).Equal(true)
	gt.S(t, assigneeBlock.BlockID).Equal(slackService.QuickActionsAssigneeBlockID)
	users := assigneeBlock.Accessory.MultiSelectElement
	gt.V(t, users == nil).Equal(false)
	gt.S(t, users.ActionID).Equal(slackService.QuickActionsAssigneeActionID)
	gt.A(t, users.InitialUsers).Length(2)
	gt.S(t, users.InitialUsers[0]).Equal("U111")

	statusBlock, ok := blocks[3].(*slackgo.SectionBlock)
	gt.V(t, ok).Equal(true)
	gt.S(t, statusBlock.BlockID).Equal(slackService.QuickActionsStatusBlockID)
	sel := statusBlock.Accessory.SelectElement
	gt.V(t, sel == nil).Equal(false)
	gt.S(t, sel.ActionID).Equal(slackService.QuickActionsStatusActionID)
	gt.V(t, sel.InitialOption.Value).Equal("in-progress")
}

func TestBuildQuickActionsBlocks_NoAssignees_NoInitialUsers(t *testing.T) {
	ref := slackService.TicketRef{ID: "tkt-1", SeqNum: 1, Title: "T", URL: "https://x.test/1"}
	state := slackService.QuickActionsTicketState{StatusID: "open"}
	statuses := []config.StatusDef{{ID: "open", Name: "Open"}}

	blocks := slackService.BuildQuickActionsBlocks(context.Background(), ref, state, statuses)
	gt.A(t, blocks).Longer(2)

	assigneeBlock := blocks[2].(*slackgo.SectionBlock)
	users := assigneeBlock.Accessory.MultiSelectElement
	gt.A(t, users.InitialUsers).Length(0)
}
