package usecase

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jwt"
	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/shepherd/pkg/domain/interfaces"
	"github.com/m-mizutani/shepherd/pkg/domain/model/auth"
	"github.com/m-mizutani/shepherd/pkg/utils/safe"
)

type AuthUseCase struct {
	repo         interfaces.Repository
	clientID     string
	clientSecret string
	callbackURL  string
	cache        *authCache
}

var _ AuthUseCaseInterface = (*AuthUseCase)(nil)

func NewAuthUseCase(repo interfaces.Repository, clientID, clientSecret, callbackURL string) *AuthUseCase {
	return &AuthUseCase{
		repo:         repo,
		clientID:     clientID,
		clientSecret: clientSecret,
		callbackURL:  callbackURL,
		cache:        newAuthCache(),
	}
}

func (uc *AuthUseCase) GetAuthURL(state string) string {
	params := url.Values{}
	params.Set("client_id", uc.clientID)
	params.Set("scope", "openid,email,profile")
	params.Set("redirect_uri", uc.callbackURL)
	params.Set("response_type", "code")
	params.Set("state", state)
	return "https://slack.com/openid/connect/authorize?" + params.Encode()
}

func (uc *AuthUseCase) IsNoAuthn() bool { return false }

type slackTokenResponse struct {
	OK      bool   `json:"ok"`
	IDToken string `json:"id_token"`
	Error   string `json:"error"`
}

type slackIDToken struct {
	Sub   string `json:"sub"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

func (uc *AuthUseCase) HandleCallback(ctx context.Context, code string) (*auth.Token, error) {
	tokenResp, err := uc.exchangeCodeForToken(ctx, code)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to exchange code for token")
	}

	if !tokenResp.OK || tokenResp.Error != "" {
		return nil, goerr.New("slack oauth error", goerr.V("error", tokenResp.Error))
	}

	idToken, err := uc.decodeIDToken(ctx, tokenResp.IDToken)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to decode ID token")
	}

	token := auth.NewToken(idToken.Sub, idToken.Email, idToken.Name)
	if err := uc.repo.PutToken(ctx, token); err != nil {
		return nil, goerr.Wrap(err, "failed to store token")
	}

	return token, nil
}

func (uc *AuthUseCase) exchangeCodeForToken(ctx context.Context, code string) (*slackTokenResponse, error) {
	data := url.Values{}
	data.Set("client_id", uc.clientID)
	data.Set("client_secret", uc.clientSecret)
	data.Set("code", code)
	data.Set("redirect_uri", uc.callbackURL)

	encoded := data.Encode()
	req, err := http.NewRequestWithContext(ctx, "POST", "https://slack.com/api/openid.connect.token", strings.NewReader(encoded))
	if err != nil {
		return nil, goerr.Wrap(err, "failed to create request")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.ContentLength = int64(len(encoded))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to make token request")
	}
	defer safe.Close(ctx, resp.Body)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to read response body")
	}

	var tokenResp slackTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, goerr.Wrap(err, "failed to parse token response")
	}

	return &tokenResp, nil
}

func (uc *AuthUseCase) decodeIDToken(ctx context.Context, idTokenStr string) (*slackIDToken, error) {
	keySet, err := jwk.Fetch(ctx, "https://slack.com/openid/connect/keys")
	if err != nil {
		return nil, goerr.Wrap(err, "failed to fetch Slack's public keys")
	}

	token, err := jwt.Parse([]byte(idTokenStr),
		jwt.WithKeySet(keySet),
		jwt.WithValidate(true),
		jwt.WithAudience(uc.clientID),
		jwt.WithAcceptableSkew(10),
	)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to parse or verify JWT token")
	}

	sub, _ := token.Get("sub")
	email, _ := token.Get("email")
	name, _ := token.Get("name")

	subStr, ok := sub.(string)
	if !ok {
		return nil, goerr.New("sub claim is not a string")
	}
	emailStr, ok := email.(string)
	if !ok {
		return nil, goerr.New("email claim is not a string")
	}
	nameStr, ok := name.(string)
	if !ok {
		return nil, goerr.New("name claim is not a string")
	}

	return &slackIDToken{Sub: subStr, Email: emailStr, Name: nameStr}, nil
}

func (uc *AuthUseCase) ValidateToken(ctx context.Context, tokenID auth.TokenID, tokenSecret auth.TokenSecret) (*auth.Token, error) {
	return uc.validateTokenWithCache(ctx, tokenID, tokenSecret)
}

func (uc *AuthUseCase) Logout(ctx context.Context, tokenID auth.TokenID) error {
	uc.cache.remove(tokenID)
	return uc.repo.DeleteToken(ctx, tokenID)
}
