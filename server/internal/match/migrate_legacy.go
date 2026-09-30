package match

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/db"
	"zolik/server/internal/models"
)

// Moving matches stored before moves and boards left the match document.
//
// Such a match carried its board in "state", every move in "actionLog", and
// its round boundaries in "checkpoints" — all rewritten on every move. The
// migration writes each logged move as a move record (with its round mark),
// the board as a snapshot at the last move, and the envelope without the old
// fields. Checkpoint boards are not carried over: they only shortened a
// replay's fold, which now starts from the deal for these matches.
//
// Each match moves in one write — its moves, its board and its new envelope
// together — so it is safe to stop at any point and to run more than once: a
// match is either still in the old layout, and migrated again, or wholly in
// the new one, and skipped. One write per match also keeps the store's own
// history to one commit per match rather than one per move.

type legacyAction struct {
	Seq      int             `bson:"seq"`
	PlayerID string          `bson:"playerId"`
	Action   json.RawMessage `bson:"action"`
	At       time.Time       `bson:"at"`
}

type legacyCheckpoint struct {
	Seq   int `bson:"seq"`
	Round int `bson:"round"`
}

// legacyParts are the fields an old match document carried.
type legacyParts struct {
	State       json.RawMessage    `bson:"state"`
	ActionLog   []legacyAction     `bson:"actionLog"`
	Checkpoints []legacyCheckpoint `bson:"checkpoints"`
}

func (p legacyParts) present() bool {
	return len(p.State) > 0 || len(p.ActionLog) > 0 || len(p.Checkpoints) > 0
}

// MigrationReport counts what a migration did.
type MigrationReport struct {
	Matches  int // matches moved to the new layout
	Moves    int // move records written
	Skipped  int // matches already in the new layout
	Verified int // matches read back and found identical
}

func (r MigrationReport) String() string {
	return fmt.Sprintf("%d matches migrated (%d moves), %d already migrated, %d verified",
		r.Matches, r.Moves, r.Skipped, r.Verified)
}

type legacyMatch struct {
	env   models.Match
	parts legacyParts
}

// migrateOne moves one match, then reads it back.
func migrateOne(ctx context.Context, repo Repository, lm legacyMatch) (int, error) {
	env, old := lm.env, lm.parts
	rounds := map[int]int{}
	for _, c := range old.Checkpoints {
		rounds[c.Seq] = c.Round
	}
	moves := make([]models.MatchAction, 0, len(old.ActionLog))
	for _, a := range old.ActionLog {
		moves = append(moves, models.MatchAction{Seq: a.Seq, PlayerID: a.PlayerID,
			Action: models.JSONDoc(a.Action), At: a.At, Rounds: rounds[a.Seq]})
	}

	next := env
	final := len(old.ActionLog)
	if len(old.State) == 0 {
		// Never dealt: nothing to snapshot, only the old fields to drop.
		next.Snapshots = nil
		if err := repo.UpdateWithVersion(ctx, env.ID, env.Version, next); err != nil {
			return 0, fmt.Errorf("match %s: %w", env.ID.Hex(), err)
		}
	} else {
		next.Snapshots = []int{final}
		if err := repo.CommitSnapshot(ctx, env.ID, env.Version, next, moves, final, models.JSONDoc(old.State)); err != nil {
			return 0, fmt.Errorf("match %s: %w", env.ID.Hex(), err)
		}
	}

	// Read back: every move, and the board.
	moves, err := repo.Moves(ctx, env.ID, 0, -1)
	if err != nil {
		return 0, err
	}
	if len(moves) != len(old.ActionLog) {
		return 0, fmt.Errorf("match %s: %d moves logged, %d stored", env.ID.Hex(), len(old.ActionLog), len(moves))
	}
	if len(old.State) > 0 {
		snap, err := repo.Snapshot(ctx, env.ID, final)
		if err != nil || string(snap) != string(old.State) {
			return 0, fmt.Errorf("match %s: the stored board differs from the old one (%v)", env.ID.Hex(), err)
		}
	}
	return len(moves), nil
}

// keepUpdatedAt puts a migrated match's UpdatedAt back to what it was: moving a
// match is not activity, and "my games" sorts and the stranded-match sweep
// judges by it.
type keepUpdatedAt func(ctx context.Context, id bson.ObjectID, at time.Time) error

func migrateAll(ctx context.Context, repo Repository, keep keepUpdatedAt, found []legacyMatch, skipped int, dry bool) (MigrationReport, error) {
	r := MigrationReport{Skipped: skipped}
	for _, lm := range found {
		if dry {
			r.Matches++
			r.Moves += len(lm.parts.ActionLog)
			continue
		}
		n, err := migrateOne(ctx, repo, lm)
		if err != nil {
			return r, err
		}
		if !lm.env.UpdatedAt.IsZero() {
			if err := keep(ctx, lm.env.ID, lm.env.UpdatedAt); err != nil {
				return r, fmt.Errorf("match %s: restoring updatedAt: %w", lm.env.ID.Hex(), err)
			}
		}
		r.Matches++
		r.Moves += n
		r.Verified++
	}
	return r, nil
}

// MigrateKDB moves every old-layout match in an embedded store. The server
// must be stopped: the store's directory lock refuses a second opener anyway.
func MigrateKDB(ctx context.Context, k *db.KDB, dry bool) (MigrationReport, error) {
	var found []legacyMatch
	skipped := 0
	err := k.Scan(db.NSMatches, func(raw []byte) error {
		var lm legacyMatch
		if err := db.UnmarshalDoc(raw, &lm.env); err != nil {
			return err
		}
		if err := db.UnmarshalDoc(raw, &lm.parts); err != nil {
			return fmt.Errorf("match %s: %w", lm.env.ID.Hex(), err)
		}
		if !lm.parts.present() {
			skipped++
			return nil
		}
		found = append(found, lm)
		return nil
	})
	if err != nil {
		return MigrationReport{}, err
	}
	keep := func(ctx context.Context, id bson.ObjectID, at time.Time) error {
		// Both copies of the envelope, in one transaction, exactly as every
		// other write to one does — see putEnvelope. Putting the time back in
		// the index alone would leave the match itself saying it had been
		// played on the day it was migrated, which is the version every read
		// answers from.
		return k.UpdateMulti(envelopeNamespaces(id), func(tx *db.MultiTx) error {
			raw, err := tx.Get(db.MatchNS(id.Hex()), keyMatchEnvelope)
			if db.IsNotFound(err) {
				raw, err = tx.Get(db.NSMatches, id.Hex())
			}
			if err != nil {
				return err
			}
			var m models.Match
			if err := db.UnmarshalDoc(raw, &m); err != nil {
				return err
			}
			m.UpdatedAt = at
			out, err := db.MarshalDoc(m)
			if err != nil {
				return err
			}
			putEnvelope(tx, id, out)
			return nil
		})
	}
	return migrateAll(ctx, NewKDBRepository(k), keep, found, skipped, dry)
}

// MigrateMongo moves every old-layout match in a Mongo database.
func MigrateMongo(ctx context.Context, m *db.Mongo, dry bool) (MigrationReport, error) {
	if err := m.EnsureIndexes(ctx); err != nil {
		return MigrationReport{}, err
	}
	coll := m.Collections().Matches
	total, err := coll.CountDocuments(ctx, bson.M{})
	if err != nil {
		return MigrationReport{}, err
	}
	cur, err := coll.Find(ctx, bson.M{"$or": bson.A{
		bson.M{"state": bson.M{"$exists": true}},
		bson.M{"actionLog": bson.M{"$exists": true}},
		bson.M{"checkpoints": bson.M{"$exists": true}},
	}})
	if err != nil {
		return MigrationReport{}, err
	}
	var found []legacyMatch
	for cur.Next(ctx) {
		var lm legacyMatch
		if err := cur.Decode(&lm.env); err != nil {
			return MigrationReport{}, err
		}
		if err := cur.Decode(&lm.parts); err != nil {
			return MigrationReport{}, fmt.Errorf("match %s: %w", lm.env.ID.Hex(), err)
		}
		found = append(found, lm)
	}
	if err := cur.Err(); err != nil {
		return MigrationReport{}, err
	}
	keep := func(ctx context.Context, id bson.ObjectID, at time.Time) error {
		_, err := coll.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": bson.M{"updatedAt": at}})
		return err
	}
	return migrateAll(ctx, NewRepository(m), keep, found, int(total)-len(found), dry)
}
