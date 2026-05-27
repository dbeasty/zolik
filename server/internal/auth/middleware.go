package auth

import (
	"context"
	"net/http"
	"strings"
)

type UserContext struct {
	UserID   string
	Username string
	IsGuest  bool
	Token    string
}

type ctxKey struct{}

func GetUserContext(r *http.Request) (UserContext, bool) {
	v := r.Context().Value(ctxKey{})
	if v == nil {
		return UserContext{}, false
	}
	uc, ok := v.(UserContext)
	return uc, ok
}

func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authz := strings.TrimSpace(r.Header.Get("Authorization"))
		if authz == "" {
			http.Error(w, "missing Authorization header", http.StatusUnauthorized)
			return
		}
		parts := strings.Fields(authz)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			http.Error(w, "invalid Authorization header", http.StatusUnauthorized)
			return
		}
		token := parts[1]

		claims, err := ParseAccessClaims(token)
		if err != nil {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}

		uc := UserContext{
			UserID:   claims.Subject,
			Username: claims.Username,
			IsGuest:  claims.IsGuest,
			Token:    token,
		}
		ctx := context.WithValue(r.Context(), ctxKey{}, uc)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

