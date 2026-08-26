package admin

import (
	"context"
	"net/http"
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/auth"
	"zolik/server/internal/models"
	"zolik/server/internal/ratelimit"
)

// UserLookup resolves a signed-in caller to their account. An interface rather
// than the concrete repository so the guard can be tested without a database.
type UserLookup interface {
	FindByID(ctx context.Context, id bson.ObjectID) (models.User, error)
}

// Guard decides who may use the admin API.
//
// There are two ways in, and a deployment may configure either, both, or
// neither.
//
// The allow-list (ADMIN_EMAILS) names ordinary accounts, which sign in the way
// every other player does. Membership is configuration rather than a flag on
// the user document: there is then no bootstrap problem, removal takes effect
// on the very next request rather than at token expiry, and nothing the
// running system exposes can grant it.
//
// The password login (ADMIN_USERNAME / ADMIN_PASSWORD_HASH) is the way in that
// depends on nothing else — see PasswordLogin. The allow-list path needs mail
// delivery to work, and "the console is unreachable" and "mail is broken" are
// conditions that like to occur together.
//
// Configuring neither denies everyone. That is the intended posture for a
// deployment that has not thought about it: an admin API that defaults to open
// is a far worse failure than one that defaults to unreachable.
type Guard struct {
	users    UserLookup
	admins   map[string]struct{}
	password PasswordLogin
	logins   *ratelimit.Limiter
}

// NewGuard builds a guard over an allow-list of addresses and an optional
// password login. Addresses are compared case-insensitively and trimmed, since
// that is how people type them into a deployment's environment.
func NewGuard(users UserLookup, emails []string, password PasswordLogin) *Guard {
	admins := make(map[string]struct{}, len(emails))
	for _, e := range emails {
		if norm := strings.ToLower(strings.TrimSpace(e)); norm != "" {
			admins[norm] = struct{}{}
		}
	}
	return &Guard{
		users:    users,
		admins:   admins,
		password: PasswordLogin{Username: strings.TrimSpace(password.Username), Hash: strings.TrimSpace(password.Hash)},
		logins:   ratelimit.New(loginWindow, loginLimit),
	}
}

// Enabled reports whether any way into the console is configured.
func (g *Guard) Enabled() bool { return len(g.admins) > 0 || g.password.Enabled() }

// Admin is who is behind an admin request.
type Admin struct {
	// User is the account behind an allow-list administrator. It is the zero
	// value for a password sign-in, which has no account — so anything keyed
	// on identity must check ViaPassword rather than assume an id.
	User models.User
	// Username and Email describe the caller whichever door they came through.
	Username    string
	Email       string
	ViaPassword bool
}

type ctxKey struct{}

// Caller returns the administrator behind a request that came through Require.
func Caller(r *http.Request) (Admin, bool) {
	a, ok := r.Context().Value(ctxKey{}).(Admin)
	return a, ok
}

// bearer pulls the token out of an Authorization header.
func bearer(r *http.Request) string {
	parts := strings.Fields(strings.TrimSpace(r.Header.Get("Authorization")))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return parts[1]
}

// Require admits an administrator and attaches who they are.
//
// It does its own token handling rather than chaining behind
// auth.AuthMiddleware, because a console token is deliberately not a player
// token — it is signed with a different key — and that middleware would reject
// it as malformed before this ever saw it.
func (g *Guard) Require(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := bearer(r)
		if token == "" {
			http.Error(w, "missing Authorization header", http.StatusUnauthorized)
			return
		}

		// The console token is tried first: it needs no database round trip,
		// and it is the only thing that works when mail is down.
		if g.password.Enabled() {
			if username, err := g.password.parseToken(token); err == nil {
				admin := Admin{Username: username, ViaPassword: true}
				next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, admin)))
				return
			}
		}

		admin, ok := g.allowListAdmin(r.Context(), token)
		if !ok {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, admin)))
	})
}

// allowListAdmin resolves a player access token to an allow-listed account.
func (g *Guard) allowListAdmin(ctx context.Context, token string) (Admin, bool) {
	claims, err := auth.ParseAccessClaims(token)
	if err != nil || claims.IsGuest {
		return Admin{}, false
	}
	oid, err := bson.ObjectIDFromHex(claims.Subject)
	if err != nil {
		return Admin{}, false
	}
	u, err := g.users.FindByID(ctx, oid)
	if err != nil {
		return Admin{}, false
	}
	// The verified check is what stops the allow-list from being defeated by
	// simply claiming an administrator's address: an unverified email on an
	// account proves nothing about who holds it.
	if !u.EmailVerified {
		return Admin{}, false
	}
	if _, ok := g.admins[strings.ToLower(strings.TrimSpace(u.Email))]; !ok {
		return Admin{}, false
	}
	return Admin{User: u, Username: u.Username, Email: u.Email}, true
}
