package usecase_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/gollem/mock"
	"github.com/m-mizutani/gt"
	"github.com/m-mizutani/shepherd/pkg/adapter/agentstore"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
	"github.com/m-mizutani/shepherd/pkg/domain/model/config"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
	"github.com/m-mizutani/shepherd/pkg/repository/memory"
	slackService "github.com/m-mizutani/shepherd/pkg/service/slack"
	"github.com/m-mizutani/shepherd/pkg/usecase"
)

const (
	testWS      = types.WorkspaceID("ws-test")
	testChannel = "C-mapped"
)

type fakeSlackClient struct {
	mu             sync.Mutex
	threadReplies  []threadReplyCall
	ticketCreated  []ticketCreatedCall
	usersByID      map[string]*slackService.UserInfo
}

type threadReplyCall struct {
	channelID string
	threadTS  string
	text      string
}

type ticketCreatedCall struct {
	channelID string
	threadTS  string
	seqNum    int64
	ticketURL string
}

func (f *fakeSlackClient) ReplyThread(_ context.Context, channelID, threadTS, text string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.threadReplies = append(f.threadReplies, threadReplyCall{channelID, threadTS, text})
	return nil
}

func (f *fakeSlackClient) ReplyTicketCreated(_ context.Context, channelID, threadTS string, seqNum int64, ticketURL string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ticketCreated = append(f.ticketCreated, ticketCreatedCall{channelID, threadTS, seqNum, ticketURL})
	return nil
}

func (f *fakeSlackClient) GetUserInfo(_ context.Context, userID string) (*slackService.UserInfo, error) {
	if u, ok := f.usersByID[userID]; ok {
		return u, nil
	}
	return &slackService.UserInfo{ID: userID, Name: userID}, nil
}

func (f *fakeSlackClient) ListUsers(_ context.Context) ([]*slackService.UserInfo, error) {
	out := make([]*slackService.UserInfo, 0, len(f.usersByID))
	for _, u := range f.usersByID {
		out = append(out, u)
	}
	return out, nil
}

func newSlackTestRig(t *testing.T, llm gollem.LLMClient) (*usecase.SlackUseCase, *fakeSlackClient, *memory.Repository, *model.WorkspaceRegistry, string) {
	t.Helper()
	repo := memory.New()
	t.Cleanup(func() { _ = repo.Close() })

	registry := model.NewWorkspaceRegistry()
	registry.Register(&model.WorkspaceEntry{
		Workspace: model.Workspace{ID: testWS, Name: "Test"},
		FieldSchema: &config.FieldSchema{
			Statuses: []config.StatusDef{{ID: "open", Name: "Open"}},
			TicketConfig: config.TicketConfig{
				DefaultStatusID: "open",
			},
		},
		SlackChannelID: types.SlackChannelID(testChannel),
	})

	storeDir := t.TempDir()
	be := gt.R1(agentstore.NewFileBackend(storeDir)).NoError(t)
	t.Cleanup(func() { _ = be.Close() })
	historyRepo := agentstore.NewHistoryRepository(be)
	traceRepo := agentstore.NewTraceRepository(be)

	slack := &fakeSlackClient{usersByID: map[string]*slackService.UserInfo{}}
	uc := usecase.NewSlackUseCase(repo, registry, slack, "https://shepherd.example.com", llm, historyRepo, traceRepo)
	return uc, slack, repo, registry, storeDir
}

func TestSlackUseCase_HandleNewMessage_CreatesTicketAndReplies(t *testing.T) {
	uc, slack, repo, _, _ := newSlackTestRig(t, nil)
	ctx := context.Background()

	gt.NoError(t, uc.HandleNewMessage(ctx, testChannel, "U1", "First report", "100.000"))

	tickets := gt.R1(repo.Ticket().List(ctx, testWS, nil)).NoError(t)
	gt.A(t, tickets).Length(1)
	gt.S(t, tickets[0].Title).Equal("First report")
	gt.S(t, string(tickets[0].SlackThreadTS)).Equal("100.000")

	gt.A(t, slack.ticketCreated).Length(1)
	gt.S(t, slack.ticketCreated[0].channelID).Equal(testChannel)
	gt.S(t, slack.ticketCreated[0].threadTS).Equal("100.000")
}

func TestSlackUseCase_HandleThreadReply_PersistsComment(t *testing.T) {
	uc, _, repo, _, _ := newSlackTestRig(t, nil)
	ctx := context.Background()

	gt.NoError(t, uc.HandleNewMessage(ctx, testChannel, "U1", "ticket body", "200.000"))
	gt.NoError(t, uc.HandleThreadReply(ctx, testChannel, "200.000", "U2", "first reply", "200.001", false))
	gt.NoError(t, uc.HandleThreadReply(ctx, testChannel, "200.000", "U3", "second reply", "200.002", true))

	tickets := gt.R1(repo.Ticket().List(ctx, testWS, nil)).NoError(t)
	gt.A(t, tickets).Length(1)

	comments := gt.R1(repo.Comment().List(ctx, testWS, tickets[0].ID)).NoError(t)
	gt.A(t, comments).Length(2)

	bodies := []string{comments[0].Body, comments[1].Body}
	gt.A(t, bodies).Has("first reply").Has("second reply")

	for _, c := range comments {
		switch c.Body {
		case "first reply":
			gt.S(t, string(c.SlackUserID)).Equal("U2")
			gt.False(t, c.IsBot)
		case "second reply":
			gt.S(t, string(c.SlackUserID)).Equal("U3")
			gt.True(t, c.IsBot)
		}
	}
}

func TestSlackUseCase_HandleThreadReply_DeduplicatesBySlackTS(t *testing.T) {
	uc, _, repo, _, _ := newSlackTestRig(t, nil)
	ctx := context.Background()

	gt.NoError(t, uc.HandleNewMessage(ctx, testChannel, "U1", "body", "300.000"))
	gt.NoError(t, uc.HandleThreadReply(ctx, testChannel, "300.000", "U2", "dup", "300.001", false))
	gt.NoError(t, uc.HandleThreadReply(ctx, testChannel, "300.000", "U2", "dup", "300.001", false))

	tickets := gt.R1(repo.Ticket().List(ctx, testWS, nil)).NoError(t)
	comments := gt.R1(repo.Comment().List(ctx, testWS, tickets[0].ID)).NoError(t)
	gt.A(t, comments).Length(1)
}

func TestSlackUseCase_HandleAppMention_PostsLLMReply(t *testing.T) {
	var capturedUserPrompt string
	session := &mock.SessionMock{
		GenerateFunc: func(_ context.Context, input []gollem.Input, _ ...gollem.GenerateOption) (*gollem.Response, error) {
			if len(input) > 0 {
				capturedUserPrompt = input[0].String()
			}
			return &gollem.Response{Texts: []string{"Here is what I found.\nLooks like a CSP issue."}}, nil
		},
		HistoryFunc: func() (*gollem.History, error) {
			return &gollem.History{
				LLType:  gollem.LLMTypeOpenAI,
				Version: gollem.HistoryVersion,
			}, nil
		},
		AppendHistoryFunc: func(_ *gollem.History) error { return nil },
	}

	var capturedSystemPrompt string
	llm := &mock.LLMClientMock{
		NewSessionFunc: func(_ context.Context, opts ...gollem.SessionOption) (gollem.Session, error) {
			cfg := gollem.NewSessionConfig(opts...)
			capturedSystemPrompt = cfg.SystemPrompt()
			return session, nil
		},
	}

	uc, slack, repo, _, storeDir := newSlackTestRig(t, llm)
	ctx := context.Background()

	gt.NoError(t, uc.HandleNewMessage(ctx, testChannel, "Ureporter", "Login fails on Safari", "400.000"))
	gt.NoError(t, uc.HandleThreadReply(ctx, testChannel, "400.000", "Uengineer", "Looks like CSP", "400.001", false))

	// drop the ticket-created reply so we can isolate the app_mention reply
	slack.threadReplies = nil

	gt.NoError(t, uc.HandleAppMention(ctx, testChannel, "Ucarol", "<@UBOT> any update?", "400.002", "400.000"))

	// System prompt carries the static ticket context.
	gt.S(t, capturedSystemPrompt).Contains("Login fails on Safari")

	// User prompt only carries the latest mention; ticket context must NOT
	// appear here (that's what the system prompt is for).
	gt.S(t, capturedUserPrompt).Contains("any update?")
	if strings.Contains(capturedUserPrompt, "Login fails on Safari") {
		t.Errorf("user prompt must not contain ticket title, got:\n%s", capturedUserPrompt)
	}
	if len(capturedUserPrompt) == 0 || containsMentionToken(capturedUserPrompt) {
		t.Errorf("mention tokens should be stripped from user prompt, got:\n%s", capturedUserPrompt)
	}

	// Slack received exactly one threaded reply with the LLM output.
	gt.A(t, slack.threadReplies).Length(1)
	gt.S(t, slack.threadReplies[0].channelID).Equal(testChannel)
	gt.S(t, slack.threadReplies[0].threadTS).Equal("400.000")
	gt.S(t, slack.threadReplies[0].text).Equal("Here is what I found.\nLooks like a CSP issue.")

	calls := session.GenerateCalls()
	gt.A(t, calls).Length(1)
	gt.A(t, calls[0].Input).Length(1)

	// Agent storage must contain the persisted history & trace for this run.
	tickets := gt.R1(repo.Ticket().List(ctx, testWS, nil)).NoError(t)
	gt.A(t, tickets).Length(1)
	historyPath := filepath.Join(storeDir, "history", "v1", string(testWS), string(tickets[0].ID)+".json")
	_ = gt.R1(os.Stat(historyPath)).NoError(t)

	traceDir := filepath.Join(storeDir, "trace", "v1")
	entries := gt.R1(os.ReadDir(traceDir)).NoError(t)
	gt.A(t, entries).Length(1)
}

func TestSlackUseCase_HandleAppMention_NoLLMConfigured(t *testing.T) {
	uc, slack, _, _, _ := newSlackTestRig(t, nil)
	gt.NoError(t, uc.HandleAppMention(context.Background(), testChannel, "U1", "<@UBOT> hi", "1.0", "1.0"))
	gt.A(t, slack.threadReplies).Length(0)
}

func TestSlackUseCase_HandleAppMention_NoTicketForThread(t *testing.T) {
	llm := &mock.LLMClientMock{
		NewSessionFunc: func(_ context.Context, _ ...gollem.SessionOption) (gollem.Session, error) {
			t.Fatalf("LLM should not be invoked when no ticket is bound to the thread")
			return nil, nil
		},
	}
	uc, slack, _, _, _ := newSlackTestRig(t, llm)
	gt.NoError(t, uc.HandleAppMention(context.Background(), testChannel, "U1", "<@UBOT> hi", "9.0", "9.0"))
	gt.A(t, slack.threadReplies).Length(0)
}

func TestSlackUseCase_HandleAppMention_UnknownChannel(t *testing.T) {
	llm := &mock.LLMClientMock{
		NewSessionFunc: func(_ context.Context, _ ...gollem.SessionOption) (gollem.Session, error) {
			t.Fatalf("LLM should not be invoked for unmapped channels")
			return nil, nil
		},
	}
	uc, slack, _, _, _ := newSlackTestRig(t, llm)
	gt.NoError(t, uc.HandleAppMention(context.Background(), "C-not-mapped", "U1", "<@UBOT> hi", "1.0", "1.0"))
	gt.A(t, slack.threadReplies).Length(0)
}

func containsMentionToken(s string) bool {
	for i := 0; i < len(s)-1; i++ {
		if s[i] == '<' && (s[i+1] == '@' || s[i+1] == '!') {
			return true
		}
	}
	return false
}
