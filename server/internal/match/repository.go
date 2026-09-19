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
	// UpdateWithVersion replaces the document only if its version is
	// unchanged. A whole match is one document, so load → apply → store is
	// safe without transactions as long as a concurrent writer loses.
	UpdateWithVersion(ctx context.Context, id bson.ObjectID, expected int64, next models.Match) error
	// FindAbandonable lists suspended matches whose AbandonAt has passed.
	//
	// It exists because AbandonAt did not, in any useful sense: it was written
	// by SuspendOnDisconnect and read by nothing, so a table whose player
	// closed the tab stayed suspended for ever — not finished, not abandoned,
	// not swept, and invisible to any count of how many games went unfinished.
	// This is the read that makes the field mean something.
	FindAbandonable(ctx context.Context, now time.Time, limit int) ([]models.Match, error)
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
	// first, capped at f.Limit. Neither State nor ActionLog is populated — a
	// list row needs neither, and together they are the bulk of the document.
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
}

func NewRepository(m *db.Mongo) Repository {
	return &mongoRepository{coll: m.Collections().Matches}
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

func (r *mongoRepository) DeleteIfUnchanged(ctx context.Context, id bson.ObjectID, expected int64) error {
	res, err := r.coll.DeleteOne(ctx, bson.M{"_id": id, "version": expected})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return ErrVersionConflict
	}
	return nil
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

// FindForPlayer lists the matches a seat id sits at, newest activity first.
//
// Projected without state/actionLog/checkpoints: those are the bulk of the
// document and a list row uses none of them. Checkpoints belong on that list
// for the same reason state does — each one *is* a stored board, so a list of
// twenty tables would otherwise carry a hundred of them. Sorted by updatedAt then _id, so two matches
// updated in the same instant still come back in a stable order — an
// ObjectID embeds its creation time, so this is a legitimate tiebreak, not
// an arbitrary one.
func (r *mongoRepository) FindForPlayer(ctx context.Context, playerID string, f PlayerMatchFilter) ([]models.Match, error) {
	statuses, limit := resolvePlayerMatchFilter(f)
	cur, err := r.coll.Find(ctx,
		bson.M{"players.id": playerID, "status": bson.M{"$in": statuses}},
		options.Find().
			SetSort(bson.D{{Key: "updatedAt", Value: -1}, {Key: "_id", Value: -1}}).
			SetLimit(int64(limit)).
			SetProjection(bson.M{"state": 0, "actionLog": 0, "checkpoints": 0}),
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
