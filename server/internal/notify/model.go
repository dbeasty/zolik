// Package notify tells players about tables they might want to sit at.
//
// Two audiences, one vocabulary. A host's *circle* is the people they have
// chosen to tell when they open a table; each of those people keeps the right
// to hear nothing — invites off altogether, or one notifier muted. Delivery is
// an in-app message on the caller's personal socket (/ws/me) when an app is
// open, and an OS push to every registered device otherwise. The client
// suppresses the push when it is already showing the socket's copy, which is
// what lets the server send both without tracking who is looking.
//
// Everyone is addressed by subject key — "user:<hex>" or "guest:<id>", the same
// keys stats.MatchResult carries — so a guest can be in a circle, and signing
// in carries their circle along with their history (see Repository.Rekey).
package notify

import (
	"strings"
	"time"

	"zolik/server/internal/auth"
	"zolik/server/internal/models"
)

// Invites is the recipient's own switch for circle notifications.
type Invites string

const (
	InvitesCircle Invites = "circle"
	InvitesOff    Invites = "off"
)

// Profile is one subject's notification settings, and the friend code that
// lets someone add them without having played them first.
//
// Name and Avatar are the display details last seen for this subject. A guest
// has no account document to read them from, so they are kept here, refreshed
// whenever the subject themself calls in.
type Profile struct {
	Key        string    `bson:"_id" json:"key"`
	FriendCode string    `bson:"friendCode" json:"friendCode"`
	Invites    Invites   `bson:"invites" json:"invites"`
	Nearby     bool      `bson:"nearby" json:"nearby"`
	Name       string    `bson:"name,omitempty" json:"-"`
	Avatar     string    `bson:"avatar,omitempty" json:"-"`
	UpdatedAt  time.Time `bson:"updatedAt" json:"-"`
}

// EdgeStatus is where one "owner notifies member" relationship stands.
type EdgeStatus string

const (
	// EdgeActive means the owner's tables reach the member.
	EdgeActive EdgeStatus = "active"
	// EdgePending is a request by username, waiting for the member to accept.
	EdgePending EdgeStatus = "pending"
	// EdgeRemoved is an edge the owner took away while the member had it
	// muted. Kept only so the mute outlives it: being removed and re-added
	// must not be a way round somebody's decision to stop hearing from you.
	EdgeRemoved EdgeStatus = "removed"
)

// EdgeSource records how an edge came to exist, for the circle screen and for
// whoever next has to reason about consent.
type EdgeSource string

const (
	SourcePlayed   EdgeSource = "played"
	SourceLink     EdgeSource = "link"
	SourceUsername EdgeSource = "username"
)

// Edge is one direction of a circle: Owner's tables are announced to Member.
//
// Muted is the member's choice, stored on the owner's edge because it is a
// fact about that one direction. It is the only field the member writes.
type Edge struct {
	ID        string     `bson:"_id" json:"-"`
	OwnerKey  string     `bson:"ownerKey" json:"-"`
	MemberKey string     `bson:"memberKey" json:"-"`
	Status    EdgeStatus `bson:"status" json:"-"`
	Source    EdgeSource `bson:"source" json:"-"`
	Muted     bool       `bson:"muted,omitempty" json:"-"`
	// OwnerName and MemberName are what each side was called when the edge
	// was made — the fallback when neither has a profile to read a fresher
	// one from.
	OwnerName  string    `bson:"ownerName,omitempty" json:"-"`
	MemberName string    `bson:"memberName,omitempty" json:"-"`
	CreatedAt  time.Time `bson:"createdAt" json:"-"`
}

func edgeID(owner, member string) string { return owner + ">" + member }

// DeviceKind names the push service a device is reached through.
type DeviceKind string

const (
	DeviceExpo    DeviceKind = "expo"
	DeviceWebPush DeviceKind = "webpush"
)

// Subscription is a browser's PushSubscription.toJSON(), as Web Push needs it.
type Subscription struct {
	Endpoint string `bson:"endpoint" json:"endpoint"`
	Keys     struct {
		P256dh string `bson:"p256dh" json:"p256dh"`
		Auth   string `bson:"auth" json:"auth"`
	} `bson:"keys" json:"keys"`
}

// Device is one place a subject can be reached when no app is open.
//
// The id is derived from the token or endpoint, so the same install
// registering again — every launch does — updates one row instead of adding
// another, and a device that changes hands (a guest signing in) moves to the
// new subject instead of notifying both.
type Device struct {
	ID           string        `bson:"_id" json:"id"`
	SubjectKey   string        `bson:"subjectKey" json:"-"`
	Kind         DeviceKind    `bson:"kind" json:"kind"`
	Token        string        `bson:"token,omitempty" json:"-"`
	Subscription *Subscription `bson:"subscription,omitempty" json:"-"`
	Platform     string        `bson:"platform,omitempty" json:"platform,omitempty"`
	Locale       string        `bson:"locale,omitempty" json:"locale,omitempty"`
	LastSeenAt   time.Time     `bson:"lastSeenAt" json:"-"`
}

// KeyFor is the subject key of whoever holds this token.
func KeyFor(uc auth.UserContext) string {
	if uc.IsGuest {
		return "guest:" + uc.UserID
	}
	return "user:" + uc.UserID
}

// KeyForPlayer is the subject key behind a seat, or "" for a bot.
func KeyForPlayer(p models.Player) string {
	switch {
	case p.IsAI:
		return ""
	case p.GuestID != "":
		return "guest:" + p.GuestID
	case p.UserID != "":
		return "user:" + p.UserID
	}
	return ""
}

// validKey accepts only the two human key shapes. Everything else a client
// sends — a bot key, a bare id, a typo — is somebody nobody can notify.
func validKey(k string) bool {
	for _, prefix := range []string{"user:", "guest:"} {
		if strings.HasPrefix(k, prefix) && len(k) > len(prefix) {
			return true
		}
	}
	return false
}
