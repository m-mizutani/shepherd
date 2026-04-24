package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/m-mizutani/shepherd/pkg/domain/interfaces"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
)

func TestTicketCreate(t *testing.T) {
	runTest(t, "Create", func(t *testing.T, repo interfaces.Repository) {
		ctx := context.Background()
		now := time.Now().Truncate(time.Millisecond)

		ticket := &model.Ticket{
			ID:          uuid.Must(uuid.NewV7()).String(),
			WorkspaceID: "test-ws",
			Title:       "Test Ticket",
			Description: "Test Description",
			StatusID:    "open",
			FieldValues: map[string]model.FieldValue{
				"priority": {FieldID: "priority", Type: types.FieldTypeSelect, Value: "high"},
			},
			CreatedAt: now,
			UpdatedAt: now,
		}

		created, err := repo.Ticket().Create(ctx, "test-ws", ticket)
		if err != nil {
			t.Fatalf("Create failed: %v", err)
		}

		if created.SeqNum < 1 {
			t.Errorf("expected SeqNum >= 1, got %d", created.SeqNum)
		}
		if created.Title != "Test Ticket" {
			t.Errorf("expected title 'Test Ticket', got %q", created.Title)
		}
	})
}

func TestTicketGet(t *testing.T) {
	runTest(t, "Get", func(t *testing.T, repo interfaces.Repository) {
		ctx := context.Background()
		now := time.Now().Truncate(time.Millisecond)
		id := uuid.Must(uuid.NewV7()).String()

		ticket := &model.Ticket{
			ID:          id,
			WorkspaceID: "test-ws",
			Title:       "Get Test",
			StatusID:    "open",
			FieldValues: make(map[string]model.FieldValue),
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if _, err := repo.Ticket().Create(ctx, "test-ws", ticket); err != nil {
			t.Fatalf("Create failed: %v", err)
		}

		got, err := repo.Ticket().Get(ctx, "test-ws", id)
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		if got.Title != "Get Test" {
			t.Errorf("expected title 'Get Test', got %q", got.Title)
		}
		if got.StatusID != "open" {
			t.Errorf("expected statusID 'open', got %q", got.StatusID)
		}
	})
}

func TestTicketList(t *testing.T) {
	runTest(t, "List", func(t *testing.T, repo interfaces.Repository) {
		ctx := context.Background()
		now := time.Now().Truncate(time.Millisecond)
		wsID := "test-list-" + uuid.Must(uuid.NewV7()).String()[:8]

		for i, title := range []string{"Ticket A", "Ticket B", "Ticket C"} {
			ticket := &model.Ticket{
				ID:          uuid.Must(uuid.NewV7()).String(),
				WorkspaceID: wsID,
				Title:       title,
				StatusID:    "open",
				FieldValues: make(map[string]model.FieldValue),
				CreatedAt:   now.Add(time.Duration(i) * time.Second),
				UpdatedAt:   now.Add(time.Duration(i) * time.Second),
			}
			if _, err := repo.Ticket().Create(ctx, wsID, ticket); err != nil {
				t.Fatalf("Create failed: %v", err)
			}
		}

		tickets, err := repo.Ticket().List(ctx, wsID, nil)
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}
		if len(tickets) != 3 {
			t.Errorf("expected 3 tickets, got %d", len(tickets))
		}
	})
}

func TestTicketUpdate(t *testing.T) {
	runTest(t, "Update", func(t *testing.T, repo interfaces.Repository) {
		ctx := context.Background()
		now := time.Now().Truncate(time.Millisecond)
		id := uuid.Must(uuid.NewV7()).String()

		ticket := &model.Ticket{
			ID:          id,
			WorkspaceID: "test-ws",
			Title:       "Before Update",
			StatusID:    "open",
			FieldValues: make(map[string]model.FieldValue),
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if _, err := repo.Ticket().Create(ctx, "test-ws", ticket); err != nil {
			t.Fatalf("Create failed: %v", err)
		}

		ticket.Title = "After Update"
		ticket.StatusID = "in-progress"
		ticket.UpdatedAt = now.Add(time.Minute)

		updated, err := repo.Ticket().Update(ctx, "test-ws", ticket)
		if err != nil {
			t.Fatalf("Update failed: %v", err)
		}
		if updated.Title != "After Update" {
			t.Errorf("expected title 'After Update', got %q", updated.Title)
		}

		got, err := repo.Ticket().Get(ctx, "test-ws", id)
		if err != nil {
			t.Fatalf("Get after update failed: %v", err)
		}
		if got.StatusID != "in-progress" {
			t.Errorf("expected statusID 'in-progress', got %q", got.StatusID)
		}
	})
}

func TestTicketDelete(t *testing.T) {
	runTest(t, "Delete", func(t *testing.T, repo interfaces.Repository) {
		ctx := context.Background()
		now := time.Now().Truncate(time.Millisecond)
		id := uuid.Must(uuid.NewV7()).String()

		ticket := &model.Ticket{
			ID:          id,
			WorkspaceID: "test-ws",
			Title:       "To Delete",
			StatusID:    "open",
			FieldValues: make(map[string]model.FieldValue),
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if _, err := repo.Ticket().Create(ctx, "test-ws", ticket); err != nil {
			t.Fatalf("Create failed: %v", err)
		}

		if err := repo.Ticket().Delete(ctx, "test-ws", id); err != nil {
			t.Fatalf("Delete failed: %v", err)
		}

		_, err := repo.Ticket().Get(ctx, "test-ws", id)
		if err == nil {
			t.Error("expected error after delete, got nil")
		}
	})
}

func TestTicketSeqNumIncrement(t *testing.T) {
	runTest(t, "SeqNumIncrement", func(t *testing.T, repo interfaces.Repository) {
		ctx := context.Background()
		now := time.Now().Truncate(time.Millisecond)
		wsID := "test-seq-" + uuid.Must(uuid.NewV7()).String()[:8]

		var seqNums []int64
		for i := 0; i < 3; i++ {
			ticket := &model.Ticket{
				ID:          uuid.Must(uuid.NewV7()).String(),
				WorkspaceID: wsID,
				Title:       "Seq Test",
				StatusID:    "open",
				FieldValues: make(map[string]model.FieldValue),
				CreatedAt:   now,
				UpdatedAt:   now,
			}
			created, err := repo.Ticket().Create(ctx, wsID, ticket)
			if err != nil {
				t.Fatalf("Create #%d failed: %v", i, err)
			}
			seqNums = append(seqNums, created.SeqNum)
		}

		for i := 1; i < len(seqNums); i++ {
			if seqNums[i] != seqNums[i-1]+1 {
				t.Errorf("expected SeqNum %d to be %d+1, got %d", i, seqNums[i-1], seqNums[i])
			}
		}
	})
}

func TestTicketGetBySlackThreadTS(t *testing.T) {
	runTest(t, "GetBySlackThreadTS", func(t *testing.T, repo interfaces.Repository) {
		ctx := context.Background()
		now := time.Now().Truncate(time.Millisecond)
		wsID := "test-slack-" + uuid.Must(uuid.NewV7()).String()[:8]

		ticket := &model.Ticket{
			ID:             uuid.Must(uuid.NewV7()).String(),
			WorkspaceID:    wsID,
			Title:          "Slack Thread Test",
			StatusID:       "open",
			SlackChannelID: "C123456",
			SlackThreadTS:  "1234567890.123456",
			FieldValues:    make(map[string]model.FieldValue),
			CreatedAt:      now,
			UpdatedAt:      now,
		}
		if _, err := repo.Ticket().Create(ctx, wsID, ticket); err != nil {
			t.Fatalf("Create failed: %v", err)
		}

		got, err := repo.Ticket().GetBySlackThreadTS(ctx, wsID, "C123456", "1234567890.123456")
		if err != nil {
			t.Fatalf("GetBySlackThreadTS failed: %v", err)
		}
		if got == nil {
			t.Fatal("expected ticket, got nil")
		}
		if got.Title != "Slack Thread Test" {
			t.Errorf("expected title 'Slack Thread Test', got %q", got.Title)
		}

		notFound, err := repo.Ticket().GetBySlackThreadTS(ctx, wsID, "C999999", "9999999999.999999")
		if err != nil {
			t.Fatalf("GetBySlackThreadTS (not found) failed: %v", err)
		}
		if notFound != nil {
			t.Error("expected nil for non-existent thread ts")
		}
	})
}
