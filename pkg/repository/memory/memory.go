package memory

import (
	"context"
	"sync"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/shepherd/pkg/domain/interfaces"
	"github.com/m-mizutani/shepherd/pkg/domain/model/auth"
)

type Repository struct {
	mu            sync.RWMutex
	tickets       *TicketRepo
	comments      *CommentRepo
	ticketHistory *TicketHistoryRepo
	sources       *SourceRepo
	toolSettings  *ToolSettingsRepo
	prompts       *PromptRepo
	tokens        map[string]*auth.Token
}

func New() *Repository {
	history := newTicketHistoryRepo()
	return &Repository{
		tickets:       newTicketRepo(history),
		comments:      newCommentRepo(),
		ticketHistory: history,
		sources:       newSourceRepo(),
		toolSettings:  newToolSettingsRepo(),
		prompts:       newPromptRepo(),
		tokens:        make(map[string]*auth.Token),
	}
}

var _ interfaces.Repository = (*Repository)(nil)

func (r *Repository) Ticket() interfaces.TicketRepository              { return r.tickets }
func (r *Repository) Comment() interfaces.CommentRepository            { return r.comments }
func (r *Repository) TicketHistory() interfaces.TicketHistoryRepository { return r.ticketHistory }
func (r *Repository) Source() interfaces.SourceRepository              { return r.sources }
func (r *Repository) ToolSettings() interfaces.ToolSettingsRepository  { return r.toolSettings }
func (r *Repository) Prompt() interfaces.PromptRepository              { return r.prompts }

func (r *Repository) PutToken(ctx context.Context, token *auth.Token) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tokens[token.ID.String()] = token
	return nil
}

func (r *Repository) GetToken(ctx context.Context, tokenID auth.TokenID) (*auth.Token, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	token, ok := r.tokens[tokenID.String()]
	if !ok {
		return nil, goerr.New("token not found")
	}
	return token, nil
}

func (r *Repository) DeleteToken(ctx context.Context, tokenID auth.TokenID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.tokens, tokenID.String())
	return nil
}

func (r *Repository) Close() error { return nil }
