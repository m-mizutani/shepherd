package memory

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/shepherd/pkg/domain/interfaces"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
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

func (r *TicketRepo) Create(ctx context.Context, workspaceID types.WorkspaceID, t *model.Ticket) (*model.Ticket, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	wsKey := string(workspaceID)
	if r.tickets[wsKey] == nil {
		r.tickets[wsKey] = make(map[string]*model.Ticket)
	}

	r.seqNums[wsKey]++
	if t.ID == "" {
		t.ID = types.TicketID(uuid.Must(uuid.NewV7()).String())
	}
	t.WorkspaceID = workspaceID
	t.SeqNum = r.seqNums[wsKey]

	copied := *t
	r.tickets[wsKey][string(t.ID)] = &copied
	return t, nil
}

func (r *TicketRepo) Get(ctx context.Context, workspaceID types.WorkspaceID, id types.TicketID) (*model.Ticket, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ws, ok := r.tickets[string(workspaceID)]
	if !ok {
		return nil, goerr.New("ticket not found")
	}
	t, ok := ws[string(id)]
	if !ok {
		return nil, goerr.New("ticket not found")
	}
	copied := *t
	return &copied, nil
}

func (r *TicketRepo) List(ctx context.Context, workspaceID types.WorkspaceID, statusIDs []types.StatusID) ([]*model.Ticket, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	statusSet := make(map[types.StatusID]struct{}, len(statusIDs))
	for _, id := range statusIDs {
		statusSet[id] = struct{}{}
	}

	ws := r.tickets[string(workspaceID)]
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

func (r *TicketRepo) Update(ctx context.Context, workspaceID types.WorkspaceID, t *model.Ticket) (*model.Ticket, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	ws, ok := r.tickets[string(workspaceID)]
	if !ok {
		return nil, goerr.New("ticket not found")
	}
	if _, ok := ws[string(t.ID)]; !ok {
		return nil, goerr.New("ticket not found")
	}

	t.UpdatedAt = time.Now()
	copied := *t
	ws[string(t.ID)] = &copied
	return t, nil
}

func (r *TicketRepo) Delete(ctx context.Context, workspaceID types.WorkspaceID, id types.TicketID) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	ws, ok := r.tickets[string(workspaceID)]
	if !ok {
		return nil
	}
	delete(ws, string(id))
	return nil
}

func (r *TicketRepo) GetBySlackThreadTS(ctx context.Context, workspaceID types.WorkspaceID, channelID types.SlackChannelID, threadTS types.SlackThreadTS) (*model.Ticket, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ws := r.tickets[string(workspaceID)]
	for _, t := range ws {
		if t.SlackChannelID == channelID && t.SlackThreadTS == threadTS {
			copied := *t
			return &copied, nil
		}
	}
	return nil, nil
}
