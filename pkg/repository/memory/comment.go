package memory

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/m-mizutani/shepherd/pkg/domain/interfaces"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
)

type CommentRepo struct {
	mu       sync.RWMutex
	comments map[string]map[string][]*model.Comment // workspaceID -> ticketID -> comments
}

func newCommentRepo() *CommentRepo {
	return &CommentRepo{
		comments: make(map[string]map[string][]*model.Comment),
	}
}

var _ interfaces.CommentRepository = (*CommentRepo)(nil)

func (r *CommentRepo) Create(ctx context.Context, workspaceID string, ticketID string, c *model.Comment) (*model.Comment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.comments[workspaceID] == nil {
		r.comments[workspaceID] = make(map[string][]*model.Comment)
	}

	c.ID = uuid.Must(uuid.NewV7()).String()
	c.TicketID = ticketID
	c.CreatedAt = time.Now()

	copied := *c
	r.comments[workspaceID][ticketID] = append(r.comments[workspaceID][ticketID], &copied)
	return c, nil
}

func (r *CommentRepo) List(ctx context.Context, workspaceID string, ticketID string) ([]*model.Comment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ws := r.comments[workspaceID]
	if ws == nil {
		return nil, nil
	}
	comments := ws[ticketID]
	result := make([]*model.Comment, len(comments))
	for i, c := range comments {
		copied := *c
		result[i] = &copied
	}
	return result, nil
}

func (r *CommentRepo) GetBySlackTS(ctx context.Context, workspaceID string, ticketID string, slackTS string) (*model.Comment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ws := r.comments[workspaceID]
	if ws == nil {
		return nil, nil
	}
	for _, c := range ws[ticketID] {
		if c.SlackTS == slackTS {
			copied := *c
			return &copied, nil
		}
	}
	return nil, nil
}
