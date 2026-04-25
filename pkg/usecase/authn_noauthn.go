package usecase

import (
	"context"

	"github.com/m-mizutani/shepherd/pkg/domain/model/auth"
)

type NoAuthnUseCase struct {
	sub   string
	email string
	name  string
}

func NewNoAuthnUseCase(sub, email, name string) *NoAuthnUseCase {
	return &NoAuthnUseCase{
		sub:   sub,
		email: email,
		name:  name,
	}
}

var _ AuthUseCaseInterface = (*NoAuthnUseCase)(nil)

func (uc *NoAuthnUseCase) GetAuthURL(state string) string {
	return "/"
}

func (uc *NoAuthnUseCase) HandleCallback(ctx context.Context, code string) (*auth.Token, error) {
	return auth.NewToken(uc.sub, uc.email, uc.name), nil
}

func (uc *NoAuthnUseCase) ValidateToken(ctx context.Context, tokenID auth.TokenID, tokenSecret auth.TokenSecret) (*auth.Token, error) {
	return auth.NewToken(uc.sub, uc.email, uc.name), nil
}

func (uc *NoAuthnUseCase) Logout(ctx context.Context, tokenID auth.TokenID) error {
	return nil
}

func (uc *NoAuthnUseCase) IsNoAuthn() bool {
	return true
}
