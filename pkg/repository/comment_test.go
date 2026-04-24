package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/m-mizutani/shepherd/pkg/domain/interfaces"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
	"github.com/m-mizutani/shepherd/pkg/domain/model/auth"
)

func TestCommentCreate(t *testing.T) {
	runTest(t, "CommentCreate", func(t *testing.T, repo interfaces.Repository) {
		ctx := context.Background()
		now := time.Now().Truncate(time.Millisecond)
		wsID := "test-comment-" + uuid.Must(uuid.NewV7()).String()[:8]
		ticketID := uuid.Must(uuid.NewV7()).String()

		// Create a ticket first
		ticket := &model.Ticket{
			ID:          ticketID,
			WorkspaceID: wsID,
			Title:       "Comment Test Ticket",
			StatusID:    "open",
			FieldValues: make(map[string]model.FieldValue),
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if _, err := repo.Ticket().Create(ctx, wsID, ticket); err != nil {
			t.Fatalf("Create ticket failed: %v", err)
		}

		comment := &model.Comment{
			ID:          uuid.Must(uuid.NewV7()).String(),
			TicketID:    ticketID,
			SlackUserID: "U123",
			Body:        "Hello world",
			SlackTS:     "1111111111.111111",
			CreatedAt:   now,
		}
		created, err := repo.Comment().Create(ctx, wsID, ticketID, comment)
		if err != nil {
			t.Fatalf("Create comment failed: %v", err)
		}
		if created.Body != "Hello world" {
			t.Errorf("expected body 'Hello world', got %q", created.Body)
		}
	})
}

func TestCommentList(t *testing.T) {
	runTest(t, "CommentList", func(t *testing.T, repo interfaces.Repository) {
		ctx := context.Background()
		now := time.Now().Truncate(time.Millisecond)
		wsID := "test-clist-" + uuid.Must(uuid.NewV7()).String()[:8]
		ticketID := uuid.Must(uuid.NewV7()).String()

		ticket := &model.Ticket{
			ID:          ticketID,
			WorkspaceID: wsID,
			Title:       "Comment List Test",
			StatusID:    "open",
			FieldValues: make(map[string]model.FieldValue),
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if _, err := repo.Ticket().Create(ctx, wsID, ticket); err != nil {
			t.Fatalf("Create ticket failed: %v", err)
		}

		for i, body := range []string{"First", "Second", "Third"} {
			comment := &model.Comment{
				ID:          uuid.Must(uuid.NewV7()).String(),
				TicketID:    ticketID,
				SlackUserID: "U123",
				Body:        body,
				SlackTS:     "1111111111.11111" + string(rune('0'+i)),
				CreatedAt:   now.Add(time.Duration(i) * time.Second),
			}
			if _, err := repo.Comment().Create(ctx, wsID, ticketID, comment); err != nil {
				t.Fatalf("Create comment %d failed: %v", i, err)
			}
		}

		comments, err := repo.Comment().List(ctx, wsID, ticketID)
		if err != nil {
			t.Fatalf("List comments failed: %v", err)
		}
		if len(comments) != 3 {
			t.Errorf("expected 3 comments, got %d", len(comments))
		}
	})
}

func TestCommentGetBySlackTS(t *testing.T) {
	runTest(t, "CommentGetBySlackTS", func(t *testing.T, repo interfaces.Repository) {
		ctx := context.Background()
		now := time.Now().Truncate(time.Millisecond)
		wsID := "test-cslack-" + uuid.Must(uuid.NewV7()).String()[:8]
		ticketID := uuid.Must(uuid.NewV7()).String()

		ticket := &model.Ticket{
			ID:          ticketID,
			WorkspaceID: wsID,
			Title:       "Comment Slack TS Test",
			StatusID:    "open",
			FieldValues: make(map[string]model.FieldValue),
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if _, err := repo.Ticket().Create(ctx, wsID, ticket); err != nil {
			t.Fatalf("Create ticket failed: %v", err)
		}

		slackTS := "2222222222.222222"
		comment := &model.Comment{
			ID:          uuid.Must(uuid.NewV7()).String(),
			TicketID:    ticketID,
			SlackUserID: "U456",
			Body:        "Slack TS test comment",
			SlackTS:     slackTS,
			CreatedAt:   now,
		}
		if _, err := repo.Comment().Create(ctx, wsID, ticketID, comment); err != nil {
			t.Fatalf("Create comment failed: %v", err)
		}

		got, err := repo.Comment().GetBySlackTS(ctx, wsID, ticketID, slackTS)
		if err != nil {
			t.Fatalf("GetBySlackTS failed: %v", err)
		}
		if got == nil {
			t.Fatal("expected comment, got nil")
		}
		if got.Body != "Slack TS test comment" {
			t.Errorf("expected body 'Slack TS test comment', got %q", got.Body)
		}

		notFound, err := repo.Comment().GetBySlackTS(ctx, wsID, ticketID, "9999999999.999999")
		if err != nil {
			t.Fatalf("GetBySlackTS (not found) failed: %v", err)
		}
		if notFound != nil {
			t.Error("expected nil for non-existent slack ts")
		}
	})
}

func TestTokenCRUD(t *testing.T) {
	runTest(t, "TokenCRUD", func(t *testing.T, repo interfaces.Repository) {
		ctx := context.Background()
		token := auth.NewToken("U_TEST", "test@example.com", "Test User")

		if err := repo.PutToken(ctx, token); err != nil {
			t.Fatalf("PutToken failed: %v", err)
		}

		got, err := repo.GetToken(ctx, token.ID)
		if err != nil {
			t.Fatalf("GetToken failed: %v", err)
		}
		if got.Sub != "U_TEST" {
			t.Errorf("expected sub 'U_TEST', got %q", got.Sub)
		}
		if got.Email != "test@example.com" {
			t.Errorf("expected email 'test@example.com', got %q", got.Email)
		}

		if err := repo.DeleteToken(ctx, token.ID); err != nil {
			t.Fatalf("DeleteToken failed: %v", err)
		}

		_, err = repo.GetToken(ctx, token.ID)
		if err == nil {
			t.Error("expected error after delete, got nil")
		}
	})
}
