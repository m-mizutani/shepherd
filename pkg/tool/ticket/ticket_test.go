package ticket_test

import (
	"context"
	"testing"
	"time"

	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/gt"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
	"github.com/m-mizutani/shepherd/pkg/repository/memory"
	tticket "github.com/m-mizutani/shepherd/pkg/tool/ticket"
)

const wsID = types.WorkspaceID("ws-1")

func setup(t *testing.T) (context.Context, *memory.Repository, map[string]gollem.Tool) {
	t.Helper()
	repo := memory.New()
	t.Cleanup(func() { _ = repo.Close() })
	f := tticket.New(repo)
	gt.NoError(t, f.Init(context.Background()))
	tools := f.Tools()
	byName := make(map[string]gollem.Tool, len(tools))
	for _, tool := range tools {
		byName[tool.Spec().Name] = tool
	}
	ctx := types.ContextWithWorkspace(context.Background(), wsID)
	return ctx, repo, byName
}

func seedTicket(t *testing.T, repo *memory.Repository, id string, title, desc string, status types.StatusID, updated time.Time) *model.Ticket {
	t.Helper()
	tk := &model.Ticket{
		ID:          types.TicketID(id),
		WorkspaceID: wsID,
		Title:       title,
		Description: desc,
		StatusID:    status,
		CreatedAt:   updated,
		UpdatedAt:   updated,
	}
	created, err := repo.Ticket().Create(context.Background(), wsID, tk)
	gt.NoError(t, err)
	return created
}

func TestSearchTool(t *testing.T) {
	ctx, repo, tools := setup(t)
	now := time.Now()
	seedTicket(t, repo, "tk-old", "Login failure", "user can't sign in", "open", now.Add(-2*time.Hour))
	seedTicket(t, repo, "tk-new", "CSP error on login", "Content-Security-Policy blocked", "open", now)
	seedTicket(t, repo, "tk-closed", "old issue", "stale", "closed", now.Add(-time.Hour))

	t.Run("text query matches title and description, sorted newest", func(t *testing.T) {
		out, err := tools["ticket_search"].Run(ctx, map[string]any{"query": "login"})
		gt.NoError(t, err)
		gt.Equal(t, out["count"].(int), 2)
		got := out["tickets"].([]map[string]any)
		gt.Equal(t, got[0]["id"], "tk-new")
		gt.Equal(t, got[1]["id"], "tk-old")
	})

	t.Run("status filter narrows", func(t *testing.T) {
		out, err := tools["ticket_search"].Run(ctx, map[string]any{
			"status_ids": []any{"closed"},
		})
		gt.NoError(t, err)
		gt.Equal(t, out["count"].(int), 1)
		got := out["tickets"].([]map[string]any)
		gt.Equal(t, got[0]["id"], "tk-closed")
	})

	t.Run("limit clamps", func(t *testing.T) {
		out, err := tools["ticket_search"].Run(ctx, map[string]any{"limit": 1})
		gt.NoError(t, err)
		gt.Equal(t, out["count"].(int), 1)
	})

	t.Run("missing workspace context errors", func(t *testing.T) {
		_, err := tools["ticket_search"].Run(context.Background(), map[string]any{})
		gt.Error(t, err)
	})
}

func TestGetTool(t *testing.T) {
	ctx, repo, tools := setup(t)
	tk := seedTicket(t, repo, "tk-1", "boom", "details", "open", time.Now())

	t.Run("by ticket_id", func(t *testing.T) {
		out, err := tools["ticket_get"].Run(ctx, map[string]any{"ticket_id": "tk-1"})
		gt.NoError(t, err)
		gt.Equal(t, out["found"].(bool), true)
		got := out["ticket"].(map[string]any)
		gt.Equal(t, got["title"], "boom")
	})

	t.Run("by seq_num", func(t *testing.T) {
		out, err := tools["ticket_get"].Run(ctx, map[string]any{"seq_num": int64(tk.SeqNum)})
		gt.NoError(t, err)
		gt.Equal(t, out["found"].(bool), true)
	})

	t.Run("not found returns found:false", func(t *testing.T) {
		out, err := tools["ticket_get"].Run(ctx, map[string]any{"ticket_id": "missing"})
		gt.NoError(t, err)
		gt.Equal(t, out["found"].(bool), false)
	})

	t.Run("rejects both args", func(t *testing.T) {
		_, err := tools["ticket_get"].Run(ctx, map[string]any{"ticket_id": "x", "seq_num": int64(1)})
		gt.Error(t, err)
	})

	t.Run("rejects neither arg", func(t *testing.T) {
		_, err := tools["ticket_get"].Run(ctx, map[string]any{})
		gt.Error(t, err)
	})
}

func TestGetCommentsAndHistory(t *testing.T) {
	ctx, repo, tools := setup(t)
	seedTicket(t, repo, "tk-1", "x", "y", "open", time.Now())

	now := time.Now()
	_, err := repo.Comment().Create(context.Background(), wsID, "tk-1", &model.Comment{
		ID: "c-1", TicketID: "tk-1", SlackUserID: "U-1", Body: "first", CreatedAt: now,
	})
	gt.NoError(t, err)
	_, err = repo.TicketHistory().Create(context.Background(), wsID, "tk-1", &model.TicketHistory{
		ID: "h-1", NewStatusID: "open", Action: "created", CreatedAt: now,
	})
	gt.NoError(t, err)

	t.Run("comments", func(t *testing.T) {
		out, err := tools["ticket_get_comments"].Run(ctx, map[string]any{"ticket_id": "tk-1"})
		gt.NoError(t, err)
		gt.Equal(t, out["count"].(int), 1)
		comments := out["comments"].([]map[string]any)
		gt.Equal(t, comments[0]["body"], "first")
	})

	t.Run("comments missing ticket_id errors", func(t *testing.T) {
		_, err := tools["ticket_get_comments"].Run(ctx, map[string]any{})
		gt.Error(t, err)
	})

	t.Run("history", func(t *testing.T) {
		out, err := tools["ticket_get_history"].Run(ctx, map[string]any{"ticket_id": "tk-1"})
		gt.NoError(t, err)
		gt.Equal(t, out["count"].(int), 1)
	})
}
