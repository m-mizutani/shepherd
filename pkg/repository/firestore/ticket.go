package firestore

import (
	"context"

	"cloud.google.com/go/firestore"
	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
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
	if _, err := ref.Set(ctx, ticketToMap(t)); err != nil {
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
	return mapToTicket(doc.Data(), id, workspaceID), nil
}

func (r *ticketRepository) List(ctx context.Context, workspaceID string, statusIDs []string) ([]*model.Ticket, error) {
	query := r.ticketsCollection(workspaceID).OrderBy("created_at", firestore.Desc)

	if len(statusIDs) > 0 {
		query = r.ticketsCollection(workspaceID).Where("status_id", "in", statusIDs).OrderBy("created_at", firestore.Desc)
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
		tickets = append(tickets, mapToTicket(doc.Data(), doc.Ref.ID, workspaceID))
	}
	return tickets, nil
}

func (r *ticketRepository) Update(ctx context.Context, workspaceID string, t *model.Ticket) (*model.Ticket, error) {
	ref := r.ticketsCollection(workspaceID).Doc(t.ID)
	if _, err := ref.Set(ctx, ticketToMap(t)); err != nil {
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
		Where("slack_channel_id", "==", channelID).
		Where("slack_thread_ts", "==", threadTS).
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
	return mapToTicket(doc.Data(), doc.Ref.ID, workspaceID), nil
}

func ticketToMap(t *model.Ticket) map[string]any {
	fieldValues := map[string]any{}
	for k, v := range t.FieldValues {
		fieldValues[k] = map[string]any{
			"field_id": v.FieldID,
			"type":     string(v.Type),
			"value":    v.Value,
		}
	}

	return map[string]any{
		"seq_num":               t.SeqNum,
		"title":                 t.Title,
		"description":           t.Description,
		"status_id":             t.StatusID,
		"assignee_id":           t.AssigneeID,
		"reporter_slack_user_id": t.ReporterSlackUserID,
		"slack_channel_id":      t.SlackChannelID,
		"slack_thread_ts":       t.SlackThreadTS,
		"field_values":          fieldValues,
		"created_at":            t.CreatedAt,
		"updated_at":            t.UpdatedAt,
	}
}

func mapToTicket(data map[string]any, id, workspaceID string) *model.Ticket {
	t := &model.Ticket{
		ID:          id,
		WorkspaceID: workspaceID,
		FieldValues: make(map[string]model.FieldValue),
	}

	if v, ok := data["seq_num"]; ok {
		t.SeqNum = v.(int64)
	}
	if v, ok := data["title"]; ok {
		t.Title = v.(string)
	}
	if v, ok := data["description"]; ok {
		t.Description = v.(string)
	}
	if v, ok := data["status_id"]; ok {
		t.StatusID = v.(string)
	}
	if v, ok := data["assignee_id"]; ok {
		t.AssigneeID = v.(string)
	}
	if v, ok := data["reporter_slack_user_id"]; ok {
		t.ReporterSlackUserID = v.(string)
	}
	if v, ok := data["slack_channel_id"]; ok {
		t.SlackChannelID = v.(string)
	}
	if v, ok := data["slack_thread_ts"]; ok {
		t.SlackThreadTS = v.(string)
	}
	if v, ok := data["created_at"]; ok {
		t.CreatedAt = toTime(v)
	}
	if v, ok := data["updated_at"]; ok {
		t.UpdatedAt = toTime(v)
	}
	if v, ok := data["field_values"]; ok {
		if fvMap, ok := v.(map[string]any); ok {
			for k, fv := range fvMap {
				if m, ok := fv.(map[string]any); ok {
					t.FieldValues[k] = model.FieldValue{
						FieldID: m["field_id"].(string),
						Type:    types.FieldType(m["type"].(string)),
						Value:   m["value"],
					}
				}
			}
		}
	}

	return t
}
