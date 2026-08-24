package models

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Identity is one proven way of signing in to one account: "this Google
// subject is this user", "this verified email address is this user", "the
// guest history from this device belongs to this user".
//
// Identities live in their own collection rather than as an array on the user
// document for one reason that matters more than tidiness: the unique index on
// (provider, subject) is what makes it *impossible* for one external identity
// to end up attached to two accounts. That invariant is enforced by the
// database, not by handler code that could race with itself, and it is what
// makes "sign in with Google" idempotent under concurrent first logins.
//
// A user with several identities is the normal case — signing in with Google
// and later adding an email address is linking, not a second account.
type Identity struct {
	ID bson.ObjectID `bson:"_id,omitempty" json:"-"`
	// UserID is the owning account, as a hex ObjectID.
	UserID string `bson:"userId" json:"-"`
	// Provider is the identity.Provider id: "google", "apple", "microsoft",
	// "email", "guest", or "local" for legacy username/password accounts.
	Provider string `bson:"provider" json:"provider"`
	// Subject is the provider's stable id for this person — the Google `sub`,
	// the normalised email address, the device's guest id. Unique with
	// Provider.
	Subject string `bson:"subject" json:"-"`
	// Email and DisplayName are a snapshot of what the provider last told us,
	// kept for rendering the "linked accounts" list. Never used as a key.
	Email       string `bson:"email,omitempty" json:"email,omitempty"`
	DisplayName string `bson:"displayName,omitempty" json:"displayName,omitempty"`

	CreatedAt   time.Time  `bson:"createdAt" json:"linkedAt"`
	LastLoginAt *time.Time `bson:"lastLoginAt,omitempty" json:"lastLoginAt,omitempty"`
}

// Identity provider ids that this server issues itself, as opposed to the
// external ones the identity registry knows about.
const (
	// IdentityProviderEmail is an address proven by a one-time code we mailed.
	IdentityProviderEmail = "email"
	// IdentityProviderGuest records that an account has absorbed the play
	// history of a particular device's guest. It is never a way to *sign in* —
	// it exists so the unique index prevents the same guest history being
	// claimed twice by two different accounts.
	IdentityProviderGuest = "guest"
	// IdentityProviderLocal is the legacy username/password login that
	// predates this system. Kept working for the SSH/TUI client, which cannot
	// open a browser and cannot receive mail.
	IdentityProviderLocal = "local"
)

// LoginCode is a one-time code mailed to an address to prove control of it.
//
// The code itself is never stored: only a hash, exactly as with a password,
// because a leaked database dump of live codes would otherwise be a leak of
// live credentials. Attempts and expiry are on the document so a code can be
// burned by guessing as well as by use.
type LoginCode struct {
	ID bson.ObjectID `bson:"_id,omitempty"`
	// Email is normalised (trimmed, lower-cased) — the same normalisation the
	// identity subject uses, so a code issued to "Foo@Example.com" verifies an
	// identity keyed on "foo@example.com".
	Email string `bson:"email"`
	// CodeHash is a bcrypt hash of the code.
	CodeHash string `bson:"codeHash"`
	// Attempts counts wrong guesses; the code dies after MaxCodeAttempts.
	Attempts int `bson:"attempts"`
	// ConsumedAt marks a code that has been redeemed. Redeemed codes are kept
	// until they expire rather than deleted, so a replayed request gets a
	// clean "already used" instead of an indistinguishable "wrong code".
	ConsumedAt *time.Time `bson:"consumedAt,omitempty"`
	CreatedAt  time.Time  `bson:"createdAt"`
	// ExpiresAt drives a TTL index: expired codes delete themselves, so
	// nothing has to sweep them and a stale code cannot be redeemed even if
	// the sweeper is broken.
	ExpiresAt time.Time `bson:"expiresAt"`
	// RequestIP is recorded for abuse investigation only.
	RequestIP string `bson:"requestIp,omitempty"`
}

// OAuthFlow is the server-side memory of one in-flight browser sign-in.
//
// The redirect flow hands the provider a `state` and a `nonce` and gets them
// back later, possibly on a different server instance — so they live in Mongo
// rather than in process memory, and the flow works behind a load balancer
// with no sticky sessions.
//
// After the provider redirects back, the flow is also where the resulting
// tokens wait for the client to collect them. The client never sees them in a
// URL: the callback hands back a short-lived, single-use ExchangeCode, and the
// app posts that to swap for real tokens. Tokens in a redirect URL end up in
// browser history, server logs and `Referer` headers; a code that dies on
// first use does not.
type OAuthFlow struct {
	ID bson.ObjectID `bson:"_id,omitempty"`
	// State is the CSRF token echoed by the provider, and the lookup key on
	// the way back.
	State string `bson:"state"`
	// Nonce is echoed inside the ID token, tying the token to this flow.
	Nonce    string `bson:"nonce"`
	Provider string `bson:"provider"`
	// RedirectURI is the exact value sent to the provider. The token endpoint
	// requires it byte-for-byte on exchange, so it is stored rather than
	// rebuilt.
	RedirectURI string `bson:"redirectUri"`
	// ReturnTo is the app URL the browser is sent back to at the end — the
	// deep link for the mobile app, or a web route.
	ReturnTo string `bson:"returnTo,omitempty"`
	// LinkUserID is set when the flow was started by an already-signed-in user
	// adding a provider, rather than by someone signing in. It is what stops a
	// link flow from silently becoming a login to a different account.
	LinkUserID string `bson:"linkUserId,omitempty"`
	// GuestID carries the device's guest id through the redirect, so a guest
	// who signs in mid-session has their play history claimed without a second
	// round trip.
	GuestID string `bson:"guestId,omitempty"`

	// ExchangeCode is issued at the callback and swapped by the client for
	// tokens. Empty until the provider has answered.
	ExchangeCode string `bson:"exchangeCode,omitempty"`
	// Result holds the tokens waiting to be collected.
	Result *OAuthFlowResult `bson:"result,omitempty"`

	CreatedAt time.Time `bson:"createdAt"`
	ExpiresAt time.Time `bson:"expiresAt"`
}

// OAuthFlowResult is the outcome of a completed redirect flow, held for the
// few seconds between the provider's callback and the client collecting it.
type OAuthFlowResult struct {
	AccessToken  string `bson:"accessToken"`
	RefreshToken string `bson:"refreshToken"`
	UserID       string `bson:"userId"`
	Username     string `bson:"username"`
	// Linked reports that the flow attached a provider to an existing signed-in
	// account rather than producing a session.
	Linked bool `bson:"linked,omitempty"`
	// ClaimedMatches is how many guest matches were absorbed into the account,
	// so the client can say "we kept your 12 games" instead of guessing.
	ClaimedMatches int `bson:"claimedMatches,omitempty"`
}
