package firestore

import (
	"context"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/google/uuid"
	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
	"google.golang.org/api/iterator"
)

type ticketRepository struct {
	client *firestore.Client
}

func (r *ticketRepository) ticketsCollection(workspaceID types.WorkspaceID) *firestore.CollectionRef {
	return r.client.Collection("workspaces").Doc(string(workspaceID)).Collection("tickets")
}

func (r *ticketRepository) counterRef(workspaceID types.WorkspaceID) *firestore.DocumentRef {
	return r.client.Collection("workspaces").Doc(string(workspaceID)).Collection("counters").Doc("ticket")
}

func (r *ticketRepository) getNextSeqNum(ctx context.Context, workspaceID types.WorkspaceID) (int64, error) {
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

func (r *ticketRepository) Create(ctx context.Context, workspaceID types.WorkspaceID, t *model.Ticket) (*model.Ticket, error) {
	seqNum, err := r.getNextSeqNum(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	t.SeqNum = seqNum

	ref := r.ticketsCollection(workspaceID).Doc(string(t.ID))
	if _, err := ref.Set(ctx, t); err != nil {
		return nil, goerr.Wrap(err, "failed to create ticket")
	}
	return t, nil
}

func (r *ticketRepository) Get(ctx context.Context, workspaceID types.WorkspaceID, id types.TicketID) (*model.Ticket, error) {
	doc, err := r.ticketsCollection(workspaceID).Doc(string(id)).Get(ctx)
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
	t.ID = types.TicketID(doc.Ref.ID)
	t.WorkspaceID = workspaceID
	return &t, nil
}

func (r *ticketRepository) List(ctx context.Context, workspaceID types.WorkspaceID, statusIDs []types.StatusID) ([]*model.Ticket, error) {
	query := r.ticketsCollection(workspaceID).OrderBy("CreatedAt", firestore.Desc)

	if len(statusIDs) > 0 {
		// Convert to []any for Firestore "in" query
		ids := make([]any, len(statusIDs))
		for i, id := range statusIDs {
			ids[i] = id
		}
		query = r.ticketsCollection(workspaceID).Where("StatusID", "in", ids).OrderBy("CreatedAt", firestore.Desc)
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
		t.ID = types.TicketID(doc.Ref.ID)
		t.WorkspaceID = workspaceID
		tickets = append(tickets, &t)
	}
	return tickets, nil
}

func (r *ticketRepository) Update(ctx context.Context, workspaceID types.WorkspaceID, t *model.Ticket) (*model.Ticket, error) {
	ref := r.ticketsCollection(workspaceID).Doc(string(t.ID))
	if _, err := ref.Set(ctx, t); err != nil {
		return nil, goerr.Wrap(err, "failed to update ticket")
	}
	return t, nil
}

func (r *ticketRepository) Delete(ctx context.Context, workspaceID types.WorkspaceID, id types.TicketID) error {
	ref := r.ticketsCollection(workspaceID).Doc(string(id))
	if _, err := ref.Delete(ctx); err != nil {
		return goerr.Wrap(err, "failed to delete ticket")
	}
	return nil
}

func (r *ticketRepository) FinalizeTriage(ctx context.Context, workspaceID types.WorkspaceID, ticketID types.TicketID, assignees *[]types.SlackUserID, history *model.TicketHistory) error {
	if history == nil {
		return goerr.New("history entry is required")
	}

	ticketRef := r.ticketsCollection(workspaceID).Doc(string(ticketID))
	historyCol := r.client.Collection("workspaces").Doc(string(workspaceID)).
		Collection("tickets").Doc(string(ticketID)).Collection("history")

	if history.ID == "" {
		history.ID = uuid.Must(uuid.NewV7()).String()
	}
	if history.CreatedAt.IsZero() {
		history.CreatedAt = time.Now()
	}

	return r.client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		doc, err := tx.Get(ticketRef)
		if err != nil {
			if isNotFound(err) {
				return goerr.New("ticket not found", goerr.V("ticket_id", ticketID))
			}
			return goerr.Wrap(err, "failed to read ticket")
		}
		var t model.Ticket
		if err := doc.DataTo(&t); err != nil {
			return goerr.Wrap(err, "failed to decode ticket")
		}
		// Idempotent: already finalized.
		if t.Triaged {
			return nil
		}

		updates := []firestore.Update{
			{Path: "Triaged", Value: true},
			{Path: "UpdatedAt", Value: time.Now()},
		}
		if assignees != nil {
			ids := make([]string, len(*assignees))
			for i, id := range *assignees {
				ids[i] = string(id)
			}
			updates = append(updates, firestore.Update{Path: "AssigneeIDs", Value: ids})
		}
		if err := tx.Update(ticketRef, updates); err != nil {
			return goerr.Wrap(err, "failed to update ticket")
		}
		if err := tx.Set(historyCol.Doc(history.ID), history); err != nil {
			return goerr.Wrap(err, "failed to append history")
		}
		return nil
	})
}

func (r *ticketRepository) GetBySlackThreadTS(ctx context.Context, workspaceID types.WorkspaceID, channelID types.SlackChannelID, threadTS types.SlackThreadTS) (*model.Ticket, error) {
	iter := r.ticketsCollection(workspaceID).
		Where("SlackChannelID", "==", string(channelID)).
		Where("SlackThreadTS", "==", string(threadTS)).
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
	t.ID = types.TicketID(doc.Ref.ID)
	t.WorkspaceID = workspaceID
	return &t, nil
}
