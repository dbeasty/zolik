package match

import (
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/models"
)

// A match's history as the repositories store it.
//
// A move is stored once, as a small record that is never rewritten. The game
// state is stored only as a snapshot now and then — the deal, round
// boundaries, the end — and rebuilt from the newest snapshot and the moves
// after it, which is exact because every module's Apply is deterministic.
// So a normal move writes about 150 bytes: who moved, what they did, and when.
//
// The alternative this replaced rewrote the whole state, and before that the
// whole log, on every move; under a store that keeps every version that made a
// match's history cost the square of its length.

// Record kinds, in the Mongo collection's "k" field and in kdb keys.
const (
	kindMove     = "m"
	kindSnapshot = "s"
)

// moveRecord is one stored move. Field names are one letter because there is
// one of these per move of every match ever played.
type moveRecord struct {
	Match  bson.ObjectID  `bson:"m"`
	Kind   string         `bson:"k"`
	Seq    int            `bson:"s"`
	Player string         `bson:"p"`
	Action models.JSONDoc `bson:"a"`
	// At is Unix milliseconds, the precision a stored date has anyway.
	At     int64 `bson:"t"`
	Rounds int   `bson:"r,omitempty"`
}

// snapshotRecord is the whole state after Seq moves.
type snapshotRecord struct {
	Match bson.ObjectID  `bson:"m"`
	Kind  string         `bson:"k"`
	Seq   int            `bson:"s"`
	State models.JSONDoc `bson:"state"`
}

func toMoveRecord(id bson.ObjectID, a models.MatchAction) moveRecord {
	return moveRecord{Match: id, Kind: kindMove, Seq: a.Seq, Player: a.PlayerID,
		Action: a.Action, At: a.At.UnixMilli(), Rounds: a.Rounds}
}

func (r moveRecord) action() models.MatchAction {
	return models.MatchAction{Seq: r.Seq, PlayerID: r.Player, Action: r.Action,
		At: time.UnixMilli(r.At).UTC(), Rounds: r.Rounds}
}

// logKey addresses a record in a store keyed by string (kdb): the match, the
// kind and the seq, so a match's moves are found by counting rather than by
// searching.
func logKey(id bson.ObjectID, kind string, seq int) string {
	return fmt.Sprintf("%s/%s/%d", id.Hex(), kind, seq)
}

// latestSnapshot is the newest snapshot a match lists, and whether it has any.
func latestSnapshot(m models.Match) (int, bool) {
	if len(m.Snapshots) == 0 {
		return 0, false
	}
	return m.Snapshots[len(m.Snapshots)-1], true
}

// SnapshotDocument is the stored form of the state after seq moves, for the
// one-shot tools that write the match_log collection directly.
func SnapshotDocument(id bson.ObjectID, seq int, state models.JSONDoc) any {
	return snapshotRecord{Match: id, Kind: kindSnapshot, Seq: seq, State: state}
}
