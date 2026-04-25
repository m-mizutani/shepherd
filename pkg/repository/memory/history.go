package memory

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/m-mizutani/shepherd/pkg/domain/interfaces"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
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

func (r *TicketHistoryRepo) Create(ctx context.Context, workspaceID, ticketID string, h *model.TicketHistory) (*model.TicketHistory, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.histories[workspaceID] == nil {
		r.histories[workspaceID] = make(map[string][]*model.TicketHistory)
	}

	h.ID = uuid.Must(uuid.NewV7()).String()
	if h.CreatedAt.IsZero() {
		h.CreatedAt = time.Now()
	}

	copied := *h
	r.histories[workspaceID][ticketID] = append(r.histories[workspaceID][ticketID], &copied)
	return h, nil
}

func (r *TicketHistoryRepo) List(ctx context.Context, workspaceID, ticketID string) ([]*model.TicketHistory, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ws := r.histories[workspaceID]
	if ws == nil {
		return nil, nil
	}
	histories := ws[ticketID]
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
