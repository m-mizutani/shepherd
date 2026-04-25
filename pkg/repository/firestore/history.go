package firestore

import (
	"context"

	"cloud.google.com/go/firestore"
	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
	"google.golang.org/api/iterator"
)

type ticketHistoryRepository struct {
	client *firestore.Client
}

func (r *ticketHistoryRepository) historyCollection(workspaceID types.WorkspaceID, ticketID types.TicketID) *firestore.CollectionRef {
	return r.client.Collection("workspaces").Doc(string(workspaceID)).Collection("tickets").Doc(string(ticketID)).Collection("history")
}

func (r *ticketHistoryRepository) Create(ctx context.Context, workspaceID types.WorkspaceID, ticketID types.TicketID, h *model.TicketHistory) (*model.TicketHistory, error) {
	ref := r.historyCollection(workspaceID, ticketID).Doc(h.ID)
	if _, err := ref.Set(ctx, h); err != nil {
		return nil, goerr.Wrap(err, "failed to create ticket history")
	}
	return h, nil
}

func (r *ticketHistoryRepository) List(ctx context.Context, workspaceID types.WorkspaceID, ticketID types.TicketID) ([]*model.TicketHistory, error) {
	iter := r.historyCollection(workspaceID, ticketID).OrderBy("CreatedAt", firestore.Asc).Documents(ctx)
	defer iter.Stop()

	var histories []*model.TicketHistory
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, goerr.Wrap(err, "failed to iterate ticket history")
		}
		var h model.TicketHistory
		if err := doc.DataTo(&h); err != nil {
			return nil, goerr.Wrap(err, "failed to decode ticket history")
		}
		h.ID = doc.Ref.ID
		histories = append(histories, &h)
	}
	return histories, nil
}
