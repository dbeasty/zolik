package auth

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// AccountForgetter is a part of the server that keeps something about an
// account outside this package — its statistics, its seats at tables, its
// circle — and must let go of it when the account is deleted.
//
// ForgetAccount has to be safe to run twice. Deletion runs every forgetter
// before it removes the account itself, so a failure part way leaves the
// account in place for the person to try again, and the parts that already
// ran run again.
type AccountForgetter interface {
	ForgetAccount(ctx context.Context, userID string) error
}

// AccountForgetterFunc lets a plain function be an AccountForgetter.
type AccountForgetterFunc func(ctx context.Context, userID string) error

func (f AccountForgetterFunc) ForgetAccount(ctx context.Context, userID string) error {
	return f(ctx, userID)
}

// AddAccountForgetter has deletion call f, in the order they were added. They
// are supplied after construction because what they forget — the match
// manager, the stats — is built after this package's handlers and imports
// this package for its middleware.
func (h *Handlers) AddAccountForgetter(name string, f AccountForgetter) {
	h.forgetters = append(h.forgetters, namedForgetter{name: name, AccountForgetter: f})
}

type namedForgetter struct {
	name string
	AccountForgetter
}

// deleteAccount deletes the signed-in account, which the person asked for in
// so many words: the stores require that an app which creates accounts lets
// people delete them from inside it, and the privacy policy promises it.
//
// What goes: the account, every way of signing in to it, every session on
// every device, its lifetime record, its circle, its devices and its
// scorepads. What stays is the games, because the other players were there
// too; the account's seats in them are renamed "Player N" and lead nowhere
// (see match.Manager.ForgetAccount and stats.Claimer.ForgetUser).
//
// A token already issued keeps working until it expires — fifteen minutes
// for an account — because nothing checks an access token against the store.
// The client throws its tokens away as soon as this answers.
func (h *Handlers) deleteAccount(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	uc, _ := GetUserContext(req)
	if uc.IsGuest {
		http.Error(w, "guests have no account to delete", http.StatusForbidden)
		return
	}
	if err := h.forgetAccount(ctx, uc.UserID); err != nil {
		internalError(w, "deleteAccount", err)
		return
	}
	log.Printf("auth: deleted account %s", uc.UserID)
	if wantsSessionCookies(req) {
		clearAuthCookie(w, req, refreshCookie)
	}
	writeJSON(w, map[string]any{"deleted": true})
}

// forgetAccount is the deletion itself. Everything outside this package goes
// first and the account last, so the account is only gone once nothing else
// still names it.
func (h *Handlers) forgetAccount(ctx context.Context, userID string) error {
	id, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		return fmt.Errorf("account id %q: %w", userID, err)
	}
	for _, f := range h.forgetters {
		if err := f.ForgetAccount(ctx, userID); err != nil {
			return fmt.Errorf("%s: %w", f.name, err)
		}
	}
	if err := h.nodes.DeleteNodesForOwner(ctx, userID); err != nil {
		return fmt.Errorf("nodes: %w", err)
	}
	if h.guestClaims != nil {
		if err := h.guestClaims.ForgetAccount(ctx, userID); err != nil {
			return fmt.Errorf("guest claims: %w", err)
		}
	}
	identities, err := h.store.ListIdentities(ctx, userID)
	if err != nil {
		return fmt.Errorf("identities: %w", err)
	}
	for _, ident := range identities {
		if err := h.store.DeleteIdentity(ctx, userID, ident.Provider); err != nil && !errors.Is(err, ErrNotFound) {
			return fmt.Errorf("identity %s: %w", ident.Provider, err)
		}
	}
	if err := h.sessionRepo.DeleteForUser(ctx, userID); err != nil {
		return fmt.Errorf("sessions: %w", err)
	}
	if err := h.store.DeleteUser(ctx, id); err != nil {
		return fmt.Errorf("account: %w", err)
	}
	return nil
}
