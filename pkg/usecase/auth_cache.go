package usecase

import (
	"context"
	"sync"
	"time"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/shepherd/pkg/domain/model/auth"
)

const authCacheTTL = 5 * time.Minute

type cachedToken struct {
	token     *auth.Token
	expiresAt time.Time
}

type authCache struct {
	cache sync.Map
}

func newAuthCache() *authCache {
	return &authCache{}
}

func (c *authCache) get(tokenID auth.TokenID) (*auth.Token, bool) {
	val, ok := c.cache.Load(tokenID)
	if !ok {
		return nil, false
	}
	cached := val.(*cachedToken)
	if time.Now().After(cached.expiresAt) {
		c.cache.Delete(tokenID)
		return nil, false
	}
	return cached.token, true
}

func (c *authCache) set(token *auth.Token) {
	c.cache.Store(token.ID, &cachedToken{
		token:     token,
		expiresAt: time.Now().Add(authCacheTTL),
	})
}

func (c *authCache) remove(tokenID auth.TokenID) {
	c.cache.Delete(tokenID)
}

func (uc *AuthUseCase) validateTokenWithCache(ctx context.Context, tokenID auth.TokenID, tokenSecret auth.TokenSecret) (*auth.Token, error) {
	if token, ok := uc.cache.get(tokenID); ok {
		if token.Secret != tokenSecret {
			return nil, goerr.New("invalid token secret")
		}
		if token.IsExpired() {
			uc.cache.remove(tokenID)
			return nil, goerr.New("token expired")
		}
		return token, nil
	}

	token, err := uc.repo.GetToken(ctx, tokenID)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to get token from repository")
	}

	if token.Secret != tokenSecret {
		return nil, goerr.New("invalid token secret")
	}

	if token.IsExpired() {
		if err := uc.repo.DeleteToken(ctx, tokenID); err != nil {
			return nil, goerr.Wrap(err, "failed to delete expired token")
		}
		return nil, goerr.New("token expired")
	}

	uc.cache.set(token)
	return token, nil
}
