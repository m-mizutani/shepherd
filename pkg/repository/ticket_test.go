package repository_test

import (
	"context"
	"testing"
	"time"

	"cloud.google.com/go/firestore"
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
			ID:          types.TicketID(uuid.Must(uuid.NewV7()).String()),
			WorkspaceID: "test-ws",
			Title:       "Test Ticket",
			Description: "Test Description",
			StatusID:    "open",
			FieldValues: map[string]model.FieldValue{
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
		id := types.TicketID(uuid.Must(uuid.NewV7()).String())

		ticket := &model.Ticket{
			ID:          id,
			WorkspaceID: "test-ws",
			Title:       "Get Test",
			StatusID:    "open",
			FieldValues: make(map[string]model.FieldValue),
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		gt.R1(repo.Ticket().Create(ctx, "test-ws", ticket)).NoError(t)

		got := gt.R1(repo.Ticket().Get(ctx, "test-ws", id)).NoError(t)
		gt.S(t, got.Title).Equal("Get Test")
		gt.S(t, string(got.StatusID)).Equal("open")
	})
}

func TestTicketList(t *testing.T) {
	runTest(t, "List", func(t *testing.T, repo interfaces.Repository) {
		ctx := context.Background()
		now := time.Now().Truncate(time.Millisecond)
		wsID := types.WorkspaceID("test-list-" + uuid.Must(uuid.NewV7()).String()[:8])

		for i, title := range []string{"Ticket A", "Ticket B", "Ticket C"} {
			ticket := &model.Ticket{
				ID:          types.TicketID(uuid.Must(uuid.NewV7()).String()),
				WorkspaceID: wsID,
				Title:       title,
				StatusID:    "open",
				FieldValues: make(map[string]model.FieldValue),
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
		id := types.TicketID(uuid.Must(uuid.NewV7()).String())

		ticket := &model.Ticket{
			ID:          id,
			WorkspaceID: "test-ws",
			Title:       "Before Update",
			StatusID:    "open",
			FieldValues: make(map[string]model.FieldValue),
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
		gt.S(t, string(got.StatusID)).Equal("in-progress")
	})
}

func TestTicketDelete(t *testing.T) {
	runTest(t, "Delete", func(t *testing.T, repo interfaces.Repository) {
		ctx := context.Background()
		now := time.Now().Truncate(time.Millisecond)
		id := types.TicketID(uuid.Must(uuid.NewV7()).String())

		ticket := &model.Ticket{
			ID:          id,
			WorkspaceID: "test-ws",
			Title:       "To Delete",
			StatusID:    "open",
			FieldValues: make(map[string]model.FieldValue),
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
		wsID := types.WorkspaceID("test-seq-" + uuid.Must(uuid.NewV7()).String()[:8])

		var seqNums []int64
		for i := 0; i < 3; i++ {
			ticket := &model.Ticket{
				ID:          types.TicketID(uuid.Must(uuid.NewV7()).String()),
				WorkspaceID: wsID,
				Title:       "Seq Test",
				StatusID:    "open",
				FieldValues: make(map[string]model.FieldValue),
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
		wsID := types.WorkspaceID("test-slack-" + uuid.Must(uuid.NewV7()).String()[:8])

		ticket := &model.Ticket{
			ID:             types.TicketID(uuid.Must(uuid.NewV7()).String()),
			WorkspaceID:    wsID,
			Title:          "Slack Thread Test",
			StatusID:       "open",
			SlackChannelID: "C123456",
			SlackThreadTS:  "1234567890.123456",
			FieldValues:    make(map[string]model.FieldValue),
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

func TestTicketUpdateEmbedding(t *testing.T) {
	runTest(t, "UpdateEmbedding", func(t *testing.T, repo interfaces.Repository) {
		ctx := context.Background()
		now := time.Now().Truncate(time.Millisecond)
		wsID := types.WorkspaceID("test-ws-embedding")
		id := types.TicketID(uuid.Must(uuid.NewV7()).String())

		original := &model.Ticket{
			ID:          id,
			WorkspaceID: wsID,
			Title:       "Embed me",
			Description: "the original body",
			StatusID:    "open",
			FieldValues: make(map[string]model.FieldValue),
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		gt.R1(repo.Ticket().Create(ctx, wsID, original)).NoError(t)

		vec := firestore.Vector32{0.1, 0.2, 0.3, 0.4}
		gt.NoError(t, repo.Ticket().UpdateEmbedding(ctx, wsID, id, vec, "gemini:test-model"))

		got := gt.R1(repo.Ticket().Get(ctx, wsID, id)).NoError(t)
		gt.S(t, got.Title).Equal("Embed me")
		gt.S(t, got.Description).Equal("the original body")
		gt.S(t, got.EmbeddingModel).Equal("gemini:test-model")
		gt.A(t, got.Embedding).Length(len(vec))
		for i, v := range vec {
			gt.Equal(t, v, got.Embedding[i])
		}
	})
}

func TestTicketFindSimilar(t *testing.T) {
	runTest(t, "FindSimilar", func(t *testing.T, repo interfaces.Repository) {
		ctx := context.Background()
		now := time.Now().Truncate(time.Millisecond)
		wsID := types.WorkspaceID("test-ws-similar-" + uuid.Must(uuid.NewV7()).String())

		makeTicket := func(title string, status types.StatusID, vec firestore.Vector32) types.TicketID {
			id := types.TicketID(uuid.Must(uuid.NewV7()).String())
			tk := &model.Ticket{
				ID:             id,
				WorkspaceID:    wsID,
				Title:          title,
				StatusID:       status,
				FieldValues:    make(map[string]model.FieldValue),
				Embedding:      vec,
				EmbeddingModel: "gemini:test-model",
				CreatedAt:      now,
				UpdatedAt:      now,
			}
			gt.R1(repo.Ticket().Create(ctx, wsID, tk)).NoError(t)
			return id
		}
		nearID := makeTicket("near", "open", firestore.Vector32{1, 0, 0, 0})
		mediumID := makeTicket("medium", "open", firestore.Vector32{0.7, 0.7, 0, 0})
		farID := makeTicket("far", "closed", firestore.Vector32{0, 1, 0, 0})

		// Firestore requires a COLLECTION_GROUP vector index on Embedding;
		// when missing, FindNearest returns an error. Skip the assertions in
		// that case so the contract test still verifies the memory backend.
		query := firestore.Vector32{1, 0, 0, 0}
		got, err := repo.Ticket().FindSimilar(ctx, wsID, query, 3, nil)
		if err != nil {
			t.Skipf("FindSimilar unavailable: %v", err)
		}
		gt.A(t, got).Length(3)
		gt.S(t, string(got[0].Ticket.ID)).Equal(string(nearID))
		gt.S(t, string(got[1].Ticket.ID)).Equal(string(mediumID))
		gt.S(t, string(got[2].Ticket.ID)).Equal(string(farID))
		gt.N(t, got[0].Distance).LessOrEqual(got[1].Distance)
		gt.N(t, got[1].Distance).LessOrEqual(got[2].Distance)

		filtered, err := repo.Ticket().FindSimilar(ctx, wsID, query, 3, []types.StatusID{"open"})
		if err != nil {
			t.Skipf("FindSimilar unavailable: %v", err)
		}
		gt.A(t, filtered).Length(2)
		for _, r := range filtered {
			gt.S(t, string(r.Ticket.StatusID)).Equal("open")
		}
	})
}
