package stats

import (
	"fmt"
	"strings"

	"zolik/server/internal/models"
)

// SubjectKind distinguishes the three sorts of thing that can occupy a seat.
type SubjectKind string

const (
	// SubjectUser is a registered account. Durable: it accumulates lifetime
	// statistics and appears in leaderboards and head-to-head records.
	SubjectUser SubjectKind = "user"
	// SubjectAI is a bot, identified by its *difficulty* rather than by the
	// per-lobby bot instance. Bot instance IDs look like
	// "ai:medium:1724280000000000000:0" — a fresh one every time a bot is
	// added — so aggregating on them would produce a new one-match "player"
	// per lobby and never a usable record. Difficulty is the thing that is
	// stable across matches and the thing anyone actually wants a number for
	// ("how do I do against hard?"), so that is the subject.
	SubjectAI SubjectKind = "ai"
	// SubjectGuest is an unauthenticated player. Deliberately *not* durable:
	// a guest name is claimed per session and two people can hold the same
	// one, so crediting lifetime statistics to it would merge strangers.
	// Guests are still recorded in the match result and still count as human
	// opponents for everyone else's vs-humans split — they just carry no
	// lifetime record of their own.
	SubjectGuest SubjectKind = "guest"
)

// Subject is the durable identity behind a seat, as opposed to the per-match
// player ID. It is what lifetime statistics aggregate over.
type Subject struct {
	Kind SubjectKind `bson:"kind" json:"kind"`
	// ID is the user's ObjectID hex for SubjectUser, the difficulty
	// ("easy"|"medium"|"hard") for SubjectAI, and empty for SubjectGuest.
	ID string `bson:"id,omitempty" json:"id,omitempty"`
	// Name is the display name as of the match being recorded. It is a
	// snapshot for rendering, never a key — users can rename.
	Name string `bson:"name" json:"name"`
}

// Key is the aggregation key: stable, collision-free across kinds, and safe as
// a BSON map key (no dots or leading '$'), since head-to-head records are
// stored as a map keyed by it.
func (s Subject) Key() string {
	switch s.Kind {
	case SubjectUser:
		return "user:" + s.ID
	case SubjectAI:
		return "ai:" + s.ID
	default:
		return ""
	}
}

// Durable reports whether this subject accumulates lifetime statistics.
func (s Subject) Durable() bool { return s.Key() != "" }

// IsHuman reports whether a person occupied the seat — registered or not.
// This is the test that drives the vs-humans / vs-AI split, so a guest counts
// as human even though they have no lifetime record.
func (s Subject) IsHuman() bool {
	return s.Kind == SubjectUser || s.Kind == SubjectGuest
}

// ParseSubjectKey is the inverse of Key, for reading a key back off a request
// or a head-to-head map. The returned Subject carries no Name.
func ParseSubjectKey(key string) (Subject, error) {
	kind, id, ok := strings.Cut(key, ":")
	if !ok || id == "" {
		return Subject{}, fmt.Errorf("malformed subject key %q", key)
	}
	switch SubjectKind(kind) {
	case SubjectUser:
		return Subject{Kind: SubjectUser, ID: id}, nil
	case SubjectAI:
		return Subject{Kind: SubjectAI, ID: id}, nil
	default:
		return Subject{}, fmt.Errorf("unknown subject kind %q", kind)
	}
}

// SubjectForPlayer derives the durable identity behind one seat of a game.
func SubjectForPlayer(p models.Player) Subject {
	switch {
	case p.IsAI:
		diff := p.AIDifficulty
		if diff == "" {
			// Bots added before difficulty was recorded, and any future bot
			// whose difficulty failed to persist, aggregate under one bucket
			// rather than silently joining "easy".
			diff = "unspecified"
		}
		return Subject{Kind: SubjectAI, ID: diff, Name: p.Name}
	case p.UserID != "":
		return Subject{Kind: SubjectUser, ID: p.UserID, Name: p.Name}
	default:
		return Subject{Kind: SubjectGuest, Name: p.Name}
	}
}
