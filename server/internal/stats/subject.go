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
	// SubjectAI is a bot, identified by its *persona* — "hard:miroslav" — or,
	// for a bot seated before personas existed, by its difficulty alone.
	//
	// Not by the per-lobby bot instance: those ids are minted fresh every time
	// a bot is added, so aggregating on them would produce a new one-match
	// "player" per lobby and never a usable record. Difficulty was the first
	// answer, and it is stable across matches — but it is also the *only*
	// thing that was stable, so every hard bot that had ever played shared one
	// row and "has Master Miroslav ever beaten me" had nowhere to be asked.
	// A persona is stable and specific: the same opponent, with the same name,
	// at the same strength, every time it sits down. So it keeps its score.
	//
	// The difficulty has not gone anywhere — it is the first half of the key,
	// which is what lets SkillOfSubjectID read it straight back off a subject
	// that carries nothing else, and what keeps the "how do I do against hard?"
	// split working unchanged.
	SubjectAI SubjectKind = "ai"
	// SubjectGuest is an unauthenticated player. Deliberately *not* durable:
	// a guest name is claimed per session and two people can hold the same
	// one, so crediting lifetime statistics to it would merge strangers.
	// Guests are still recorded in the match result and still count as human
	// opponents for everyone else's vs-humans split — they just carry no
	// lifetime record of their own.
	//
	// A guest subject does still carry an ID — the device's durable guest id
	// (models.Session.GuestID) — and therefore a key, so the matches it played
	// can be found again. That is what "play now, sign in later, keep your
	// statistics" rests on: the record of the games exists all along, and
	// signing in re-attributes it (see the auth package's guest claim). What
	// the guest never gets is a lifetime aggregate or a leaderboard row, which
	// is the property Durable controls.
	SubjectGuest SubjectKind = "guest"
)

// Subject is the durable identity behind a seat, as opposed to the per-match
// player ID. It is what lifetime statistics aggregate over.
type Subject struct {
	Kind SubjectKind `bson:"kind" json:"kind"`
	// ID is the user's ObjectID hex for SubjectUser, the persona key
	// ("hard:miroslav") or bare difficulty for SubjectAI, and the device's
	// durable guest id for SubjectGuest — empty on guest seats recorded
	// before guest ids existed, which simply leaves those matches
	// unclaimable.
	ID string `bson:"id,omitempty" json:"id,omitempty"`
	// Name is the display name as of the match being recorded. It is a
	// snapshot for rendering, never a key — users can rename.
	Name string `bson:"name" json:"name"`
}

// Key is the aggregation key: stable, collision-free across kinds, and safe as
// a BSON map key (no dots or leading '$'), since head-to-head records are
// stored as a map keyed by it.
func (s Subject) Key() string {
	if s.ID == "" {
		return ""
	}
	switch s.Kind {
	case SubjectUser:
		return "user:" + s.ID
	case SubjectAI:
		return "ai:" + s.ID
	case SubjectGuest:
		// A guest key indexes the match record so the games can be found and
		// later claimed. It is never a player_stats key — see Durable.
		return "guest:" + s.ID
	default:
		return ""
	}
}

// Durable reports whether this subject accumulates lifetime statistics.
//
// Deliberately not "has a key": a guest has a key (so their matches can be
// looked up and claimed) but no lifetime record, because a guest identity is
// per-device rather than per-person and folding it into leaderboards would
// publish a row nobody owns.
func (s Subject) Durable() bool {
	if s.ID == "" {
		return false
	}
	return s.Kind == SubjectUser || s.Kind == SubjectAI
}

// IsHuman reports whether a person occupied the seat — registered or not.
// This is the test that drives the vs-humans / vs-AI split, so a guest counts
// as human even though they have no lifetime record.
func (s Subject) IsHuman() bool {
	return s.Kind == SubjectUser || s.Kind == SubjectGuest
}

// SkillOfSubjectID is the difficulty behind an AI subject id.
//
// A persona key is "<skill>:<slug>", so the skill is the part before the
// colon; an id with no colon is one of the bare difficulties recorded before
// personas existed, and is its own answer. This is what keeps every
// difficulty-shaped question — the vs-hard split, the ordering of the bot
// leaderboard — working on persona-keyed records without either of them
// storing the skill twice.
func SkillOfSubjectID(id string) string {
	if skill, _, ok := strings.Cut(id, ":"); ok {
		return skill
	}
	return id
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
	case SubjectGuest:
		return Subject{Kind: SubjectGuest, ID: id}, nil
	default:
		return Subject{}, fmt.Errorf("unknown subject kind %q", kind)
	}
}

// SubjectForPlayer derives the durable identity behind one seat of a game.
func SubjectForPlayer(p models.Player) Subject {
	switch {
	case p.IsAI:
		if p.AIPersona != "" {
			return Subject{Kind: SubjectAI, ID: p.AIPersona, Name: p.Name}
		}
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
		return Subject{Kind: SubjectGuest, ID: p.GuestID, Name: p.Name}
	}
}
