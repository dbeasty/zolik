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
