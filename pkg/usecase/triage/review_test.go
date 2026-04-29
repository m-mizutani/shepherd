package triage_test

import (
	"context"
	"testing"

	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/gollem/mock"
	"github.com/m-mizutani/gt"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
	slackService "github.com/m-mizutani/shepherd/pkg/service/slack"
	"github.com/m-mizutani/shepherd/pkg/usecase/triage"
	"github.com/m-mizutani/shepherd/pkg/utils/async"
	slackgo "github.com/slack-go/slack"
)

// seedReviewProposal stores a complete-kind plan as the latest assistant turn
// in the ticket's plan history, mimicking what enterReview would have written
// just before parking on the buttons.
func seedReviewProposal(t *testing.T, hist *fakeHistoryRepo, ticketID types.TicketID) {
	t.Helper()
	h := &gollem.History{Version: gollem.HistoryVersion}
	h.Messages = append(h.Messages, mustAssistantPlanMessage(t, completePlanJSON))
	gt.NoError(t, hist.Save(context.Background(), triage.PlanSessionIDForTest(tWS, ticketID), h))
}

func TestHandleReviewSubmit_HappyPath_FinalizesAndPostsFollowup(t *testing.T) {
	uc, _, repo, hist, slack := newRig(t, nil)
	ticket := mustCreateTicket(t, repo, false)
	seedReviewProposal(t, hist, ticket.ID)

	gt.NoError(t, uc.HandleReviewSubmit(context.Background(), ticket.ID, tChannel, "1234.5678", "Uactor"))

	got := gt.R1(repo.Ticket().Get(context.Background(), tWS, ticket.ID)).NoError(t)
	gt.True(t, got.Triaged)
	gt.Equal(t, got.AssigneeID, types.SlackUserID("U123"))

	// Original review message is rewritten to remove buttons (1 update); the
	// LLM hand-off message is posted as a fresh thread reply (1 post).
	gt.A(t, slack.updates).Length(1)
	gt.A(t, slack.posts).Length(1)
}

func TestHandleReviewSubmit_AlreadyTriaged_NoOpEphemeral(t *testing.T) {
	uc, _, repo, hist, slack := newRig(t, nil)
	ticket := mustCreateTicket(t, repo, true)
	seedReviewProposal(t, hist, ticket.ID)

	gt.NoError(t, uc.HandleReviewSubmit(context.Background(), ticket.ID, tChannel, "1234.5678", "Uactor"))

	// No additional Slack posts; ephemerals do not flow through fakeTriageSlack.posts.
	gt.A(t, slack.posts).Length(0)
	gt.A(t, slack.updates).Length(0)
}

func TestHandleReviewSubmit_NoCompleteProposal_NoOpEphemeral(t *testing.T) {
	uc, _, repo, hist, slack := newRig(t, nil)
	ticket := mustCreateTicket(t, repo, false)
	// Seed an Ask plan instead of Complete; submit must refuse.
	h := &gollem.History{Version: gollem.HistoryVersion}
	h.Messages = append(h.Messages, mustAssistantPlanMessage(t, askPlanJSON))
	gt.NoError(t, hist.Save(context.Background(), triage.PlanSessionIDForTest(tWS, ticket.ID), h))

	gt.NoError(t, uc.HandleReviewSubmit(context.Background(), ticket.ID, tChannel, "1234.5678", "Uactor"))

	got := gt.R1(repo.Ticket().Get(context.Background(), tWS, ticket.ID)).NoError(t)
	gt.False(t, got.Triaged)
	gt.A(t, slack.posts).Length(0)
}

func TestHandleReviewEditOpen_OpensEditModal(t *testing.T) {
	uc, _, repo, hist, slack := newRig(t, nil)
	ticket := mustCreateTicket(t, repo, false)
	seedReviewProposal(t, hist, ticket.ID)

	gt.NoError(t, uc.HandleReviewEditOpen(context.Background(), ticket.ID, tChannel, "1234.5678", "trigger-1"))

	gt.A(t, slack.views).Length(1)
	gt.S(t, slack.views[0].callbackID).Equal(slackService.TriageReviewEditModalCallbackID)
}

func TestHandleReviewReinvestigateOpen_OpensInstructionModal(t *testing.T) {
	uc, _, repo, _, slack := newRig(t, nil)
	ticket := mustCreateTicket(t, repo, false)

	gt.NoError(t, uc.HandleReviewReinvestigateOpen(context.Background(), ticket.ID, tChannel, "1234.5678", "trigger-2"))

	gt.A(t, slack.views).Length(1)
	gt.S(t, slack.views[0].callbackID).Equal(slackService.TriageReviewReinvestigateModalCallbackID)
}

func TestHandleReviewEditSubmit_AppliesEditedAssigneeAndFinalizes(t *testing.T) {
	uc, _, repo, hist, slack := newRig(t, nil)
	ticket := mustCreateTicket(t, repo, false)
	seedReviewProposal(t, hist, ticket.ID)

	state := &slackgo.ViewState{Values: map[string]map[string]slackgo.BlockAction{
		slackService.TriageReviewTitleBlockID: {
			slackService.TriageReviewTitleActionID: {Value: "Edited title"},
		},
		slackService.TriageReviewSummaryBlockID: {
			slackService.TriageReviewSummaryActionID: {Value: "Edited summary"},
		},
		slackService.TriageReviewAssigneeBlockID: {
			slackService.TriageReviewAssigneeActionID: {SelectedUser: "U999"},
		},
	}}

	fieldErrs, err := uc.HandleReviewEditSubmit(context.Background(), ticket.ID, tChannel, "1234.5678", "Uactor", state)
	gt.NoError(t, err)
	gt.Nil(t, fieldErrs)
	// Edit submit defers the heavy tail (finalize + chat.update + handoff
	// post) into async.Dispatch to honor Slack's 3s view_submission deadline,
	// so the test must wait before asserting on side effects.
	async.Wait()

	got := gt.R1(repo.Ticket().Get(context.Background(), tWS, ticket.ID)).NoError(t)
	gt.True(t, got.Triaged)
	// finalizeComplete used the edited assignee, not the planner's U123.
	gt.Equal(t, got.AssigneeID, types.SlackUserID("U999"))
	// Edited title / summary are persisted onto the ticket itself so the
	// values the user confirmed in the modal become the authoritative
	// ticket headline + body.
	gt.S(t, got.Title).Equal("Edited title")
	gt.S(t, got.Description).Equal("Edited summary")

	// Edit submit deactivates the original review message (1 update) and
	// posts the LLM hand-off as a fresh reply (1 post).
	gt.A(t, slack.updates).Length(1)
	gt.A(t, slack.posts).Length(1)
}

func TestHandleReviewReinvestigate_AppendsUserMessageAndDispatches(t *testing.T) {
	// Use an LLM that loops with another Ask so executor.run does not blow up
	// when async.Dispatch fires the planner. The post-instruction message
	// must already be present in history before the planner runs.
	session := newPlanSessionMock(t, []string{askPlanJSON})
	llm := &mock.LLMClientMock{
		NewSessionFunc: func(_ context.Context, _ ...gollem.SessionOption) (gollem.Session, error) {
			return session, nil
		},
	}

	uc, _, repo, hist, slack := newRig(t, llm)
	ticket := mustCreateTicket(t, repo, false)
	seedReviewProposal(t, hist, ticket.ID)

	state := &slackgo.ViewState{Values: map[string]map[string]slackgo.BlockAction{
		slackService.TriageReviewInstructionBlock: {
			slackService.TriageReviewInstructionAction: {Value: "Look at the auth service logs"},
		},
	}}

	gt.NoError(t, uc.HandleReviewReinvestigate(context.Background(), ticket.ID, tChannel, "1234.5678", "Uactor", state))
	async.Wait()

	// Ticket stays Triaged=false (planner re-dispatched, but did not finalize).
	got := gt.R1(repo.Ticket().Get(context.Background(), tWS, ticket.ID)).NoError(t)
	gt.False(t, got.Triaged)

	// Slack: original review message is rewritten to a "Re-investigation
	// requested" state (1 update), then the planner re-runs and the
	// "Re-investigating…" + new planner ask messages land as fresh posts.
	gt.A(t, slack.updates).Length(1)
	gt.B(t, len(slack.posts) >= 1).True()
	var sawInstructionEcho bool
	for _, p := range slack.posts {
		for _, b := range p.blocks {
			sec, ok := b.(*slackgo.SectionBlock)
			if !ok || sec.Text == nil {
				continue
			}
			if containsString(sec.Text.Text, "Look at the auth service logs") {
				sawInstructionEcho = true
			}
		}
	}
	gt.True(t, sawInstructionEcho)
}

func containsString(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (func() bool {
		for i := 0; i+len(needle) <= len(haystack); i++ {
			if haystack[i:i+len(needle)] == needle {
				return true
			}
		}
		return false
	})()
}
