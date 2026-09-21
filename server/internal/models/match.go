package models

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Match is the generic envelope a hosted game lives in.
//
// This is the other half of Phase 3: `Game` is a Žolíky document with
// `drawPile`, `melds`, `meldMeta`, `roundReqMet` as first-class Mongo columns,
// hand-mapped in both directions by a 30-odd field pair of translators. Adding
// one rules field means editing several files, and forgetting one is a silent
// bug.
//
// Here the runtime owns only what every game has — who is playing, whose match
// it is, what version the document is at — and `State` is bytes it never
// reads. A module can add a field to its own state and nothing outside it
// changes, because there is nothing outside it to change.
type Match struct {
	ID bson.ObjectID `bson:"_id,omitempty" json:"id"`

	// ModuleID selects which game this is ("zolik", "prsi"). The runtime uses
	// it to find the module in the registry and for nothing else.
	ModuleID string `bson:"moduleId" json:"moduleId"`
	// Variation and Options are what the lobby chose, kept so a match can say
	// what it was created as without asking the module to decode State.
	Variation string         `bson:"variation,omitempty" json:"variation,omitempty"`
	Options   map[string]int `bson:"options,omitempty" json:"options,omitempty"`

	Status    string   `bson:"status" json:"status"` // lobby | active | completed
	Players   []Player `bson:"players" json:"players"`
	TurnOrder []string `bson:"turnOrder" json:"turnOrder"`
	HostID    string   `bson:"hostId" json:"hostId"`
	JoinCode  string   `bson:"joinCode" json:"joinCode"`

	// State is the module's own game state, opaque here — held in memory and
	// never stored on the match. What is stored is each move, once, and the
	// state at a snapshot now and then (see Snapshots); the runtime rebuilds
	// State from the newest snapshot and the moves after it. So a match loaded
	// straight from a repository has none: ask the Manager, which keeps the
	// state of every match in play.
	//
	// It used to be stored here and rewritten, whole, on every move — along
	// with the log — and a store that keeps every version turned that into the
	// square of a match's length.
	State JSONDoc `bson:"-" json:"-"`

	// Snapshots are the move counts a stored state exists for, oldest first:
	// 0 is the deal, and the last is where a rebuild starts.
	Snapshots []int `bson:"snapshots,omitempty" json:"-"`

	// UpdatedAt is when the match was last written, and — kept within a minute
	// while moves are being made, without a write per move — when it was last
	// played. Activity listings sort by it and the stranded-match sweep reads
	// it. A match written before it existed sorts as never touched, which for
	// a row that old is the right answer.
	UpdatedAt time.Time `bson:"updatedAt,omitempty" json:"updatedAt,omitempty"`

	Seed int64 `bson:"seed" json:"-"`
	// Winners is every player who won. More than one is a real outcome — a
	// Canasta partnership, a split poker pot — and was the first place the
	// module interface was visibly the wrong shape rather than merely
	// unfamiliar with a game.
	Winners []string `bson:"winners,omitempty" json:"winners,omitempty"`
	// WinnerID is Winners[0], kept on the document and the wire so anything
	// written against a single-winner match keeps working. Derived, never
	// computed independently.
	WinnerID  string     `bson:"winnerId,omitempty" json:"winnerId,omitempty"`
	CreatedAt time.Time  `bson:"createdAt" json:"createdAt"`
	StartedAt *time.Time `bson:"startedAt,omitempty" json:"startedAt,omitempty"`
	EndedAt   *time.Time `bson:"endedAt,omitempty" json:"endedAt,omitempty"`

	// Suspension: the table is waiting for a player whose socket dropped.
	//
	// Purely envelope state. The rummy document had to remember which phase it
	// interrupted, because suspension was mixed into the same state machine as
	// drawing and melding; here the module's own state is untouched and there
	// is nothing to restore — which is a small, concrete example of what the
	// opaque-state split bought.
	SuspendedAt     *time.Time `bson:"suspendedAt,omitempty" json:"suspendedAt,omitempty"`
	AbandonAt       *time.Time `bson:"abandonAt,omitempty" json:"abandonAt,omitempty"`
	SuspendedPlayer string     `bson:"suspendedPlayer,omitempty" json:"suspendedPlayer,omitempty"`

	// Version drives the same optimistic-concurrency scheme Game uses: a
	// filtered replace that fails if someone else wrote first.
	Version int64 `bson:"version" json:"-"`

	// MigratedFrom is the `games` document this match was converted from, when
	// it was. It makes the migration idempotent — a re-run skips anything
	// already carrying it — and leaves a trail back to the original if a
	// migrated match ever looks wrong.
	MigratedFrom bson.ObjectID `bson:"migratedFrom,omitempty" json:"-"`
}

// MatchAction is one accepted move, stored verbatim and never rewritten.
//
// The runtime records what it routed without interpreting it, which is what
// makes the log replayable by the module and meaningless to anything else.
// Together with the snapshot before it, it is also how the current state is
// rebuilt: the modules' Apply is deterministic, so the moves since a snapshot
// reproduce the board exactly.
type MatchAction struct {
	Seq      int       `bson:"seq" json:"seq"`
	PlayerID string    `bson:"playerId" json:"playerId"`
	Action   JSONDoc   `bson:"action" json:"action"`
	At       time.Time `bson:"at" json:"at"`
	// Rounds is how many rounds were complete after this move, set only on a
	// move that closed one — "the end of deal 3". A replay's chapters are read
	// off these marks, so they cost a move nothing but a number.
	Rounds int `bson:"rounds,omitempty" json:"rounds,omitempty"`
}
