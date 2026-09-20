package models

import (
	"encoding/json"
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

	// State is the module's own game state, opaque here. Stored as raw bytes
	// rather than a decoded document so Mongo never imposes a schema on it and
	// the runtime is structurally unable to read it.
	State json.RawMessage `bson:"state,omitempty" json:"-"`

	// ActionLog is append-only history, in the module's own vocabulary. Versioned
	// by ModuleID so an old replay stays readable by the module that wrote it.
	ActionLog []MatchAction `bson:"actionLog,omitempty" json:"-"`

	// Checkpoints are the board at each round boundary, oldest first.
	//
	// Absent on every match played before they existed, which is exactly what
	// an old replay gets: the fold starts at the deal, as it always did.
	Checkpoints []MatchCheckpoint `bson:"checkpoints,omitempty" json:"-"`

	// UpdatedAt is set on every write that goes through UpdateWithVersion —
	// every accepted action, suspend, resume and reap — so a listing keyed on
	// activity rather than creation has something to sort by. A match written
	// before this field existed sorts as if it had never been touched, which
	// for a row that old is the right answer.
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

// MatchCheckpoint is the board at a boundary the game itself defines — the
// end of a deal, a hand, a leg.
//
// It is a landmark before it is an optimisation. A six-hundred move replay has
// nothing in it a player recognises, and a slider with six hundred positions
// is not a way to find the deal where everything went wrong; a list of deals
// is. That the same marks also let a fold start near where it is going,
// instead of at the deal every time, is the second thing they are for.
//
// The state is stored rather than derived, so reaching a checkpoint costs
// nothing and depends on nothing — not on the module still folding an old log
// the same way, and not on the log being intact before this point. Which is
// also the answer to a rules change: everything from the last checkpoint
// onwards is still readable when the actions before it have stopped replaying.
type MatchCheckpoint struct {
	// Seq is how many actions had been accepted when this was taken, so it is
	// also the replay frame index this state belongs to.
	Seq int `bson:"seq" json:"seq"`
	// Round is how many rounds were complete here, 1-based for the round this
	// closes — "the end of deal 3".
	Round int `bson:"round" json:"round"`
	// State is the board here — but only on some of them.
	//
	// Every round boundary is marked, because every one is somewhere a player
	// might want to jump to; a mark is three numbers and costs nothing. A
	// stored board is kilobytes, and is only worth keeping where it saves a
	// fold worth saving. Games disagree wildly about how long a round is — a
	// canasta deal runs hundreds of moves, a blackjack hand about nine — so
	// "snapshot every round" measured at +11% of the action log for canasta
	// and +313% for blackjack. Hence the spacing rule in the runtime, and
	// hence this being optional rather than assumed.
	State json.RawMessage `bson:"state,omitempty" json:"-"`
	At    time.Time       `bson:"at" json:"at"`
}

// MatchAction is one accepted move, stored verbatim.
//
// The runtime records what it routed without interpreting it, which is what
// makes the log replayable by the module and meaningless to anything else.
type MatchAction struct {
	Seq      int             `bson:"seq" json:"seq"`
	PlayerID string          `bson:"playerId" json:"playerId"`
	Action   json.RawMessage `bson:"action" json:"action"`
	At       time.Time       `bson:"at" json:"at"`
}
