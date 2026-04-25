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
	if _, err := ref.Set(ctx, c); err != nil {
		return nil, goerr.Wrap(err, "failed to create comment")
	}
	return c, nil
}

func (r *commentRepository) List(ctx context.Context, workspaceID, ticketID string) ([]*model.Comment, error) {
	iter := r.commentsCollection(workspaceID, ticketID).OrderBy("CreatedAt", firestore.Asc).Documents(ctx)
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
		var c model.Comment
		if err := doc.DataTo(&c); err != nil {
			return nil, goerr.Wrap(err, "failed to decode comment")
		}
		c.ID = doc.Ref.ID
		c.TicketID = ticketID
		comments = append(comments, &c)
	}
	return comments, nil
}

func (r *commentRepository) GetBySlackTS(ctx context.Context, workspaceID, ticketID, slackTS string) (*model.Comment, error) {
	iter := r.commentsCollection(workspaceID, ticketID).
		Where("SlackTS", "==", slackTS).
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
	var c model.Comment
	if err := doc.DataTo(&c); err != nil {
		return nil, goerr.Wrap(err, "failed to decode comment")
	}
	c.ID = doc.Ref.ID
	c.TicketID = ticketID
	return &c, nil
}
