package usecase

import (
	"context"

	"github.com/m-mizutani/shepherd/pkg/domain/model/auth"
)

type AuthUseCaseInterface interface {
	GetAuthURL(state string) string
	HandleCallback(ctx context.Context, code string) (*auth.Token, error)
	ValidateToken(ctx context.Context, tokenID auth.TokenID, tokenSecret auth.TokenSecret) (*auth.Token, error)
	Logout(ctx context.Context, tokenID auth.TokenID) error
	IsNoAuthn() bool
}
