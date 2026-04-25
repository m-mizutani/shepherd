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
			FieldValues: map[types.FieldID]model.FieldValue{
				"priority": {FieldID: types.FieldID("priority"), Type: types.FieldTypeSelect, Value: "high"},
			},
			CreatedAt: now,
			UpdatedAt: now,
		}

		created := gt.R1(repo.Ticket().Create(ctx, "test-ws", ticket)).NoError(t)
		gt.N(t, created.SeqNum).GreaterOrEqual(int64(1))
		gt.S(t, created.Title).Equal("Test Ticket")
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
			FieldValues: make(map[types.FieldID]model.FieldValue),
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		gt.R1(repo.Ticket().Create(ctx, "test-ws", ticket)).NoError(t)

		got := gt.R1(repo.Ticket().Get(ctx, "test-ws", id)).NoError(t)
		gt.S(t, got.Title).Equal("Get Test")
		gt.S(t, got.StatusID).Equal("open")
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
				FieldValues: make(map[types.FieldID]model.FieldValue),
				CreatedAt:   now.Add(time.Duration(i) * time.Second),
				UpdatedAt:   now.Add(time.Duration(i) * time.Second),
			}
			gt.R1(repo.Ticket().Create(ctx, wsID, ticket)).NoError(t)
		}

		tickets := gt.R1(repo.Ticket().List(ctx, wsID, nil)).NoError(t)
		gt.A(t, tickets).Length(3)
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
			FieldValues: make(map[types.FieldID]model.FieldValue),
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		gt.R1(repo.Ticket().Create(ctx, "test-ws", ticket)).NoError(t)

		ticket.Title = "After Update"
		ticket.StatusID = "in-progress"
		ticket.UpdatedAt = now.Add(time.Minute)

		updated := gt.R1(repo.Ticket().Update(ctx, "test-ws", ticket)).NoError(t)
		gt.S(t, updated.Title).Equal("After Update")

		got := gt.R1(repo.Ticket().Get(ctx, "test-ws", id)).NoError(t)
		gt.S(t, got.StatusID).Equal("in-progress")
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
			FieldValues: make(map[types.FieldID]model.FieldValue),
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		gt.R1(repo.Ticket().Create(ctx, "test-ws", ticket)).NoError(t)
		gt.NoError(t, repo.Ticket().Delete(ctx, "test-ws", id))

		_, err := repo.Ticket().Get(ctx, "test-ws", id)
		gt.Error(t, err)
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
				FieldValues: make(map[types.FieldID]model.FieldValue),
				CreatedAt:   now,
				UpdatedAt:   now,
			}
			created := gt.R1(repo.Ticket().Create(ctx, wsID, ticket)).NoError(t)
			seqNums = append(seqNums, created.SeqNum)
		}

		for i := 1; i < len(seqNums); i++ {
			gt.N(t, seqNums[i]).Equal(seqNums[i-1] + 1)
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
			FieldValues:    make(map[types.FieldID]model.FieldValue),
			CreatedAt:      now,
			UpdatedAt:      now,
		}
		gt.R1(repo.Ticket().Create(ctx, wsID, ticket)).NoError(t)

		got := gt.R1(repo.Ticket().GetBySlackThreadTS(ctx, wsID, "C123456", "1234567890.123456")).NoError(t)
		gt.V(t, got).NotNil().Required()
		gt.S(t, got.Title).Equal("Slack Thread Test")

		notFound := gt.R1(repo.Ticket().GetBySlackThreadTS(ctx, wsID, "C999999", "9999999999.999999")).NoError(t)
		gt.V(t, notFound).Nil()
	})
}
