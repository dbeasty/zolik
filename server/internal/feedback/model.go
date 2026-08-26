package feedback

import (
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Report is one thing a player told us — a bug, an idea, or anything else.
//
// It deliberately denormalises who sent it (username, and the account or guest
// id) rather than pointing at a user document. Most reports arrive from guests,
// who have no account to point at; and a report has to survive its author being
// deleted, since "the account that hit this bug is gone" must not take the bug
// report with it.
type Report struct {
	ID bson.ObjectID `bson:"_id,omitempty" json:"id"`

	Kind    string `bson:"kind" json:"kind"`
	Message string `bson:"message" json:"message"`

	// UserID is set for a signed-in reporter, GuestID for a guest — at most
	// one of them, and neither if the report came in with no session at all.
	UserID  string `bson:"userId,omitempty" json:"userId,omitempty"`
	GuestID string `bson:"guestId,omitempty" json:"guestId,omitempty"`
	// Username is who they were called at the time of writing.
	Username string `bson:"username,omitempty" json:"username,omitempty"`
	// ContactEmail is optional and supplied by the reporter, so it is not a
	// verified address and must never be treated as one — it is somewhere to
	// write back, nothing more.
	ContactEmail string `bson:"contactEmail,omitempty" json:"contactEmail,omitempty"`

	// Context the client attaches so a report is actionable without a
	// conversation: which build, which platform, and which match if the
	// report came from inside one.
	AppVersion string `bson:"appVersion,omitempty" json:"appVersion,omitempty"`
	Platform   string `bson:"platform,omitempty" json:"platform,omitempty"`
	MatchID    string `bson:"matchId,omitempty" json:"matchId,omitempty"`

	Status string `bson:"status" json:"status"`
	// Note is the operator's own scribble, never shown to the reporter.
	Note string `bson:"note,omitempty" json:"note,omitempty"`

	CreatedAt time.Time `bson:"createdAt" json:"createdAt"`
	UpdatedAt time.Time `bson:"updatedAt" json:"updatedAt"`
}

// The kinds a report can have. Anything else is rejected rather than coerced,
// so the admin console's filters always describe the whole collection.
const (
	KindBug   = "bug"
	KindIdea  = "idea"
	KindOther = "other"
)

// The triage states. A report arrives as StatusNew; the console moves it.
const (
	StatusNew      = "new"
	StatusOpen     = "open"
	StatusResolved = "resolved"
)

// MaxMessageLen caps a report body. Long enough for someone to describe what
// happened and paste an error, short enough that the endpoint cannot be used
// to store arbitrary blobs.
const MaxMessageLen = 4000

// maxContextLen caps each client-supplied context field. These are labels, not
// prose, and a client that sends something enormous is either broken or
// probing.
const maxContextLen = 200

// ValidKind reports whether a kind is one this server accepts.
func ValidKind(kind string) bool {
	switch kind {
	case KindBug, KindIdea, KindOther:
		return true
	}
	return false
}

// ValidStatus reports whether a status is one this server accepts.
func ValidStatus(status string) bool {
	switch status {
	case StatusNew, StatusOpen, StatusResolved:
		return true
	}
	return false
}

// clip trims a string and cuts it to at most n runes.
//
// Runes rather than bytes: cutting a UTF-8 string mid-character would store
// an invalid sequence, and the messages this handles are routinely not ASCII —
// this is a Czech card game.
func clip(s string, n int) string {
	s = strings.TrimSpace(s)
	if len([]rune(s)) <= n {
		return s
	}
	return string([]rune(s)[:n])
}
