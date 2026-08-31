package models

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type Player struct {
	ID           string `bson:"id" json:"id"`
	Name         string `bson:"name" json:"name"`
	IsAI         bool   `bson:"isAI" json:"isAI"`
	AIDifficulty string `bson:"aiDifficulty" json:"aiDifficulty,omitempty"`
	ConnectionID string `bson:"connectionId" json:"connectionId,omitempty"`
	UserID       string `bson:"userId" json:"userId,omitempty"`
	// Avatar is the face this seat wears, as a slug the clients agree on.
	//
	// Cosmetic, and deliberately opaque to the server: it is validated for
	// shape and stored, never interpreted. An empty or unrecognised one is
	// not an error — every client derives a face from the player id in that
	// case, identically, so a roster can grow on the client without a server
	// release and an older client never shows a blank seat.
	Avatar string `bson:"avatar,omitempty" json:"avatar,omitempty"`
	// GuestID is set instead of UserID when a guest holds the seat, and is the
	// device's durable guest id (see Session.GuestID). It is what lets a match
	// this seat played be found again — and re-attributed — if the person
	// later creates an account. Empty on seats played before guest ids
	// existed, which simply makes those matches unclaimable, exactly as they
	// were before.
	GuestID string `bson:"guestId,omitempty" json:"-"`
}

type User struct {
	ID       bson.ObjectID `bson:"_id,omitempty" json:"id"`
	Username string        `bson:"username" json:"username"`
	// Email is the address the account is *reachable* at, denormalised from
	// whichever identity carries one so it can be shown without a join. It is
	// deliberately not the login key — that is the identity's (provider,
	// subject) pair, so the same person can hold a Google identity and an
	// email identity with different addresses without either being ambiguous.
	Email string `bson:"email,omitempty" json:"email,omitempty"`
	// EmailVerified reports whether the address above has been proven, either
	// by a one-time code or by a provider that vouches for it. Only a verified
	// address may be used to attach a new provider to this existing account.
	EmailVerified bool `bson:"emailVerified,omitempty" json:"emailVerified"`
	// PasswordHash is set only on legacy username/password accounts. Accounts
	// created through an identity provider or an emailed code have none, and
	// nothing in the system requires one.
	PasswordHash string `bson:"passwordHash,omitempty" json:"-"`
	// AuthProvider records how the account was originally created. It is
	// history, not authorisation: what an account can sign in with is the set
	// of Identity documents pointing at it, never this field.
	AuthProvider string          `bson:"authProvider" json:"authProvider"`
	AvatarURL    string          `bson:"avatarUrl,omitempty" json:"avatarUrl,omitempty"`
	CreatedAt    time.Time       `bson:"createdAt" json:"createdAt"`
	LastSeenAt   time.Time       `bson:"lastSeenAt" json:"lastSeenAt"`
	Preferences  UserPreferences `bson:"preferences" json:"preferences"`
}

type UserPreferences struct {
	Language  string `bson:"language" json:"language"`
	CardStyle string `bson:"cardStyle" json:"cardStyle"`
	// Avatar is the face the account chose, so it follows them to a new
	// device rather than living beside the skin in one browser's storage.
	// Distinct from User.AvatarURL, which is whatever picture an identity
	// provider happened to hand over at sign-in and which nobody picked.
	Avatar string `bson:"avatar,omitempty" json:"avatar,omitempty"`
}

// Statistics and MatchRef used to live here: a per-user counters document
// seeded at registration and read by the stats endpoint, but never written by
// anything. Lifetime records now live in internal/stats, keyed by a subject
// that covers registered users and AI difficulties alike, and are derived
// from the immutable match records written when a match completes.

type Session struct {
	ID        bson.ObjectID `bson:"_id,omitempty" json:"id"`
	Token     string        `bson:"token" json:"token"`
	GuestName string        `bson:"guestName" json:"guestName"`
	UserID    string        `bson:"userId,omitempty" json:"userId,omitempty"`
	// GuestID is the durable identity of an unauthenticated player, empty on
	// sessions belonging to a real account.
	//
	// It exists because the guest's JWT subject used to be the refresh token
	// itself, which rotates on every refresh: the same person's in-game player
	// id changed underneath them mid-session, and nothing they did as a guest
	// could ever be attributed to them afterwards. The guest id is minted once
	// per device, kept by the client across sessions, and is what makes "sign
	// in and keep the statistics you already earned" possible — see
	// auth.ClaimGuestHistory.
	//
	// It is not a credential: possession of the *session* proves ownership of
	// the guest id, which is why claiming requires the guest's refresh token
	// rather than the id on its own.
	GuestID   string    `bson:"guestId,omitempty" json:"guestId,omitempty"`
	CreatedAt time.Time `bson:"createdAt" json:"createdAt"`
	ExpiresAt time.Time `bson:"expiresAt" json:"expiresAt"`
}
