package auth

import (
	"context"
	"errors"
	"time"

	"golang.org/x/crypto/bcrypt"

	"zolik/server/internal/models"
)

// errInvalidCredentials is the single answer to every failed password login,
// so the response cannot be used to discover which usernames exist.
var errInvalidCredentials = errors.New("invalid credentials")

// SessionTokens is one issued session, returned to REST clients and to the
// SSH/TUI host alike.
type SessionTokens struct {
	AccessToken  string
	RefreshToken string
	UserID       string
	Username     string
	IsGuest      bool
	// GuestID is the device's durable guest identity, set only on guest
	// sessions. It is the same value as UserID for a guest — UserID is
	// whatever the JWT subject is — but is named separately because a client
	// must persist *this* across sign-outs and app restarts, and must not
	// persist a user id it may not keep.
	GuestID string
}

// GuestSession starts (or resumes) a guest session with no prior identity.
func (h *Handlers) GuestSession(ctx context.Context, guestName string) (SessionTokens, error) {
	return h.GuestSessionWithID(ctx, guestName, "")
}

// GuestSessionWithID starts a guest session, reusing the device's existing
// guest identity when it has one.
//
// Reuse is the whole point. A guest id minted per session would make every
// session a different person as far as the match records are concerned, and
// there would be nothing coherent to hand over when the player finally signs
// in. One id per device, kept by the client, means the history accumulates in
// one place from the very first game — which is what makes "sign in and keep
// your statistics" a real offer rather than a hopeful one.
//
// An id supplied by the client is trusted on the same terms a bearer token is:
// it identifies a device's play history and nothing else. It grants no access
// to any account, cannot be used to sign in, and stops being claimable the
// moment somebody claims it (the identities collection's unique index).
func (h *Handlers) GuestSessionWithID(ctx context.Context, guestName, guestID string) (SessionTokens, error) {
	if guestName == "" {
		guestName = "Guest"
	}
	guestID = sanitizeGuestID(guestID)
	if guestID == "" {
		var err error
		if guestID, err = NewRandomToken(16); err != nil {
			return SessionTokens{}, err
		}
	}

	refreshToken, err := CreateRefreshToken()
	if err != nil {
		return SessionTokens{}, err
	}
	now := time.Now().UTC()
	if err := h.sessionRepo.CreateSession(ctx, models.Session{
		Token:     refreshToken,
		GuestName: guestName,
		UserID:    "",
		GuestID:   guestID,
		CreatedAt: now,
		ExpiresAt: now.Add(refreshTokenTTL),
	}); err != nil {
		return SessionTokens{}, err
	}

	accessToken, err := CreateAccessToken(guestID, guestName, true, guestAccessTokenTTL)
	if err != nil {
		return SessionTokens{}, err
	}
	return SessionTokens{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		UserID:       guestID,
		Username:     guestName,
		IsGuest:      true,
		GuestID:      guestID,
	}, nil
}

// issueUserSession mints a session for a resolved account. Every sign-in path
// ends here, so there is exactly one place that decides what a session is.
func (h *Handlers) issueUserSession(ctx context.Context, u models.User) (SessionTokens, error) {
	refreshToken, err := CreateRefreshToken()
	if err != nil {
		return SessionTokens{}, err
	}
	now := time.Now().UTC()
	if err := h.sessionRepo.CreateSession(ctx, models.Session{
		Token:     refreshToken,
		GuestName: u.Username,
		UserID:    u.ID.Hex(),
		CreatedAt: now,
		ExpiresAt: now.Add(refreshTokenTTL),
	}); err != nil {
		return SessionTokens{}, err
	}
	accessToken, err := CreateAccessToken(u.ID.Hex(), u.Username, false, accessTokenTTL)
	if err != nil {
		return SessionTokens{}, err
	}
	return SessionTokens{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		UserID:       u.ID.Hex(),
		Username:     u.Username,
		IsGuest:      false,
	}, nil
}

// LoginSession authenticates a legacy username/password account. It stays for
// the SSH/TUI client, which can neither open a browser nor read mail.
func (h *Handlers) LoginSession(ctx context.Context, username, password string) (SessionTokens, error) {
	u, err := h.findUserByUsername(ctx, username)
	if err != nil {
		return SessionTokens{}, err
	}
	if u.PasswordHash == "" {
		// An account created through a provider has no password. Saying so
		// plainly beats a bcrypt comparison against an empty hash, which fails
		// with a confusing error.
		return SessionTokens{}, errInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return SessionTokens{}, errInvalidCredentials
	}
	return h.issueUserSession(ctx, u)
}

// sanitizeGuestID accepts only the shape this server issues — 32 lowercase hex
// characters — so a client cannot supply an arbitrary string that then travels
// into subject keys, match records and BSON map keys.
func sanitizeGuestID(id string) string {
	if len(id) != 32 {
		return ""
	}
	for _, r := range id {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return ""
		}
	}
	return id
}
