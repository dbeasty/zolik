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
