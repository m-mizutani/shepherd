package firestore

import (
	"context"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/google/uuid"
	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/shepherd/pkg/domain/interfaces"
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

func (r *ticketRepository) UpdateEmbedding(ctx context.Context, workspaceID types.WorkspaceID, ticketID types.TicketID, embedding firestore.Vector32, modelID string) error {
	ref := r.ticketsCollection(workspaceID).Doc(string(ticketID))
	patch := map[string]any{
		"Embedding":      embedding,
		"EmbeddingModel": modelID,
	}
	if _, err := ref.Set(ctx, patch, firestore.Merge(
		[]string{"Embedding"},
		[]string{"EmbeddingModel"},
	)); err != nil {
		return goerr.Wrap(err, "failed to update ticket embedding",
			goerr.V("ticket_id", ticketID),
		)
	}
	return nil
}

func (r *ticketRepository) FindSimilar(ctx context.Context, workspaceID types.WorkspaceID, queryVector firestore.Vector32, limit int, statusIDs []types.StatusID) ([]interfaces.TicketWithDistance, error) {
	if len(queryVector) == 0 {
		return nil, goerr.New("query vector is empty")
	}
	if limit <= 0 {
		return nil, nil
	}

	// Status filtering happens in Go after FindNearest returns. Combining
	// Where + FindNearest at the Firestore level would require a composite
	// vector index (StatusID + Embedding), which CLAUDE.md forbids: the
	// only Firestore index this codebase declares is the single-field
	// vector index on Embedding (managed via fireconf in `migrate`). When
	// the caller passes status_ids we therefore over-fetch the nearest
	// vectors and discard ones outside the filter; the cap bounds the
	// over-fetch so a malicious limit cannot blow up the request.
	const (
		distanceField    = "vector_distance"
		statusOverFetchN = 5  // fetch up to N*limit before filtering by status
		hardFetchCap     = 200
	)
	fetchLimit := limit
	if len(statusIDs) > 0 {
		fetchLimit = limit * statusOverFetchN
		if fetchLimit > hardFetchCap {
			fetchLimit = hardFetchCap
		}
	}
	statusSet := make(map[types.StatusID]struct{}, len(statusIDs))
	for _, id := range statusIDs {
		statusSet[id] = struct{}{}
	}

	col := r.ticketsCollection(workspaceID)
	vq := col.FindNearest("Embedding", queryVector, fetchLimit, firestore.DistanceMeasureCosine, &firestore.FindNearestOptions{
		DistanceResultField: distanceField,
	})

	iter := vq.Documents(ctx)
	defer iter.Stop()

	out := make([]interfaces.TicketWithDistance, 0, limit)
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, goerr.Wrap(err, "failed to iterate similar tickets")
		}
		var t model.Ticket
		if err := doc.DataTo(&t); err != nil {
			return nil, goerr.Wrap(err, "failed to decode similar ticket",
				goerr.V("ticket_id", doc.Ref.ID),
			)
		}
		t.ID = types.TicketID(doc.Ref.ID)
		t.WorkspaceID = workspaceID
		if len(statusSet) > 0 {
			if _, ok := statusSet[t.StatusID]; !ok {
				continue
			}
		}

		distance := 0.0
		if v, ok := doc.Data()[distanceField]; ok {
			if f, ok := v.(float64); ok {
				distance = f
			}
		}
		out = append(out, interfaces.TicketWithDistance{
			Ticket:   &t,
			Distance: distance,
		})
		if len(out) >= limit {
			break
		}
	}
	return out, nil
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
