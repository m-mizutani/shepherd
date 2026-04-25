package http

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/shepherd/pkg/domain/model/auth"
	"github.com/m-mizutani/shepherd/pkg/usecase"
	"github.com/m-mizutani/shepherd/pkg/utils/errutil"
)

type userMeResponse struct {
	Sub   string `json:"sub"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

func generateState() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", goerr.Wrap(err, "failed to generate random state")
	}
	return hex.EncodeToString(bytes), nil
}

func authLoginHandler(authUC usecase.AuthUseCaseInterface) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if authUC.IsNoAuthn() {
			http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
			return
		}

		state, err := generateState()
		if err != nil {
			errutil.HandleHTTP(r.Context(), w, err, http.StatusInternalServerError)
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     "oauth_state",
			Value:    state,
			Path:     "/",
			HttpOnly: true,
			Secure:   r.TLS != nil,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   600,
		})

		http.Redirect(w, r, authUC.GetAuthURL(state), http.StatusTemporaryRedirect)
	}
}

func authCallbackHandler(authUC usecase.AuthUseCaseInterface) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		stateCookie, err := r.Cookie("oauth_state")
		if err != nil {
			errutil.HandleHTTP(r.Context(), w, err, http.StatusBadRequest)
			return
		}

		state := r.URL.Query().Get("state")
		if state == "" || state != stateCookie.Value {
			errutil.HandleHTTP(r.Context(), w, goerr.New("invalid state parameter"), http.StatusBadRequest)
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name: "oauth_state", Value: "", Path: "/",
			HttpOnly: true, Secure: r.TLS != nil,
			SameSite: http.SameSiteLaxMode, MaxAge: -1,
		})

		code := r.URL.Query().Get("code")
		if code == "" {
			errutil.HandleHTTP(r.Context(), w, goerr.New("missing authorization code"), http.StatusBadRequest)
			return
		}

		token, err := authUC.HandleCallback(r.Context(), code)
		if err != nil {
			errutil.HandleHTTP(r.Context(), w, err, http.StatusInternalServerError)
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name: "token_id", Value: token.ID.String(), Path: "/",
			HttpOnly: true, Secure: r.TLS != nil,
			SameSite: http.SameSiteLaxMode, Expires: token.ExpiresAt,
		})
		http.SetCookie(w, &http.Cookie{
			Name: "token_secret", Value: token.Secret.String(), Path: "/",
			HttpOnly: true, Secure: r.TLS != nil,
			SameSite: http.SameSiteLaxMode, Expires: token.ExpiresAt,
		})

		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
	}
}

func authLogoutHandler(authUC usecase.AuthUseCaseInterface) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if tokenIDCookie, err := r.Cookie("token_id"); err == nil {
			if err := authUC.Logout(r.Context(), auth.TokenID(tokenIDCookie.Value)); err != nil {
				errutil.HandleHTTP(r.Context(), w, goerr.Wrap(err, "failed to logout"), http.StatusInternalServerError)
				return
			}
		}

		for _, name := range []string{"token_id", "token_secret"} {
			http.SetCookie(w, &http.Cookie{
				Name: name, Value: "", Path: "/",
				HttpOnly: true, Secure: r.TLS != nil,
				SameSite: http.SameSiteLaxMode, MaxAge: -1,
			})
		}

		writeJSON(r.Context(), w, http.StatusOK, map[string]bool{"success": true})
	}
}

func authMeHandler(authUC usecase.AuthUseCaseInterface) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if authUC.IsNoAuthn() {
			token, err := authUC.ValidateToken(r.Context(), "", "")
			if err != nil {
				errutil.HandleHTTP(r.Context(), w, err, http.StatusInternalServerError)
				return
			}
			writeJSON(r.Context(), w, http.StatusOK, userMeResponse{
				Sub: token.Sub, Email: token.Email, Name: token.Name,
			})
			return
		}

		tokenIDCookie, err := r.Cookie("token_id")
		if err != nil {
			errutil.HandleHTTP(r.Context(), w, err, http.StatusUnauthorized)
			return
		}
		tokenSecretCookie, err := r.Cookie("token_secret")
		if err != nil {
			errutil.HandleHTTP(r.Context(), w, err, http.StatusUnauthorized)
			return
		}

		token, err := authUC.ValidateToken(r.Context(),
			auth.TokenID(tokenIDCookie.Value),
			auth.TokenSecret(tokenSecretCookie.Value))
		if err != nil {
			errutil.HandleHTTP(r.Context(), w, err, http.StatusUnauthorized)
			return
		}

		writeJSON(r.Context(), w, http.StatusOK, userMeResponse{
			Sub: token.Sub, Email: token.Email, Name: token.Name,
		})
	}
}

func writeJSON(ctx context.Context, w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		errutil.Handle(ctx, goerr.Wrap(err, "failed to encode JSON response"))
	}
}
