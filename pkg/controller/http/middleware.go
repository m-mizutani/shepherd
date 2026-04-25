package http

import (
	"fmt"
	"net/http"

	"github.com/m-mizutani/shepherd/pkg/domain/model/auth"
	"github.com/m-mizutani/shepherd/pkg/usecase"
	"github.com/m-mizutani/shepherd/pkg/utils/errutil"
)

func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = fmt.Fprintf(w, `{"error":"authentication required"}`)
}

func authMiddleware(authUC usecase.AuthUseCaseInterface) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if authUC.IsNoAuthn() {
				token, err := authUC.ValidateToken(r.Context(), "", "")
				if err != nil {
					errutil.HandleHTTP(r.Context(), w, err, http.StatusInternalServerError)
					return
				}
				ctx := auth.ContextWithToken(r.Context(), token)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			tokenIDCookie, err := r.Cookie("token_id")
			if err != nil {
				writeUnauthorized(w)
				return
			}
			tokenSecretCookie, err := r.Cookie("token_secret")
			if err != nil {
				writeUnauthorized(w)
				return
			}

			token, err := authUC.ValidateToken(r.Context(),
				auth.TokenID(tokenIDCookie.Value),
				auth.TokenSecret(tokenSecretCookie.Value))
			if err != nil {
				writeUnauthorized(w)
				return
			}

			ctx := auth.ContextWithToken(r.Context(), token)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
