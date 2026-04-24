package memory

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/shepherd/pkg/domain/interfaces"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
)

type TicketRepo struct {
	mu       sync.RWMutex
	tickets  map[string]map[string]*model.Ticket // workspaceID -> ticketID -> ticket
	seqNums  map[string]int64                    // workspaceID -> next seq num
}

func newTicketRepo() *TicketRepo {
	return &TicketRepo{
		tickets: make(map[string]map[string]*model.Ticket),
		seqNums: make(map[string]int64),
	}
}

var _ interfaces.TicketRepository = (*TicketRepo)(nil)

func (r *TicketRepo) Create(ctx context.Context, workspaceID string, t *model.Ticket) (*model.Ticket, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.tickets[workspaceID] == nil {
		r.tickets[workspaceID] = make(map[string]*model.Ticket)
	}

	r.seqNums[workspaceID]++
	if t.ID == "" {
		t.ID = uuid.Must(uuid.NewV7()).String()
	}
	t.WorkspaceID = workspaceID
	t.SeqNum = r.seqNums[workspaceID]

	copied := *t
	r.tickets[workspaceID][t.ID] = &copied
	return t, nil
}

func (r *TicketRepo) Get(ctx context.Context, workspaceID string, id string) (*model.Ticket, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ws, ok := r.tickets[workspaceID]
	if !ok {
		return nil, goerr.New("ticket not found")
	}
	t, ok := ws[id]
	if !ok {
		return nil, goerr.New("ticket not found")
	}
	copied := *t
	return &copied, nil
}

func (r *TicketRepo) List(ctx context.Context, workspaceID string, statusIDs []string) ([]*model.Ticket, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	statusSet := make(map[string]struct{}, len(statusIDs))
	for _, id := range statusIDs {
		statusSet[id] = struct{}{}
	}

	ws := r.tickets[workspaceID]
	var result []*model.Ticket
	for _, t := range ws {
		if len(statusSet) > 0 {
			if _, ok := statusSet[t.StatusID]; !ok {
				continue
			}
		}
		copied := *t
		result = append(result, &copied)
	}
	return result, nil
}

func (r *TicketRepo) Update(ctx context.Context, workspaceID string, t *model.Ticket) (*model.Ticket, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	ws, ok := r.tickets[workspaceID]
	if !ok {
		return nil, goerr.New("ticket not found")
	}
	if _, ok := ws[t.ID]; !ok {
		return nil, goerr.New("ticket not found")
	}

	t.UpdatedAt = time.Now()
	copied := *t
	ws[t.ID] = &copied
	return t, nil
}

func (r *TicketRepo) Delete(ctx context.Context, workspaceID string, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	ws, ok := r.tickets[workspaceID]
	if !ok {
		return nil
	}
	delete(ws, id)
	return nil
}

func (r *TicketRepo) GetBySlackThreadTS(ctx context.Context, workspaceID string, channelID, threadTS string) (*model.Ticket, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ws := r.tickets[workspaceID]
	for _, t := range ws {
		if t.SlackChannelID == channelID && t.SlackThreadTS == threadTS {
			copied := *t
			return &copied, nil
		}
	}
	return nil, nil
}
