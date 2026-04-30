package slack

import (
	"context"

	"github.com/m-mizutani/shepherd/pkg/domain/model/config"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
	"github.com/m-mizutani/shepherd/pkg/utils/i18n"
	slackgo "github.com/slack-go/slack"
)

// Quick-actions Slack identifiers. The action_id values are the contract
// between the menu message and the HTTP interactions handler. The handler
// resolves the underlying ticket from the message's channel_id + thread_ts
// rather than from an embedded value, since the menu is always posted as
// a thread reply on the ticket's root message.
const (
	QuickActionsAssigneeBlockID  = "quick_actions_assignee"
	QuickActionsAssigneeActionID = "quick_actions_assignee_select"
	QuickActionsStatusBlockID    = "quick_actions_status"
	QuickActionsStatusActionID   = "quick_actions_status_select"
)

// QuickActionsTicketState bundles the bits of ticket state the menu
// renderer needs to seed initial values, so callers populate it from a
// *model.Ticket without pulling the full domain type into this layer.
type QuickActionsTicketState struct {
	StatusID    types.StatusID
	AssigneeIDs []types.SlackUserID
}

// BuildQuickActionsBlocks renders the empty-mention quick-actions menu:
// a ticket reference header, a section header, then two section blocks
// each carrying a select element as their accessory — multi_users_select
// for assignees, static_select for statuses. The blocks are placed in
// the section accessory slot (rather than an actions block) so each
// element gets its own block_id, which keeps the interactions handler
// straightforward when the user only changes one of them at a time.
func BuildQuickActionsBlocks(ctx context.Context, ref TicketRef, state QuickActionsTicketState, statuses []config.StatusDef) []slackgo.Block {
	loc := i18n.From(ctx)

	blocks := []slackgo.Block{}
	if header := ticketRefBlock(ctx, ref, TicketRefStateActive); header != nil {
		blocks = append(blocks, header)
	}
	blocks = append(blocks, slackgo.NewSectionBlock(
		slackgo.NewTextBlockObject(slackgo.MarkdownType,
			loc.T(i18n.MsgQuickActionsHeader), false, false),
		nil, nil,
	))

	blocks = append(blocks, buildQuickActionsAssigneeBlock(ctx, state.AssigneeIDs))
	blocks = append(blocks, buildQuickActionsStatusBlock(ctx, state.StatusID, statuses))

	return blocks
}

func buildQuickActionsAssigneeBlock(ctx context.Context, assignees []types.SlackUserID) slackgo.Block {
	loc := i18n.From(ctx)
	placeholder := slackgo.NewTextBlockObject(slackgo.PlainTextType,
		loc.T(i18n.MsgQuickActionsAssigneePlaceholder), false, false)
	users := slackgo.NewOptionsMultiSelectBlockElement(slackgo.MultiOptTypeUser, placeholder, QuickActionsAssigneeActionID)
	if len(assignees) > 0 {
		initial := make([]string, 0, len(assignees))
		for _, id := range assignees {
			if id == "" {
				continue
			}
			initial = append(initial, string(id))
		}
		users.InitialUsers = initial
	}
	block := slackgo.NewSectionBlock(
		slackgo.NewTextBlockObject(slackgo.MarkdownType,
			"*"+loc.T(i18n.MsgQuickActionsAssigneeLabel)+"*", false, false),
		nil,
		slackgo.NewAccessory(users),
	)
	block.BlockID = QuickActionsAssigneeBlockID
	return block
}

func buildQuickActionsStatusBlock(ctx context.Context, statusID types.StatusID, statuses []config.StatusDef) slackgo.Block {
	loc := i18n.From(ctx)
	placeholder := slackgo.NewTextBlockObject(slackgo.PlainTextType,
		loc.T(i18n.MsgQuickActionsStatusPlaceholder), false, false)

	options := make([]*slackgo.OptionBlockObject, 0, len(statuses))
	for _, s := range statuses {
		options = append(options, slackgo.NewOptionBlockObject(
			string(s.ID),
			slackgo.NewTextBlockObject(slackgo.PlainTextType, s.Name, false, false),
			nil,
		))
	}
	sel := slackgo.NewOptionsSelectBlockElement(slackgo.OptTypeStatic, placeholder, QuickActionsStatusActionID, options...)
	for _, opt := range options {
		if opt.Value == string(statusID) {
			sel = sel.WithInitialOption(opt)
			break
		}
	}
	block := slackgo.NewSectionBlock(
		slackgo.NewTextBlockObject(slackgo.MarkdownType,
			"*"+loc.T(i18n.MsgQuickActionsStatusLabel)+"*", false, false),
		nil,
		slackgo.NewAccessory(sel),
	)
	block.BlockID = QuickActionsStatusBlockID
	return block
}
