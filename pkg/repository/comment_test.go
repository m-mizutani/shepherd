package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/m-mizutani/gt"
	"github.com/m-mizutani/shepherd/pkg/domain/interfaces"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
	"github.com/m-mizutani/shepherd/pkg/domain/model/auth"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
)

func TestCommentCreate(t *testing.T) {
	runTest(t, "CommentCreate", func(t *testing.T, repo interfaces.Repository) {
		ctx := context.Background()
		now := time.Now().Truncate(time.Millisecond)
		wsID := types.WorkspaceID("test-comment-" + uuid.Must(uuid.NewV7()).String()[:8])
		ticketID := types.TicketID(uuid.Must(uuid.NewV7()).String())

		ticket := &model.Ticket{
			ID:          ticketID,
			WorkspaceID: wsID,
			Title:       "Comment Test Ticket",
			StatusID:    "open",
			FieldValues: make(map[string]model.FieldValue),
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		gt.R1(repo.Ticket().Create(ctx, wsID, ticket)).NoError(t)

		comment := &model.Comment{
			ID:          types.CommentID(uuid.Must(uuid.NewV7()).String()),
			TicketID:    ticketID,
			SlackUserID: "U123",
			Body:        "Hello world",
			SlackTS:     "1111111111.111111",
			CreatedAt:   now,
		}
		created := gt.R1(repo.Comment().Create(ctx, wsID, ticketID, comment)).NoError(t)
		gt.S(t, created.Body).Equal("Hello world")
	})
}

func TestCommentList(t *testing.T) {
	runTest(t, "CommentList", func(t *testing.T, repo interfaces.Repository) {
		ctx := context.Background()
		now := time.Now().Truncate(time.Millisecond)
		wsID := types.WorkspaceID("test-clist-" + uuid.Must(uuid.NewV7()).String()[:8])
		ticketID := types.TicketID(uuid.Must(uuid.NewV7()).String())

		ticket := &model.Ticket{
			ID:          ticketID,
			WorkspaceID: wsID,
			Title:       "Comment List Test",
			StatusID:    "open",
			FieldValues: make(map[string]model.FieldValue),
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		gt.R1(repo.Ticket().Create(ctx, wsID, ticket)).NoError(t)

		for i, body := range []string{"First", "Second", "Third"} {
			comment := &model.Comment{
				ID:          types.CommentID(uuid.Must(uuid.NewV7()).String()),
				TicketID:    ticketID,
				SlackUserID: "U123",
				Body:        body,
				SlackTS:     types.SlackThreadTS("1111111111.11111" + string(rune('0'+i))),
				CreatedAt:   now.Add(time.Duration(i) * time.Second),
			}
			gt.R1(repo.Comment().Create(ctx, wsID, ticketID, comment)).NoError(t)
		}

		comments := gt.R1(repo.Comment().List(ctx, wsID, ticketID)).NoError(t)
		gt.A(t, comments).Length(3)
	})
}

func TestCommentGetBySlackTS(t *testing.T) {
	runTest(t, "CommentGetBySlackTS", func(t *testing.T, repo interfaces.Repository) {
		ctx := context.Background()
		now := time.Now().Truncate(time.Millisecond)
		wsID := types.WorkspaceID("test-cslack-" + uuid.Must(uuid.NewV7()).String()[:8])
		ticketID := types.TicketID(uuid.Must(uuid.NewV7()).String())

		ticket := &model.Ticket{
			ID:          ticketID,
			WorkspaceID: wsID,
			Title:       "Comment Slack TS Test",
			StatusID:    "open",
			FieldValues: make(map[string]model.FieldValue),
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		gt.R1(repo.Ticket().Create(ctx, wsID, ticket)).NoError(t)

		slackTS := types.SlackThreadTS("2222222222.222222")
		comment := &model.Comment{
			ID:          types.CommentID(uuid.Must(uuid.NewV7()).String()),
			TicketID:    ticketID,
			SlackUserID: "U456",
			Body:        "Slack TS test comment",
			SlackTS:     slackTS,
			CreatedAt:   now,
		}
		gt.R1(repo.Comment().Create(ctx, wsID, ticketID, comment)).NoError(t)

		got := gt.R1(repo.Comment().GetBySlackTS(ctx, wsID, ticketID, slackTS)).NoError(t)
		gt.V(t, got).NotNil().Required()
		gt.S(t, got.Body).Equal("Slack TS test comment")

		notFound := gt.R1(repo.Comment().GetBySlackTS(ctx, wsID, ticketID, "9999999999.999999")).NoError(t)
		gt.V(t, notFound).Nil()
	})
}

func TestTokenCRUD(t *testing.T) {
	runTest(t, "TokenCRUD", func(t *testing.T, repo interfaces.Repository) {
		ctx := context.Background()
		token := auth.NewToken("U_TEST", "test@example.com", "Test User")

		gt.NoError(t, repo.PutToken(ctx, token))

		got := gt.R1(repo.GetToken(ctx, token.ID)).NoError(t)
		gt.S(t, got.Sub).Equal("U_TEST")
		gt.S(t, got.Email).Equal("test@example.com")

		gt.NoError(t, repo.DeleteToken(ctx, token.ID))

		_, err := repo.GetToken(ctx, token.ID)
		gt.Error(t, err)
	})
}
