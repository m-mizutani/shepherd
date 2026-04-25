package firestore

import (
	"context"

	"cloud.google.com/go/firestore"
	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
	"google.golang.org/api/iterator"
)

type ticketRepository struct {
	client *firestore.Client
}

func (r *ticketRepository) ticketsCollection(workspaceID string) *firestore.CollectionRef {
	return r.client.Collection("workspaces").Doc(workspaceID).Collection("tickets")
}

func (r *ticketRepository) counterRef(workspaceID string) *firestore.DocumentRef {
	return r.client.Collection("workspaces").Doc(workspaceID).Collection("counters").Doc("ticket")
}

func (r *ticketRepository) getNextSeqNum(ctx context.Context, workspaceID string) (int64, error) {
	var seqNum int64
	err := r.client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		ref := r.counterRef(workspaceID)
		doc, err := tx.Get(ref)
		if err != nil {
			if isNotFound(err) {
				seqNum = 1
				return tx.Set(ref, map[string]any{"value": int64(1)})
			}
			return err
		}
		data := doc.Data()
		seqNum = data["value"].(int64) + 1
		return tx.Update(ref, []firestore.Update{{Path: "value", Value: seqNum}})
	})
	if err != nil {
		return 0, goerr.Wrap(err, "failed to get next seq num")
	}
	return seqNum, nil
}

func (r *ticketRepository) Create(ctx context.Context, workspaceID string, t *model.Ticket) (*model.Ticket, error) {
	seqNum, err := r.getNextSeqNum(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	t.SeqNum = seqNum

	ref := r.ticketsCollection(workspaceID).Doc(t.ID)
	if _, err := ref.Set(ctx, t); err != nil {
		return nil, goerr.Wrap(err, "failed to create ticket")
	}
	return t, nil
}

func (r *ticketRepository) Get(ctx context.Context, workspaceID string, id string) (*model.Ticket, error) {
	doc, err := r.ticketsCollection(workspaceID).Doc(id).Get(ctx)
	if err != nil {
		if isNotFound(err) {
			return nil, goerr.New("ticket not found", goerr.V("ticket_id", id))
		}
		return nil, goerr.Wrap(err, "failed to get ticket")
	}
	var t model.Ticket
	if err := doc.DataTo(&t); err != nil {
		return nil, goerr.Wrap(err, "failed to decode ticket")
	}
	t.ID = id
	t.WorkspaceID = workspaceID
	return &t, nil
}

func (r *ticketRepository) List(ctx context.Context, workspaceID string, statusIDs []string) ([]*model.Ticket, error) {
	query := r.ticketsCollection(workspaceID).OrderBy("CreatedAt", firestore.Desc)

	if len(statusIDs) > 0 {
		query = r.ticketsCollection(workspaceID).Where("StatusID", "in", statusIDs).OrderBy("CreatedAt", firestore.Desc)
	}

	iter := query.Documents(ctx)
	defer iter.Stop()

	var tickets []*model.Ticket
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, goerr.Wrap(err, "failed to iterate tickets")
		}
		var t model.Ticket
		if err := doc.DataTo(&t); err != nil {
			return nil, goerr.Wrap(err, "failed to decode ticket")
		}
		t.ID = doc.Ref.ID
		t.WorkspaceID = workspaceID
		tickets = append(tickets, &t)
	}
	return tickets, nil
}

func (r *ticketRepository) Update(ctx context.Context, workspaceID string, t *model.Ticket) (*model.Ticket, error) {
	ref := r.ticketsCollection(workspaceID).Doc(t.ID)
	if _, err := ref.Set(ctx, t); err != nil {
		return nil, goerr.Wrap(err, "failed to update ticket")
	}
	return t, nil
}

func (r *ticketRepository) Delete(ctx context.Context, workspaceID string, id string) error {
	ref := r.ticketsCollection(workspaceID).Doc(id)
	if _, err := ref.Delete(ctx); err != nil {
		return goerr.Wrap(err, "failed to delete ticket")
	}
	return nil
}

func (r *ticketRepository) GetBySlackThreadTS(ctx context.Context, workspaceID string, channelID, threadTS string) (*model.Ticket, error) {
	iter := r.ticketsCollection(workspaceID).
		Where("SlackChannelID", "==", channelID).
		Where("SlackThreadTS", "==", threadTS).
		Limit(1).
		Documents(ctx)
	defer iter.Stop()

	doc, err := iter.Next()
	if err == iterator.Done {
		return nil, nil
	}
	if err != nil {
		return nil, goerr.Wrap(err, "failed to query ticket by slack thread ts")
	}
	var t model.Ticket
	if err := doc.DataTo(&t); err != nil {
		return nil, goerr.Wrap(err, "failed to decode ticket")
	}
	t.ID = doc.Ref.ID
	t.WorkspaceID = workspaceID
	return &t, nil
}
