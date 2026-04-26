package firestore

import (
	"context"

	"cloud.google.com/go/firestore"
	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/shepherd/pkg/domain/interfaces"
	"github.com/m-mizutani/shepherd/pkg/domain/model/auth"
)

type Repository struct {
	client        *firestore.Client
	ticket        *ticketRepository
	comment       *commentRepository
	ticketHistory *ticketHistoryRepository
	source        *sourceRepository
	toolSettings  *toolSettingsRepository
}

func New(ctx context.Context, projectID, databaseID string) (*Repository, error) {
	var client *firestore.Client
	var err error

	if databaseID != "" {
		client, err = firestore.NewClientWithDatabase(ctx, projectID, databaseID)
	} else {
		client, err = firestore.NewClient(ctx, projectID)
	}
	if err != nil {
		return nil, goerr.Wrap(err, "failed to create firestore client",
			goerr.V("project_id", projectID),
			goerr.V("database_id", databaseID),
		)
	}

	return &Repository{
		client:        client,
		ticket:        &ticketRepository{client: client},
		comment:       &commentRepository{client: client},
		ticketHistory: &ticketHistoryRepository{client: client},
		source:        &sourceRepository{client: client},
		toolSettings:  &toolSettingsRepository{client: client},
	}, nil
}

func (r *Repository) Ticket() interfaces.TicketRepository {
	return r.ticket
}

func (r *Repository) Comment() interfaces.CommentRepository {
	return r.comment
}

func (r *Repository) TicketHistory() interfaces.TicketHistoryRepository {
	return r.ticketHistory
}

func (r *Repository) Source() interfaces.SourceRepository {
	return r.source
}

func (r *Repository) ToolSettings() interfaces.ToolSettingsRepository {
	return r.toolSettings
}

func (r *Repository) PutToken(ctx context.Context, token *auth.Token) error {
	ref := r.client.Collection("auth_tokens").Doc(token.ID.String())
	_, err := ref.Set(ctx, map[string]any{
		"secret":     token.Secret.String(),
		"sub":        token.Sub,
		"email":      token.Email,
		"name":       token.Name,
		"expires_at": token.ExpiresAt,
		"created_at": token.CreatedAt,
	})
	if err != nil {
		return goerr.Wrap(err, "failed to put token")
	}
	return nil
}

func (r *Repository) GetToken(ctx context.Context, tokenID auth.TokenID) (*auth.Token, error) {
	ref := r.client.Collection("auth_tokens").Doc(string(tokenID))
	doc, err := ref.Get(ctx)
	if err != nil {
		if isNotFound(err) {
			return nil, goerr.New("token not found", goerr.V("token_id", tokenID))
		}
		return nil, goerr.Wrap(err, "failed to get token")
	}

	data := doc.Data()
	token := &auth.Token{
		ID:     tokenID,
		Secret: auth.TokenSecret(data["secret"].(string)),
		Sub:    data["sub"].(string),
		Email:  data["email"].(string),
		Name:   data["name"].(string),
	}
	if v, ok := data["expires_at"]; ok {
		token.ExpiresAt = toTime(v)
	}
	if v, ok := data["created_at"]; ok {
		token.CreatedAt = toTime(v)
	}
	return token, nil
}

func (r *Repository) DeleteToken(ctx context.Context, tokenID auth.TokenID) error {
	ref := r.client.Collection("auth_tokens").Doc(string(tokenID))
	_, err := ref.Delete(ctx)
	if err != nil {
		return goerr.Wrap(err, "failed to delete token")
	}
	return nil
}

func (r *Repository) Close() error {
	return r.client.Close()
}
