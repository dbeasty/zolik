// Package match is the game-agnostic runtime: it stores matches, routes
// actions to whichever module owns them, and fans out per-viewer state.
//
// It is the counterpart to internal/game, with one difference that is the
// whole point: nothing in here knows what a meld, a trick or a suit is. Every
// game-specific decision is a call into module.GameModule, and the state it
// persists is bytes it never opens.
package match

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"zolik/server/internal/db"
	"zolik/server/internal/models"
	"zolik/server/internal/rules"
)

// ErrVersionConflict is returned when someone else wrote first.
var ErrVersionConflict = fmt.Errorf("version conflict")

// Repository is the persistence behind matches. The Mongo-backed
// implementation is the only one today; the interface exists so a future
// backend can be swapped in behind it without touching any consumer.
type Repository interface {
	Insert(ctx context.Context, m models.Match) (models.Match, error)
	FindByID(ctx context.Context, id bson.ObjectID) (models.Match, error)
	FindByJoinCode(ctx context.Context, code string) (models.Match, error)
	// Resolve accepts either an object id or a join code, so a URL can carry
	// whichever the player has.
	Resolve(ctx context.Context, idOrCode string) (models.Match, error)
	// UpdateWithVersion replaces the match — the envelope, never its state or
	// moves — only if its version is unchanged, so load → change → store is
	// safe as long as a concurrent writer loses.
	UpdateWithVersion(ctx context.Context, id bson.ObjectID, expected int64, next models.Match) error
	// AppendMove stores one move, unless a move with its Seq is already
	// stored — then it is ErrVersionConflict, which is how two writers of the
	// same match find out one of them was behind. The only write an ordinary
	// move makes.
	AppendMove(ctx context.Context, id bson.ObjectID, move models.MatchAction) error
	// CommitSnapshot stores, together: moves (under AppendMove's rule), the
	// state after seq moves, and next under the version check. next.Snapshots
	// must already list seq. For the deal, round boundaries, the final move,
	// and moving an old match across, where every move must land with the
	// envelope.
	CommitSnapshot(ctx context.Context, id bson.ObjectID, expected int64, next models.Match,
		moves []models.MatchAction, seq int, state models.JSONDoc) error
	// Moves returns moves from+1 … to, oldest first, or every stored move
	// after from when to is negative. It stops at the first gap.
	Moves(ctx context.Context, id bson.ObjectID, from, to int) ([]models.MatchAction, error)
	// Snapshot returns the state stored after seq moves, or db.ErrNotFound.
	Snapshot(ctx context.Context, id bson.ObjectID, seq int) (models.JSONDoc, error)
	// FindAbandonable lists suspended matches whose AbandonAt has passed.
	//
	// It exists because AbandonAt did not, in any useful sense: it was written
	// by SuspendOnDisconnect and read by nothing, so a table whose player
	// closed the tab stayed suspended for ever — not finished, not abandoned,
	// not swept, and invisible to any count of how many games went unfinished.
	// This is the read that makes the field mean something.
	FindAbandonable(ctx context.Context, now time.Time, limit int) ([]models.Match, error)
	// FindStranded lists active matches whose last activity (see lastActivity)
	// is no later than idleBefore.
	//
	// The other way a table goes quiet. SuspendOnDisconnect only fires when a
	// socket the table is waiting on drops, so a match whose sockets never
	// existed (created and started over HTTP) or died with the process (a
	// restart) stays active for ever, with nothing left to suspend it.
	// Candidates only: the reaper re-checks presence and writes under a
	// version guard.
	FindStranded(ctx context.Context, idleBefore time.Time, limit int) ([]models.Match, error)
	// FindRetired lists matches that have reached a status they cannot leave
	// and have sat there past that status's retention window. Candidates only:
	// the caller re-checks each one and deletes under a version guard.
	FindRetired(ctx context.Context, now time.Time, w RetentionWindows, limit int) ([]models.Match, error)
	// DeleteIfUnchanged removes a match only if its version is still the one
	// the caller read.
	//
	// The guard is not ceremony. An abandoned table can be resumed, so a row
	// that qualified for deletion when it was scanned may be a live game
	// moments later; without the check, retention would occasionally delete a
	// match out from under the player who had just picked it back up.
	DeleteIfUnchanged(ctx context.Context, id bson.ObjectID, expected int64) error
	// FindForPlayer lists the matches a seat id is sitting at, newest activity
	// first, capped at f.Limit.
	FindForPlayer(ctx context.Context, playerID string, f PlayerMatchFilter) ([]models.Match, error)
}

// PlayerMatchFilter narrows FindForPlayer. A zero value lists every
// unfinished match, which is what "my games" opens on.
type PlayerMatchFilter struct {
	// Statuses to include. Empty means UnfinishedStatuses.
	Statuses []string
	// Limit caps how many rows come back. Zero means DefaultPlayerMatchLimit.
	Limit int
}

// UnfinishedStatuses is every status with something left to go back to —
// the default a "my games" list opens on. completed is excluded on purpose:
// it has its own permanent record in match_results, and its own history
// endpoint (GET /users/me/matches) — showing it here too would be two lists
// telling a player the same thing.
var UnfinishedStatuses = []string{"lobby", "active", "suspended", "abandoned"}

// FinishedStatuses is the complement, for the "finished" tab of that same list.
var FinishedStatuses = []string{"completed"}

// DefaultPlayerMatchLimit is what FindForPlayer uses when the caller asks for
// no particular limit.
const DefaultPlayerMatchLimit = 20

// MaxPlayerMatchLimit is the most a caller may ask FindForPlayer for. The KDB
// path pays for this in a full collection scan, so a caller cannot ask for a
// page large enough to be a cheap way of downloading everything.
const MaxPlayerMatchLimit = 50

func resolvePlayerMatchFilter(f PlayerMatchFilter) ([]string, int) {
	statuses := f.Statuses
	if len(statuses) == 0 {
		statuses = UnfinishedStatuses
	}
	limit := f.Limit
	if limit <= 0 {
		limit = DefaultPlayerMatchLimit
	}
	if limit > MaxPlayerMatchLimit {
		limit = MaxPlayerMatchLimit
	}
	return statuses, limit
}

type mongoRepository struct {
	coll *mongo.Collection
	log  *mongo.Collection
}

func NewRepository(m *db.Mongo) Repository {
	c := m.Collections()
	return &mongoRepository{coll: c.Matches, log: c.MatchLog}
}

var _ Repository = (*mongoRepository)(nil)

func (r *mongoRepository) Insert(ctx context.Context, m models.Match) (models.Match, error) {
	if m.CreatedAt.IsZero() {
		m.CreatedAt = time.Now().UTC()
	}
	m.Version = 1
	res, err := r.coll.InsertOne(ctx, m)
	if err != nil {
		return models.Match{}, err
	}
	if oid, ok := res.InsertedID.(bson.ObjectID); ok {
		m.ID = oid
	}
	return m, nil
}

func (r *mongoRepository) FindByID(ctx context.Context, id bson.ObjectID) (models.Match, error) {
	var m models.Match
	if err := r.coll.FindOne(ctx, bson.M{"_id": id}).Decode(&m); err != nil {
		return models.Match{}, err
	}
	return m, nil
}

func (r *mongoRepository) FindByJoinCode(ctx context.Context, code string) (models.Match, error) {
	var m models.Match
	if err := r.coll.FindOne(ctx, bson.M{"joinCode": code}).Decode(&m); err != nil {
		return models.Match{}, err
	}
	return m, nil
}

// Resolve accepts either an object id or a join code, so a URL can carry
// whichever the player has.
func (r *mongoRepository) Resolve(ctx context.Context, idOrCode string) (models.Match, error) {
	if oid, err := bson.ObjectIDFromHex(idOrCode); err == nil {
		return r.FindByID(ctx, oid)
	}
	return r.FindByJoinCode(ctx, idOrCode)
}

func (r *mongoRepository) UpdateWithVersion(ctx context.Context, id bson.ObjectID, expected int64, next models.Match) error {
	next.Version = expected + 1
	next.UpdatedAt = time.Now().UTC()
	res, err := r.coll.ReplaceOne(ctx,
		bson.M{"_id": id, "version": expected}, next,
		options.Replace().SetUpsert(false))
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return ErrVersionConflict
	}
	return nil
}

// FindRetired lists resolved matches past their retention window.
//
// One query per enabled class, or-ed together, so a disabled class contributes
// no clause at all rather than a clause that can never match. `active` and
// `suspended` are unrepresentable here by construction.
func (r *mongoRepository) FindRetired(ctx context.Context, now time.Time, w RetentionWindows, limit int) ([]models.Match, error) {
	var or []bson.M
	if w.Lobby > 0 {
		or = append(or, bson.M{"status": "lobby", "createdAt": bson.M{"$lt": now.Add(-w.Lobby)}})
	}
	if w.Completed > 0 {
		or = append(or, bson.M{"status": "completed", "endedAt": bson.M{"$lt": now.Add(-w.Completed)}})
	}
	if w.Abandoned > 0 {
		or = append(or, bson.M{
			"status":  string(rules.StatusAbandoned),
			"endedAt": bson.M{"$lt": now.Add(-w.Abandoned)},
		})
	}
	if len(or) == 0 {
		return nil, nil
	}
	cur, err := r.coll.Find(ctx, bson.M{"$or": or}, options.Find().SetLimit(int64(limit)))
	if err != nil {
		return nil, err
	}
	var out []models.Match
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// DeleteIfUnchanged removes the match and then its log. A delete cut short
// between the two leaves log records nothing points at; they are unreachable
// and cost space only, and no transaction is needed to keep that harmless.
func (r *mongoRepository) DeleteIfUnchanged(ctx context.Context, id bson.ObjectID, expected int64) error {
	res, err := r.coll.DeleteOne(ctx, bson.M{"_id": id, "version": expected})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return ErrVersionConflict
	}
	_, err = r.log.DeleteMany(ctx, bson.M{"m": id})
	return err
}

func (r *mongoRepository) AppendMove(ctx context.Context, id bson.ObjectID, move models.MatchAction) error {
	if _, err := r.log.InsertOne(ctx, toMoveRecord(id, move)); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return ErrVersionConflict
		}
		return err
	}
	return nil
}

// CommitSnapshot writes the move, the snapshot and then the envelope. Without
// a multi-document transaction a failure part-way can leave the move and the
// snapshot stored under an envelope that does not list them; both are
// harmless — the move is part of the log, and a snapshot the envelope does
// not list is never read — and the next load rebuilds from what the envelope
// does list.
func (r *mongoRepository) CommitSnapshot(ctx context.Context, id bson.ObjectID, expected int64, next models.Match,
	moves []models.MatchAction, seq int, state models.JSONDoc) error {
	for _, mv := range moves {
		if err := r.AppendMove(ctx, id, mv); err != nil {
			return err
		}
	}
	snap := snapshotRecord{Match: id, Kind: kindSnapshot, Seq: seq, State: state}
	if _, err := r.log.ReplaceOne(ctx, bson.M{"m": id, "k": kindSnapshot, "s": seq}, snap,
		options.Replace().SetUpsert(true)); err != nil {
		return err
	}
	return r.UpdateWithVersion(ctx, id, expected, next)
}

func (r *mongoRepository) Moves(ctx context.Context, id bson.ObjectID, from, to int) ([]models.MatchAction, error) {
	seq := bson.M{"$gt": from}
	if to >= 0 {
		seq["$lte"] = to
	}
	cur, err := r.log.Find(ctx, bson.M{"m": id, "k": kindMove, "s": seq},
		options.Find().SetSort(bson.D{{Key: "s", Value: 1}}))
	if err != nil {
		return nil, err
	}
	var recs []moveRecord
	if err := cur.All(ctx, &recs); err != nil {
		return nil, err
	}
	out := make([]models.MatchAction, 0, len(recs))
	for i, rec := range recs {
		if rec.Seq != from+i+1 {
			break
		}
		out = append(out, rec.action())
	}
	return out, nil
}

func (r *mongoRepository) Snapshot(ctx context.Context, id bson.ObjectID, seq int) (models.JSONDoc, error) {
	var snap snapshotRecord
	err := r.log.FindOne(ctx, bson.M{"m": id, "k": kindSnapshot, "s": seq}).Decode(&snap)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, db.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return snap.State, nil
}

// FindAbandonable lists suspended matches past their abandon deadline.
func (r *mongoRepository) FindAbandonable(ctx context.Context, now time.Time, limit int) ([]models.Match, error) {
	cur, err := r.coll.Find(ctx,
		bson.M{"status": "suspended", "abandonAt": bson.M{"$lte": now}},
		options.Find().SetLimit(int64(limit)).SetSort(bson.D{{Key: "abandonAt", Value: 1}}),
	)
	if err != nil {
		return nil, err
	}
	var out []models.Match
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// FindStranded lists active matches untouched since idleBefore. A row written
// before UpdatedAt existed has no updatedAt field and is judged by createdAt;
// the reaper re-checks with lastActivity either way.
func (r *mongoRepository) FindStranded(ctx context.Context, idleBefore time.Time, limit int) ([]models.Match, error) {
	cur, err := r.coll.Find(ctx,
		bson.M{"status": "active", "$or": bson.A{
			bson.M{"updatedAt": bson.M{"$lte": idleBefore}},
			bson.M{"updatedAt": bson.M{"$exists": false}, "createdAt": bson.M{"$lte": idleBefore}},
		}},
		options.Find().SetLimit(int64(limit)).SetSort(bson.D{{Key: "updatedAt", Value: 1}}),
	)
	if err != nil {
		return nil, err
	}
	var out []models.Match
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// FindForPlayer lists the matches a seat id sits at, newest activity first.
//
// Sorted by updatedAt then _id, so two matches
// updated in the same instant still come back in a stable order — an
// ObjectID embeds its creation time, so this is a legitimate tiebreak, not
// an arbitrary one.
func (r *mongoRepository) FindForPlayer(ctx context.Context, playerID string, f PlayerMatchFilter) ([]models.Match, error) {
	statuses, limit := resolvePlayerMatchFilter(f)
	cur, err := r.coll.Find(ctx,
		bson.M{"players.id": playerID, "status": bson.M{"$in": statuses}},
		options.Find().
			SetSort(bson.D{{Key: "updatedAt", Value: -1}, {Key: "_id", Value: -1}}).
			SetLimit(int64(limit)),
	)
	if err != nil {
		return nil, err
	}
	var out []models.Match
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}
