package auth

import (
	"context"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"
	"unicode"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"zolik/server/internal/identity"
	"zolik/server/internal/models"
)

// GuestClaimer moves a device's guest play history onto a real account.
//
// It is an interface here, satisfied by stats.Claimer and injected at startup,
// because the stats package imports this one for its request middleware — so
// this package cannot import it back. The same shape as game.MatchRecorder,
// and for the same reason.
//
// A nil claimer is a supported configuration: claiming simply does not happen,
// and sign-in still works.
type GuestClaimer interface {
	ClaimGuestHistory(ctx context.Context, guestID, userID, username string) (int, error)
	GuestMatchCount(ctx context.Context, guestID string) (int, error)
}

// Accounts resolves proven identities into accounts.
//
// It is the one place in the server that decides whether a set of claims means
// "sign this existing person in", "attach this to the account they already
// have", or "create somebody new" — and every entry point (Google, email code,
// a future Apple button) goes through it, so those three answers cannot start
// differing per provider.
type Accounts struct {
	store   *Store
	claimer GuestClaimer
}

func NewAccounts(store *Store, claimer GuestClaimer) *Accounts {
	return &Accounts{store: store, claimer: claimer}
}

// SignInOptions carries the context of a sign-in that is not part of the
// identity itself.
type SignInOptions struct {
	// GuestID, when set, is the device's guest identity whose play history
	// should be absorbed into the resulting account.
	GuestID string
	// LinkToUserID, when set, means an already-signed-in account is adding
	// this identity rather than someone signing in with it. The distinction
	// matters: without it, a signed-in player who taps "link Google" and
	// happens to pick a Google account that already has its own profile here
	// would be silently switched into that other profile.
	LinkToUserID string
}

// SignInResult is what the handlers turn into tokens.
type SignInResult struct {
	User models.User
	// Created reports that this sign-in brought a new account into existence.
	Created bool
	// Linked reports that the identity was attached to an account that already
	// existed, rather than being the identity that signed in.
	Linked bool
	// ClaimedMatches is how many guest matches moved onto the account.
	ClaimedMatches int
}

// SignIn resolves proven claims into an account, creating or linking as
// needed, and absorbs any guest history the caller passed along.
//
// The resolution order is the security-relevant part:
//
//  1. An identity we already know wins outright — it is the strongest possible
//     evidence, and it is immune to the person's email address changing hands.
//  2. Otherwise, a *verified* address matching an existing account attaches
//     this new identity to it, so signing in with Google after having signed
//     up by email lands on one account instead of two.
//  3. Otherwise a new account is created.
//
// Step 2 is exactly where account-takeover bugs live, which is why it demands
// verification from both sides: the provider must vouch for the address, and
// the existing account's address must itself have been proven.
func (a *Accounts) SignIn(ctx context.Context, claims identity.Claims, opts SignInOptions) (SignInResult, error) {
	if !claims.Valid() {
		return SignInResult{}, errors.New("identity claims carry no subject")
	}
	claims.Email = NormalizeEmail(claims.Email)

	res, err := a.resolve(ctx, claims, opts, 0)
	if err != nil {
		return SignInResult{}, err
	}

	// Claiming is best-effort on purpose. Somebody who has just proved who
	// they are must end up signed in even if moving their old guest games
	// fails; the games are still on record under the guest id and the claim
	// can be retried from the account screen.
	if opts.GuestID != "" {
		claimed, err := a.ClaimGuest(ctx, opts.GuestID, res.User)
		if err != nil {
			log.Printf("auth: claiming guest history %s for user %s failed: %v",
				opts.GuestID, res.User.ID.Hex(), err)
		}
		res.ClaimedMatches = claimed
	}
	return res, nil
}

// resolve does the work of SignIn minus the guest claim. attempt bounds the
// one retry a lost creation race can trigger: losing twice in a row means
// something other than contention is wrong (an identity being created and
// deleted underneath us), and looping on it would turn a data problem into a
// request that never returns.
func (a *Accounts) resolve(ctx context.Context, claims identity.Claims, opts SignInOptions, attempt int) (SignInResult, error) {
	if attempt > 8 {
		return SignInResult{}, fmt.Errorf("could not resolve identity %s/%s after %d attempts",
			claims.Provider, claims.Subject, attempt)
	}
	existing, err := a.store.FindIdentity(ctx, claims.Provider, claims.Subject)
	switch {
	case err == nil:
		// The identity is known. If a signed-in account was trying to link it,
		// it either already belongs to them (a no-op) or to somebody else (a
		// refusal — never a silent account switch).
		if opts.LinkToUserID != "" && existing.UserID != opts.LinkToUserID {
			return SignInResult{}, ErrIdentityTaken
		}
		u, err := a.store.FindUserByID(ctx, existing.UserID)
		if err != nil {
			// An identity pointing at a user that no longer exists is a
			// corrupt row, not a login. Failing loudly beats resurrecting a
			// deleted account.
			return SignInResult{}, fmt.Errorf("identity %s/%s points at missing user %s",
				claims.Provider, claims.Subject, existing.UserID)
		}
		_ = a.store.TouchIdentity(ctx, existing.ID, claims.Email, claims.Name)
		a.touchUser(ctx, u, claims)
		return SignInResult{User: u, Linked: opts.LinkToUserID != ""}, nil

	case !errors.Is(err, ErrNotFound):
		return SignInResult{}, err
	}

	// The identity is new: find the account it should attach to, or make one.
	owner, created, err := a.ownerFor(ctx, claims, opts)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			// Lost a race to even create the account backing this identity —
			// a concurrent racer for the *same* identity claimed the
			// username or (verified) email first, and createUser has already
			// exhausted its own retries picking a free username. The winner
			// is presumably about to attach their identity (or just did);
			// re-resolving either finds it now or hits this same collision
			// again, which converges once they finish — see the ErrIdentityTaken
			// case below, which this mirrors for the account-creation half of
			// the same race.
			return a.resolve(ctx, claims, opts, attempt+1)
		}
		return SignInResult{}, err
	}

	_, err = a.store.InsertIdentity(ctx, models.Identity{
		UserID:      owner.ID.Hex(),
		Provider:    claims.Provider,
		Subject:     claims.Subject,
		Email:       claims.Email,
		DisplayName: claims.Name,
		CreatedAt:   time.Now().UTC(),
	})
	if errors.Is(err, ErrIdentityTaken) {
		// Lost a race against a concurrent first sign-in. The winner has
		// created the identity we were about to; re-resolving lands both
		// requests on the same account. Any account we created a moment ago is
		// left behind unused rather than being deleted — an orphan row is
		// harmless, whereas deleting one the winner might have picked is not.
		if created {
			log.Printf("auth: lost identity race for %s/%s, abandoning fresh account %s",
				claims.Provider, claims.Subject, owner.ID.Hex())
		}
		return a.resolve(ctx, claims, opts, attempt+1)
	}
	if err != nil {
		return SignInResult{}, err
	}

	a.touchUser(ctx, owner, claims)
	return SignInResult{User: owner, Created: created, Linked: !created}, nil
}

// ownerFor picks the account a brand-new identity belongs to.
func (a *Accounts) ownerFor(ctx context.Context, claims identity.Claims, opts SignInOptions) (models.User, bool, error) {
	// An explicit link request names its account and never guesses.
	if opts.LinkToUserID != "" {
		u, err := a.store.FindUserByID(ctx, opts.LinkToUserID)
		if err != nil {
			return models.User{}, false, err
		}
		return u, false, nil
	}

	// Automatic merge, only on evidence from both sides (see SignIn's doc).
	if claims.EmailVerified && claims.Email != "" {
		u, err := a.store.FindUserByVerifiedEmail(ctx, claims.Email)
		if err == nil {
			return u, false, nil
		}
		if !errors.Is(err, ErrNotFound) {
			return models.User{}, false, err
		}
	}

	u, err := a.createUser(ctx, claims)
	return u, true, err
}

// createUser inserts a brand-new account.
//
// uniqueUsername's availability check is a read, and the insert that follows
// it is a separate write — the classic check-then-act gap, and one that
// concurrent first sign-ins for the same identity open wide: every racer
// suggests the same base name, all of them see it free, and only the first
// insert actually claims it. Rather than fail the rest with a raw duplicate
// key error, a losing insert re-picks a name (now correctly seeing the
// winner's as taken) and tries again, bounded so a genuinely broken database
// cannot loop forever.
func (a *Accounts) createUser(ctx context.Context, claims identity.Claims) (models.User, error) {
	base := suggestUsername(claims)
	var lastErr error
	for attempt := 0; attempt < 5; attempt++ {
		username, err := a.uniqueUsername(ctx, base)
		if err != nil {
			return models.User{}, err
		}
		u := models.User{
			Username:      username,
			Email:         claims.Email,
			EmailVerified: claims.EmailVerified && claims.Email != "",
			AuthProvider:  claims.Provider,
			AvatarURL:     claims.Picture,
			Preferences: models.UserPreferences{
				Language:  "en",
				CardStyle: "classic",
			},
		}
		// No lifetime statistics row is seeded: a record is created the first
		// time the player finishes a match, and an account that has never
		// played simply has none, which the stats endpoint renders as zeroes.
		created, err := a.store.InsertUser(ctx, u)
		if err == nil {
			return created, nil
		}
		if !mongo.IsDuplicateKeyError(err) {
			return models.User{}, err
		}
		lastErr = err
	}
	return models.User{}, fmt.Errorf("could not create an account after retrying past username collisions: %w", lastErr)
}

// touchUser keeps the account's denormalised contact details in step with what
// the provider last said, without ever downgrading a proven address.
func (a *Accounts) touchUser(ctx context.Context, u models.User, claims identity.Claims) {
	set := bson.M{"lastSeenAt": time.Now().UTC()}
	if claims.Email != "" && claims.EmailVerified && !u.EmailVerified {
		set["email"] = claims.Email
		set["emailVerified"] = true
	}
	if u.AvatarURL == "" && claims.Picture != "" {
		set["avatarUrl"] = claims.Picture
	}
	if err := a.store.UpdateUser(ctx, u.ID, set); err != nil {
		log.Printf("auth: updating user %s after sign-in failed: %v", u.ID.Hex(), err)
	}
}

// ClaimGuest absorbs a device's guest play history into an account.
//
// The guest id is recorded as an identity purely so the unique index refuses a
// second claim: one device's history belongs to exactly one account, and two
// people signing in on a shared phone must not both walk off with the same
// games. A history already claimed by this same account re-runs the move,
// which is harmless and repairs a claim that failed halfway.
func (a *Accounts) ClaimGuest(ctx context.Context, guestID string, u models.User) (int, error) {
	if guestID == "" || a.claimer == nil {
		return 0, nil
	}
	userID := u.ID.Hex()

	_, err := a.store.InsertIdentity(ctx, models.Identity{
		UserID:      userID,
		Provider:    models.IdentityProviderGuest,
		Subject:     guestID,
		DisplayName: u.Username,
		CreatedAt:   time.Now().UTC(),
	})
	if errors.Is(err, ErrIdentityTaken) {
		owner, findErr := a.store.FindIdentity(ctx, models.IdentityProviderGuest, guestID)
		if findErr != nil {
			return 0, findErr
		}
		if owner.UserID != userID {
			// Claimed by somebody else on this device. Not an error worth
			// failing a sign-in over — the person is signed in, they simply
			// have no guest history to inherit.
			return 0, nil
		}
	} else if err != nil {
		return 0, err
	}

	return a.claimer.ClaimGuestHistory(ctx, guestID, userID, u.Username)
}

// GuestMatchCount reports how many recorded matches a guest id has, for the
// "sign in to keep your N games" prompt.
func (a *Accounts) GuestMatchCount(ctx context.Context, guestID string) int {
	if guestID == "" || a.claimer == nil {
		return 0
	}
	n, err := a.claimer.GuestMatchCount(ctx, guestID)
	if err != nil {
		return 0
	}
	return n
}

// Identities lists an account's sign-in methods.
func (a *Accounts) Identities(ctx context.Context, userID string) ([]models.Identity, error) {
	return a.store.ListIdentities(ctx, userID)
}

// Unlink removes a sign-in method from an account, refusing to remove the last
// one.
//
// Without that refusal the account screen would offer a button that locks the
// person out of their own statistics permanently — the failure mode is silent,
// irreversible and entirely self-inflicted, which is the worst combination.
// A legacy password counts as a way back in.
func (a *Accounts) Unlink(ctx context.Context, userID, provider string) error {
	if provider == models.IdentityProviderGuest {
		// Unlinking a claimed guest id would free it to be claimed again by
		// another account, which is a way to launder history rather than a
		// feature anybody wants.
		return errors.New("a claimed guest history cannot be unlinked")
	}
	ids, err := a.store.ListIdentities(ctx, userID)
	if err != nil {
		return err
	}
	remaining := 0
	found := false
	for _, id := range ids {
		if id.Provider == models.IdentityProviderGuest {
			continue
		}
		if id.Provider == provider {
			found = true
			continue
		}
		remaining++
	}
	if !found {
		return ErrNotFound
	}
	if remaining == 0 {
		u, err := a.store.FindUserByID(ctx, userID)
		if err != nil {
			return err
		}
		if u.PasswordHash == "" {
			return errors.New("this is the only way to sign in to this account")
		}
	}
	return a.store.DeleteIdentity(ctx, userID, provider)
}

// --- display names ---

var usernameSanitizer = regexp.MustCompile(`[^\p{L}\p{N}_. -]+`)

// suggestUsername proposes a display name for a brand-new account, in
// descending order of how much the person would recognise it.
//
// Nothing here is authoritative — it is a starting point the player can change
// on the account screen — but a good default matters: "Guest4823" as a
// permanent name is the kind of small indignity that makes an account feel
// like a formality rather than theirs.
func suggestUsername(claims identity.Claims) string {
	if n := cleanUsername(claims.Name); n != "" {
		return n
	}
	if local, _, ok := strings.Cut(claims.Email, "@"); ok {
		if n := cleanUsername(local); n != "" {
			return n
		}
	}
	return "Player"
}

func cleanUsername(s string) string {
	s = strings.TrimSpace(usernameSanitizer.ReplaceAllString(s, ""))
	s = strings.Join(strings.Fields(s), " ")
	if len([]rune(s)) > 24 {
		s = string([]rune(s)[:24])
	}
	// A name made only of punctuation would pass the length check while being
	// unreadable, so require at least one letter or digit.
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return s
		}
	}
	return ""
}

// uniqueUsername appends a numeric suffix until the name is free.
//
// The loop is bounded and falls back to a name derived from the clock, because
// an unbounded retry on a contended base name is a request that never returns.
// The unique index remains the actual guarantee — this only avoids provoking
// it in the common case.
func (a *Accounts) uniqueUsername(ctx context.Context, base string) (string, error) {
	if base == "" {
		base = "Player"
	}
	for i := 0; i < 50; i++ {
		candidate := base
		if i > 0 {
			candidate = fmt.Sprintf("%s%d", base, i+1)
		}
		taken, err := a.store.UsernameTaken(ctx, candidate)
		if err != nil {
			return "", err
		}
		if !taken {
			return candidate, nil
		}
	}
	return fmt.Sprintf("%s%d", base, time.Now().UnixNano()%1_000_000), nil
}

// NormalizeEmail lower-cases and trims an address so the same mailbox is one
// identity however it was typed.
//
// It stops there on purpose. Stripping dots or +tags is a Gmail-specific rule
// that is wrong for most other hosts, and applying it would silently merge two
// genuinely different mailboxes into one account.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
