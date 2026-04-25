package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/m-mizutani/gt"
	"github.com/m-mizutani/shepherd/pkg/domain/interfaces"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
)

func createTestTicket(t *testing.T, repo interfaces.Repository, wsID string) *model.Ticket {
	t.Helper()
	now := time.Now().Truncate(time.Millisecond)
	ticket := &model.Ticket{
		ID:          uuid.Must(uuid.NewV7()).String(),
		WorkspaceID: wsID,
		Title:       "History Test Ticket",
		StatusID:    "open",
		FieldValues: make(map[types.FieldID]model.FieldValue),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	return gt.R1(repo.Ticket().Create(ctx(t), wsID, ticket)).NoError(t)
}

func ctx(t *testing.T) context.Context {
	t.Helper()
	return context.Background()
}

func TestTicketHistoryCreate(t *testing.T) {
	runTest(t, "TicketHistoryCreate", func(t *testing.T, repo interfaces.Repository) {
		wsID := "test-hist-" + uuid.Must(uuid.NewV7()).String()[:8]
		ticket := createTestTicket(t, repo, wsID)

		history := &model.TicketHistory{
			ID:        uuid.Must(uuid.NewV7()).String(),
			StatusID:  "open",
			ChangedBy: "U123",
			Action:    "created",
			CreatedAt: time.Now().Truncate(time.Millisecond),
		}
		created := gt.R1(repo.TicketHistory().Create(ctx(t), wsID, ticket.ID, history)).NoError(t)
		gt.S(t, created.Action).Equal("created")
		gt.S(t, created.StatusID).Equal("open")
		gt.S(t, created.ChangedBy).Equal("U123")
	})
}

func TestTicketHistoryList(t *testing.T) {
	runTest(t, "TicketHistoryList", func(t *testing.T, repo interfaces.Repository) {
		wsID := "test-hlist-" + uuid.Must(uuid.NewV7()).String()[:8]
		ticket := createTestTicket(t, repo, wsID)
		now := time.Now().Truncate(time.Millisecond)

		for i, action := range []string{"created", "changed", "changed"} {
			h := &model.TicketHistory{
				ID:        uuid.Must(uuid.NewV7()).String(),
				StatusID:  "status-" + action,
				ChangedBy: "U123",
				Action:    action,
				CreatedAt: now.Add(time.Duration(i) * time.Second),
			}
			if action == "changed" {
				h.OldStatusID = "open"
			}
			gt.R1(repo.TicketHistory().Create(ctx(t), wsID, ticket.ID, h)).NoError(t)
		}

		histories := gt.R1(repo.TicketHistory().List(ctx(t), wsID, ticket.ID)).NoError(t)
		gt.A(t, histories).Length(3)
		gt.S(t, histories[0].Action).Equal("created")
		gt.S(t, histories[1].Action).Equal("changed")
		gt.S(t, histories[2].Action).Equal("changed")

		// Verify chronological order
		for i := 1; i < len(histories); i++ {
			gt.B(t, !histories[i].CreatedAt.Before(histories[i-1].CreatedAt)).True()
		}
	})
}

func TestTicketHistoryList_Empty(t *testing.T) {
	runTest(t, "TicketHistoryListEmpty", func(t *testing.T, repo interfaces.Repository) {
		wsID := "test-hempty-" + uuid.Must(uuid.NewV7()).String()[:8]
		ticket := createTestTicket(t, repo, wsID)

		histories := gt.R1(repo.TicketHistory().List(ctx(t), wsID, ticket.ID)).NoError(t)
		gt.A(t, histories).Length(0)
	})
}

func TestTicketInitialMessage(t *testing.T) {
	runTest(t, "TicketInitialMessage", func(t *testing.T, repo interfaces.Repository) {
		wsID := "test-initmsg-" + uuid.Must(uuid.NewV7()).String()[:8]
		now := time.Now().Truncate(time.Millisecond)

		ticket := &model.Ticket{
			ID:             uuid.Must(uuid.NewV7()).String(),
			WorkspaceID:    wsID,
			Title:          "Initial Message Test",
			InitialMessage: "original message text",
			StatusID:       "open",
			FieldValues:    make(map[types.FieldID]model.FieldValue),
			CreatedAt:      now,
			UpdatedAt:      now,
		}
		gt.R1(repo.Ticket().Create(ctx(t), wsID, ticket)).NoError(t)

		got := gt.R1(repo.Ticket().Get(ctx(t), wsID, ticket.ID)).NoError(t)
		gt.S(t, got.InitialMessage).Equal("original message text")

		got.InitialMessage = "edited message text"
		gt.R1(repo.Ticket().Update(ctx(t), wsID, got)).NoError(t)

		updated := gt.R1(repo.Ticket().Get(ctx(t), wsID, ticket.ID)).NoError(t)
		gt.S(t, updated.InitialMessage).Equal("edited message text")
	})
}

func TestCommentIsBot(t *testing.T) {
	runTest(t, "CommentIsBot", func(t *testing.T, repo interfaces.Repository) {
		wsID := "test-cbot-" + uuid.Must(uuid.NewV7()).String()[:8]
		ticket := createTestTicket(t, repo, wsID)

		humanComment := &model.Comment{
			ID:          uuid.Must(uuid.NewV7()).String(),
			TicketID:    ticket.ID,
			SlackUserID: "U123",
			IsBot:       false,
			Body:        "human message",
			SlackTS:     "1111111111.111111",
			CreatedAt:   time.Now().Truncate(time.Millisecond),
		}
		gt.R1(repo.Comment().Create(ctx(t), wsID, ticket.ID, humanComment)).NoError(t)

		botComment := &model.Comment{
			ID:          uuid.Must(uuid.NewV7()).String(),
			TicketID:    ticket.ID,
			SlackUserID: "B456",
			IsBot:       true,
			Body:        "bot message",
			SlackTS:     "2222222222.222222",
			CreatedAt:   time.Now().Add(time.Second).Truncate(time.Millisecond),
		}
		gt.R1(repo.Comment().Create(ctx(t), wsID, ticket.ID, botComment)).NoError(t)

		comments := gt.R1(repo.Comment().List(ctx(t), wsID, ticket.ID)).NoError(t)
		gt.A(t, comments).Length(2)
		gt.B(t, comments[0].IsBot).False()
		gt.B(t, comments[1].IsBot).True()
		gt.S(t, comments[1].Body).Equal("bot message")
	})
}
