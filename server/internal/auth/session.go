package auth

import (
	"context"
	"errors"
	"log"
	"time"

	"golang.org/x/crypto/bcrypt"

	"zolik/server/internal/metrics"
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
	// OfflinePass is the cloud-signed pass this account takes to a host with
	// no internet, empty for a guest and on any deployment that has no
	// asymmetric signing key. See offlinepass.go.
	OfflinePass string
	// SeatReceipt is what an offline host hands a guest so that the seat can
	// be reconciled with an account later. Empty everywhere else, and empty
	// on a host with no node key of its own.
	SeatReceipt string
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
	guestID = sanitizeGuestID(guestID)
	newDevice := guestID == ""
	if newDevice {
		var err error
		if guestID, err = NewRandomToken(16); err != nil {
			return SessionTokens{}, err
		}
	}
	// Named after the id rather than after the moment, so a caller that keeps
	// no name of its own — the SSH host, a returning device whose stored
	// session has lapsed — is greeted as the same player every time instead of
	// as a new one. See GuestNameFor.
	if guestName == "" {
		guestName = GuestNameFor(guestID)
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

	// Counted only when an id had to be minted. A returning device sends the
	// id it already has, and counting every guest *session* would report the
	// same person again every time they opened the tab — which is a measure
	// of how often people reload, not of how many arrived.
	if newDevice {
		h.metrics.Add(metrics.SessionsGuest, 1)
	}

	accessToken, err := CreateAccessToken(guestID, guestName, true, guestAccessTokenTTL)
	if err != nil {
		return SessionTokens{}, err
	}
	// The receipt is issued here, with the seat, because this is the moment
	// the node can honestly attest to: it is the node that seated this guest,
	// and it says so with its own key. A node with no key issues none, and
	// the guest is exactly as well off as they were before receipts existed.
	receipt, err := CreateSeatReceipt(guestID)
	if err != nil && !errors.Is(err, ErrNoNodeKey) {
		log.Printf("auth: seat receipt for %s: %v", guestID, err)
	}
	return SessionTokens{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		UserID:       guestID,
		Username:     guestName,
		IsGuest:      true,
		GuestID:      guestID,
		SeatReceipt:  receipt,
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
		OfflinePass:  h.offlinePassFor(ctx, u.ID.Hex(), u.Username),
	}, nil
}

// offlinePassFor mints the pass that travels with a session, or returns
// nothing at all.
//
// Nothing at all is the right answer in more cases than it looks. A
// deployment with no Ed25519 key signs no passes, and a client that receives
// none simply cannot be seated offline, which is exactly where everyone was
// before passes existed. A credential version this server cannot read is the
// more interesting case: a pass stating the wrong version would go on being
// honoured by an offline host after a revocation this server already knows
// about, so the pass is withheld rather than guessed at.
func (h *Handlers) offlinePassFor(ctx context.Context, userID, username string) string {
	if !isObjectIDHex(userID) {
		return ""
	}
	version := 1
	if h.credentials != nil {
		v, err := h.credentials.CredentialVersion(ctx, userID)
		if err != nil {
			log.Printf("auth: credential version for %s: %v", userID, err)
			return ""
		}
		version = v
	}
	pass, err := CreateOfflinePass(userID, username, version, OfflinePassTTL)
	if err != nil {
		if !errors.Is(err, ErrNoSigningKey) {
			log.Printf("auth: offline pass for %s: %v", userID, err)
		}
		return ""
	}
	return pass
}

// LoginSession authenticates a legacy username/password account. It stays for
// the SSH/TUI client, which can neither open a browser nor read mail.
func (h *Handlers) LoginSession(ctx context.Context, username, password string) (SessionTokens, error) {
	u, err := h.store.FindUserByUsername(ctx, username)
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
