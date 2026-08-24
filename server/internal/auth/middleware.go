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

// PlayerUserID is the account id to record on a seat this person takes, and
// PlayerGuestID the device id — exactly one of them is ever set.
//
// Both live here so the two lobby entry points cannot drift on the question of
// what identity a seat carries, which is the question the whole statistics
// pipeline is keyed on.
func (uc UserContext) PlayerUserID() string {
	if uc.IsGuest {
		return ""
	}
	return uc.UserID
}

func (uc UserContext) PlayerGuestID() string {
	if uc.IsGuest {
		return uc.UserID
	}
	return ""
}

// OptionalAuthMiddleware attaches the caller's identity when they present one
// and lets them through when they do not.
//
// The sign-in endpoints need this: signing in is by definition something an
// unauthenticated person does, but *who they currently are* changes what
// signing in means — a guest's play history gets claimed, a signed-in player
// is linking a second provider. Requiring a token would break the first case;
// ignoring one would break the other.
//
// A malformed or expired token is treated as absent rather than rejected. The
// alternative would leave someone whose token has quietly expired unable to
// sign in again, which is precisely when they need to most.
func OptionalAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Fields(strings.TrimSpace(r.Header.Get("Authorization")))
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			next.ServeHTTP(w, r)
			return
		}
		claims, err := ParseAccessClaims(parts[1])
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}
		ctx := context.WithValue(r.Context(), ctxKey{}, UserContext{
			UserID:   claims.Subject,
			Username: claims.Username,
			IsGuest:  claims.IsGuest,
			Token:    parts[1],
		})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
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
