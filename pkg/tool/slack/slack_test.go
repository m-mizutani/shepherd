package slack_test

import (
	"context"
	"testing"

	"github.com/m-mizutani/gt"
	slackService "github.com/m-mizutani/shepherd/pkg/service/slack"
	tslack "github.com/m-mizutani/shepherd/pkg/tool/slack"
)

type fakeSlack struct {
	searchCalls   []searchCall
	threadCalls   []threadCall
	historyCalls  []historyCall
	userInfoCalls []string

	matches  []*slackService.SearchMatch
	messages []*slackService.Message
	user     *slackService.UserInfo
	err      error
}

type searchCall struct {
	query string
	count int
	sort  string
}

type threadCall struct {
	channel  string
	threadTS string
	limit    int
}

type historyCall struct {
	channel        string
	oldest, latest string
	limit          int
}

func (f *fakeSlack) SearchMessages(_ context.Context, q string, n int, s string) ([]*slackService.SearchMatch, error) {
	f.searchCalls = append(f.searchCalls, searchCall{q, n, s})
	return f.matches, f.err
}
func (f *fakeSlack) GetThreadMessages(_ context.Context, ch, ts string, n int) ([]*slackService.Message, error) {
	f.threadCalls = append(f.threadCalls, threadCall{ch, ts, n})
	return f.messages, f.err
}
func (f *fakeSlack) GetChannelHistory(_ context.Context, ch, o, l string, n int) ([]*slackService.Message, error) {
	f.historyCalls = append(f.historyCalls, historyCall{ch, o, l, n})
	return f.messages, f.err
}
func (f *fakeSlack) GetUserInfo(_ context.Context, id string) (*slackService.UserInfo, error) {
	f.userInfoCalls = append(f.userInfoCalls, id)
	return f.user, f.err
}

func toolByName(t *testing.T, name string, svc tslack.SlackTooler) func(map[string]any) (map[string]any, error) {
	t.Helper()
	f := tslack.New(svc)
	if err := f.Init(context.Background()); err != nil {
		t.Fatalf("factory init: %v", err)
	}
	for _, tool := range f.Tools() {
		if tool.Spec().Name == name {
			return func(args map[string]any) (map[string]any, error) {
				return tool.Run(context.Background(), args)
			}
		}
	}
	t.Fatalf("tool %q not found", name)
	return nil
}

func TestSearchMessages(t *testing.T) {
	t.Run("happy path passes args and formats matches", func(t *testing.T) {
		f := &fakeSlack{
			matches: []*slackService.SearchMatch{
				{ChannelID: "C1", ChannelName: "alerts", User: "U1", Text: "boom", Timestamp: "1.0", Permalink: "https://x"},
			},
		}
		run := toolByName(t, "slack_search_messages", f)
		out, err := run(map[string]any{"query": "boom", "limit": 5, "sort": "timestamp"})
		gt.NoError(t, err)
		gt.Equal(t, len(f.searchCalls), 1)
		gt.Equal(t, f.searchCalls[0].query, "boom")
		gt.Equal(t, f.searchCalls[0].count, 5)
		gt.Equal(t, f.searchCalls[0].sort, "timestamp")
		gt.Equal(t, out["count"].(int), 1)
		matches := out["matches"].([]map[string]any)
		gt.Equal(t, matches[0]["channel_name"], "alerts")
	})

	t.Run("missing query errors without calling slack", func(t *testing.T) {
		f := &fakeSlack{}
		run := toolByName(t, "slack_search_messages", f)
		_, err := run(map[string]any{})
		gt.Error(t, err)
		gt.Equal(t, len(f.searchCalls), 0)
	})

	t.Run("limit clamps to max", func(t *testing.T) {
		f := &fakeSlack{}
		run := toolByName(t, "slack_search_messages", f)
		_, err := run(map[string]any{"query": "x", "limit": 9999})
		gt.NoError(t, err)
		gt.Equal(t, f.searchCalls[0].count, 50)
	})
}

func TestGetThread(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		f := &fakeSlack{
			messages: []*slackService.Message{
				{User: "U1", Text: "hi", Timestamp: "1.0"},
				{User: "U2", Text: "yo", Timestamp: "1.1"},
			},
		}
		run := toolByName(t, "slack_get_thread", f)
		out, err := run(map[string]any{"channel_id": "C1", "thread_ts": "1.0"})
		gt.NoError(t, err)
		gt.Equal(t, len(f.threadCalls), 1)
		gt.Equal(t, f.threadCalls[0].channel, "C1")
		gt.Equal(t, f.threadCalls[0].threadTS, "1.0")
		gt.Equal(t, f.threadCalls[0].limit, 50)
		gt.Equal(t, out["count"].(int), 2)
	})

	t.Run("missing thread_ts errors", func(t *testing.T) {
		f := &fakeSlack{}
		run := toolByName(t, "slack_get_thread", f)
		_, err := run(map[string]any{"channel_id": "C1"})
		gt.Error(t, err)
		gt.Equal(t, len(f.threadCalls), 0)
	})
}

func TestGetChannelHistory(t *testing.T) {
	f := &fakeSlack{messages: []*slackService.Message{{User: "U1", Text: "hi", Timestamp: "1.0"}}}
	run := toolByName(t, "slack_get_channel_history", f)
	out, err := run(map[string]any{"channel_id": "C1", "oldest": "0", "latest": "9", "limit": 10})
	gt.NoError(t, err)
	gt.Equal(t, len(f.historyCalls), 1)
	gt.Equal(t, f.historyCalls[0].oldest, "0")
	gt.Equal(t, f.historyCalls[0].latest, "9")
	gt.Equal(t, f.historyCalls[0].limit, 10)
	gt.Equal(t, out["count"].(int), 1)
}

func TestGetUserInfo(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		f := &fakeSlack{user: &slackService.UserInfo{ID: "U1", Name: "alice", Email: "a@x"}}
		run := toolByName(t, "slack_get_user_info", f)
		out, err := run(map[string]any{"user_id": "U1"})
		gt.NoError(t, err)
		gt.Equal(t, out["found"].(bool), true)
		gt.Equal(t, out["name"], "alice")
		gt.Equal(t, f.userInfoCalls[0], "U1")
	})

	t.Run("not found", func(t *testing.T) {
		f := &fakeSlack{user: nil}
		run := toolByName(t, "slack_get_user_info", f)
		out, err := run(map[string]any{"user_id": "U1"})
		gt.NoError(t, err)
		gt.Equal(t, out["found"].(bool), false)
	})

	t.Run("missing user_id errors", func(t *testing.T) {
		f := &fakeSlack{}
		run := toolByName(t, "slack_get_user_info", f)
		_, err := run(map[string]any{})
		gt.Error(t, err)
		gt.Equal(t, len(f.userInfoCalls), 0)
	})
}
