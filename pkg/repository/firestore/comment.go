package firestore

import (
	"context"

	"cloud.google.com/go/firestore"
	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
	"google.golang.org/api/iterator"
)

type commentRepository struct {
	client *firestore.Client
}

func (r *commentRepository) commentsCollection(workspaceID, ticketID string) *firestore.CollectionRef {
	return r.client.Collection("workspaces").Doc(workspaceID).Collection("tickets").Doc(ticketID).Collection("comments")
}

func (r *commentRepository) Create(ctx context.Context, workspaceID, ticketID string, c *model.Comment) (*model.Comment, error) {
	ref := r.commentsCollection(workspaceID, ticketID).Doc(c.ID)
	if _, err := ref.Set(ctx, map[string]any{
		"ticket_id":     c.TicketID,
		"slack_user_id": c.SlackUserID,
		"body":          c.Body,
		"slack_ts":      c.SlackTS,
		"created_at":    c.CreatedAt,
	}); err != nil {
		return nil, goerr.Wrap(err, "failed to create comment")
	}
	return c, nil
}

func (r *commentRepository) List(ctx context.Context, workspaceID, ticketID string) ([]*model.Comment, error) {
	iter := r.commentsCollection(workspaceID, ticketID).OrderBy("created_at", firestore.Asc).Documents(ctx)
	defer iter.Stop()

	var comments []*model.Comment
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, goerr.Wrap(err, "failed to iterate comments")
		}
		comments = append(comments, mapToComment(doc.Data(), doc.Ref.ID, ticketID))
	}
	return comments, nil
}

func (r *commentRepository) GetBySlackTS(ctx context.Context, workspaceID, ticketID, slackTS string) (*model.Comment, error) {
	iter := r.commentsCollection(workspaceID, ticketID).
		Where("slack_ts", "==", slackTS).
		Limit(1).
		Documents(ctx)
	defer iter.Stop()

	doc, err := iter.Next()
	if err == iterator.Done {
		return nil, nil
	}
	if err != nil {
		return nil, goerr.Wrap(err, "failed to query comment by slack ts")
	}
	return mapToComment(doc.Data(), doc.Ref.ID, ticketID), nil
}

func mapToComment(data map[string]any, id, ticketID string) *model.Comment {
	c := &model.Comment{
		ID:       id,
		TicketID: ticketID,
	}
	if v, ok := data["slack_user_id"]; ok {
		c.SlackUserID = v.(string)
	}
	if v, ok := data["body"]; ok {
		c.Body = v.(string)
	}
	if v, ok := data["slack_ts"]; ok {
		c.SlackTS = v.(string)
	}
	if v, ok := data["created_at"]; ok {
		c.CreatedAt = toTime(v)
	}
	return c
}
