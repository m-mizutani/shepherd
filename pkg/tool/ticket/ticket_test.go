package ticket_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/gt"
	"github.com/m-mizutani/shepherd/pkg/domain/interfaces"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
	"github.com/m-mizutani/shepherd/pkg/repository/memory"
	tticket "github.com/m-mizutani/shepherd/pkg/tool/ticket"
)

const wsID = types.WorkspaceID("ws-1")

// fakeEmbedder maps a known set of texts to canned vectors so tests can
// reason about cosine ordering without calling any external service. Texts
// not in the table fall through to errFn (or a sane default).
type fakeEmbedder struct {
	vectors map[string]firestore.Vector32
	errFn   func(text string) error
}

func (f *fakeEmbedder) Generate(_ context.Context, text string) (firestore.Vector32, string, error) {
	if f.errFn != nil {
		if err := f.errFn(text); err != nil {
			return nil, "", err
		}
	}
	if v, ok := f.vectors[text]; ok {
		return v, "fake:test-model", nil
	}
	return firestore.Vector32{0, 0, 0, 1}, "fake:test-model", nil
}

func setup(t *testing.T) (context.Context, *memory.Repository, map[string]gollem.Tool) {
	t.Helper()
	return setupWithEmbedder(t, nil)
}

func setupWithEmbedder(t *testing.T, embedder interfaces.Embedder) (context.Context, *memory.Repository, map[string]gollem.Tool) {
	t.Helper()
	repo := memory.New()
	t.Cleanup(func() { _ = repo.Close() })
	f := tticket.New(repo, embedder)
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
	return seedTicketWithEmbedding(t, repo, id, title, desc, status, updated, nil)
}

func seedTicketWithEmbedding(t *testing.T, repo *memory.Repository, id string, title, desc string, status types.StatusID, updated time.Time, embedding firestore.Vector32) *model.Ticket {
	t.Helper()
	tk := &model.Ticket{
		ID:          types.TicketID(id),
		WorkspaceID: wsID,
		Title:       title,
		Description: desc,
		StatusID:    status,
		Embedding:   embedding,
		CreatedAt:   updated,
		UpdatedAt:   updated,
	}
	if embedding != nil {
		tk.EmbeddingModel = "fake:test-model"
	}
	created, err := repo.Ticket().Create(context.Background(), wsID, tk)
	gt.NoError(t, err)
	return created
}

func TestSearchTool_Recency(t *testing.T) {
	// Empty query falls back to the legacy "newest first" listing, which
	// works without an embedder configured. status filter still applies.
	ctx, repo, tools := setup(t)
	now := time.Now()
	seedTicket(t, repo, "tk-old", "Login failure", "user can't sign in", "open", now.Add(-2*time.Hour))
	seedTicket(t, repo, "tk-new", "CSP error on login", "Content-Security-Policy blocked", "open", now)
	seedTicket(t, repo, "tk-closed", "old issue", "stale", "closed", now.Add(-time.Hour))

	t.Run("empty query lists newest first", func(t *testing.T) {
		out, err := tools["ticket_search"].Run(ctx, map[string]any{})
		gt.NoError(t, err)
		gt.Equal(t, out["count"].(int), 3)
		got := out["tickets"].([]map[string]any)
		gt.Equal(t, got[0]["id"], "tk-new")
		gt.Equal(t, got[1]["id"], "tk-closed")
		gt.Equal(t, got[2]["id"], "tk-old")
		gt.Equal(t, got[0]["distance"], 0.0)
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

func TestSearchTool_Semantic(t *testing.T) {
	// Hand-picked vectors so cosine ordering is deterministic: the query
	// "login" aligns with tk-near, partially with tk-mid, orthogonally with
	// tk-far. The semantic mode should rank them in that order.
	embedder := &fakeEmbedder{
		vectors: map[string]firestore.Vector32{
			"login": {1, 0, 0, 0},
		},
	}
	ctx, repo, tools := setupWithEmbedder(t, embedder)
	now := time.Now()
	seedTicketWithEmbedding(t, repo, "tk-near", "Login failure", "users cannot sign in", "open", now, firestore.Vector32{1, 0, 0, 0})
	seedTicketWithEmbedding(t, repo, "tk-mid", "Auth flow timing", "delays on the auth callback", "open", now, firestore.Vector32{0.7, 0.7, 0, 0})
	seedTicketWithEmbedding(t, repo, "tk-far", "Disk full alert", "node ran out of space", "closed", now, firestore.Vector32{0, 1, 0, 0})

	t.Run("semantic ranking by cosine distance", func(t *testing.T) {
		out, err := tools["ticket_search"].Run(ctx, map[string]any{"query": "login"})
		gt.NoError(t, err)
		gt.Equal(t, out["count"].(int), 3)
		got := out["tickets"].([]map[string]any)
		gt.Equal(t, got[0]["id"], "tk-near")
		gt.Equal(t, got[1]["id"], "tk-mid")
		gt.Equal(t, got[2]["id"], "tk-far")
		// Distance is a float64 cosine distance and must be monotonically
		// non-decreasing along the ranked list.
		d0 := got[0]["distance"].(float64)
		d1 := got[1]["distance"].(float64)
		d2 := got[2]["distance"].(float64)
		gt.N(t, d0).LessOrEqual(d1)
		gt.N(t, d1).LessOrEqual(d2)
	})

	t.Run("status filter applies in semantic mode", func(t *testing.T) {
		out, err := tools["ticket_search"].Run(ctx, map[string]any{
			"query":      "login",
			"status_ids": []any{"open"},
		})
		gt.NoError(t, err)
		gt.Equal(t, out["count"].(int), 2)
	})

	t.Run("embedder error returns soft failure", func(t *testing.T) {
		brokenEmbedder := &fakeEmbedder{
			errFn: func(_ string) error { return errors.New("upstream timeout") },
		}
		bctx, brepo, btools := setupWithEmbedder(t, brokenEmbedder)
		seedTicketWithEmbedding(t, brepo, "tk-near", "noop", "noop", "open", now, firestore.Vector32{1, 0, 0, 0})
		out, err := btools["ticket_search"].Run(bctx, map[string]any{"query": "anything"})
		gt.NoError(t, err)
		gt.Equal(t, out["count"].(int), 0)
		_, hasErr := out["error"]
		gt.B(t, hasErr).True()
	})

	t.Run("embedder absent returns soft failure", func(t *testing.T) {
		nctx, _, ntools := setup(t)
		out, err := ntools["ticket_search"].Run(nctx, map[string]any{"query": "anything"})
		gt.NoError(t, err)
		gt.Equal(t, out["count"].(int), 0)
		_, hasErr := out["error"]
		gt.B(t, hasErr).True()
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
