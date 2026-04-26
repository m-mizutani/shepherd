package memory

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/m-mizutani/shepherd/pkg/domain/interfaces"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
)

type TicketHistoryRepo struct {
	mu        sync.RWMutex
	histories map[string]map[string][]*model.TicketHistory // workspaceID -> ticketID -> histories
}

func newTicketHistoryRepo() *TicketHistoryRepo {
	return &TicketHistoryRepo{
		histories: make(map[string]map[string][]*model.TicketHistory),
	}
}

var _ interfaces.TicketHistoryRepository = (*TicketHistoryRepo)(nil)

func (r *TicketHistoryRepo) Create(ctx context.Context, workspaceID types.WorkspaceID, ticketID types.TicketID, h *model.TicketHistory) (*model.TicketHistory, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	h.ID = uuid.Must(uuid.NewV7()).String()
	if h.CreatedAt.IsZero() {
		h.CreatedAt = time.Now()
	}
	r.appendLocked(workspaceID, ticketID, h)
	return h, nil
}

// appendLocked appends a history entry without acquiring r.mu. The caller must
// already hold a coordinating lock (e.g. TicketRepo.mu during FinalizeTriage)
// to keep the ticket update and history append atomic with respect to readers.
func (r *TicketHistoryRepo) appendLocked(workspaceID types.WorkspaceID, ticketID types.TicketID, h *model.TicketHistory) {
	wsKey := string(workspaceID)
	tkKey := string(ticketID)
	if r.histories[wsKey] == nil {
		r.histories[wsKey] = make(map[string][]*model.TicketHistory)
	}
	copied := *h
	r.histories[wsKey][tkKey] = append(r.histories[wsKey][tkKey], &copied)
}

func (r *TicketHistoryRepo) List(ctx context.Context, workspaceID types.WorkspaceID, ticketID types.TicketID) ([]*model.TicketHistory, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ws := r.histories[string(workspaceID)]
	if ws == nil {
		return nil, nil
	}
	histories := ws[string(ticketID)]
	result := make([]*model.TicketHistory, len(histories))
	for i, h := range histories {
		copied := *h
		result[i] = &copied
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.Before(result[j].CreatedAt)
	})
	return result, nil
}
