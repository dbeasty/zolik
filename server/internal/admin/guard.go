package admin

import (
	"context"
	"net/http"
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/auth"
	"zolik/server/internal/models"
)

// UserLookup resolves a signed-in caller to their account. An interface rather
// than the concrete repository so the guard can be tested without a database.
type UserLookup interface {
	FindByID(ctx context.Context, id bson.ObjectID) (models.User, error)
}

// Guard decides who may use the admin API.
//
// Membership is an allow-list of email addresses from configuration, not a
// flag on the user document, for three reasons. There is no bootstrap problem
// — the first administrator exists the moment the variable is set, with no
// chicken-and-egg "who promotes the promoter". Revocation is immediate and
// total: an address removed from the list loses access on its very next
// request, whereas a role baked into a JWT would stay valid until the token
// expired. And it cannot be escalated from inside the running system at all,
// because nothing the API exposes can write to it.
type Guard struct {
	users  UserLookup
	admins map[string]struct{}
}

// NewGuard builds a guard over an allow-list of addresses. Addresses are
// compared case-insensitively and trimmed, since that is how people type them
// into a deployment's environment.
//
// An empty list denies everyone. That is the intended posture for a deployment
// that has not configured administrators: an admin API that defaults to open
// is a far worse failure than one that defaults to unreachable.
func NewGuard(users UserLookup, emails []string) *Guard {
	admins := make(map[string]struct{}, len(emails))
	for _, e := range emails {
		if norm := strings.ToLower(strings.TrimSpace(e)); norm != "" {
			admins[norm] = struct{}{}
		}
	}
	return &Guard{users: users, admins: admins}
}

// Enabled reports whether any administrator is configured.
func (g *Guard) Enabled() bool { return len(g.admins) > 0 }

type ctxKey struct{}

// Caller returns the administrator behind a request that came through Require.
func Caller(r *http.Request) (models.User, bool) {
	u, ok := r.Context().Value(ctxKey{}).(models.User)
	return u, ok
}

// Require admits only signed-in callers whose verified address is on the
// allow-list, and attaches the resolved account for the handler.
//
// It must be chained *after* auth.AuthMiddleware, which is what establishes
// that the bearer token is real; this decides only whether the person it
// names is an administrator.
func (g *Guard) Require(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uc, ok := auth.GetUserContext(r)
		if !ok || uc.IsGuest {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		oid, err := bson.ObjectIDFromHex(uc.UserID)
		if err != nil {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		u, err := g.users.FindByID(r.Context(), oid)
		if err != nil {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		// The verified check is what stops the allow-list from being defeated
		// by simply claiming an administrator's address: an unverified email
		// on an account proves nothing about who holds it.
		if !u.EmailVerified {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if _, ok := g.admins[strings.ToLower(strings.TrimSpace(u.Email))]; !ok {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, u)))
	})
}
