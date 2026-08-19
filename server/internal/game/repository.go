package game

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

type Repository struct {
	coll *mongo.Collection
}

func NewRepository(m *db.Mongo) *Repository {
	return &Repository{coll: m.Collections().Games}
}

func (r *Repository) Insert(ctx context.Context, g models.Game) (models.Game, error) {
	if g.CreatedAt.IsZero() {
		g.CreatedAt = time.Now().UTC()
	}
	g.Version = 1
	res, err := r.coll.InsertOne(ctx, g)
	if err != nil {
		return models.Game{}, err
	}
	if oid, ok := res.InsertedID.(bson.ObjectID); ok {
		g.ID = oid
	}
	return g, nil
}

func (r *Repository) FindByID(ctx context.Context, id any) (models.Game, error) {
	var g models.Game
	if err := r.coll.FindOne(ctx, bson.M{"_id": id}).Decode(&g); err != nil {
		return models.Game{}, err
	}
	return g, nil
}

func (r *Repository) FindByJoinCode(ctx context.Context, joinCode string) (models.Game, error) {
	var g models.Game
	if err := r.coll.FindOne(ctx, bson.M{"joinCode": joinCode}).Decode(&g); err != nil {
		return models.Game{}, err
	}
	return g, nil
}

func (r *Repository) ParseGameIDOrJoin(ctx context.Context, idOrJoin string) (models.Game, string, error) {
	// Returns: game, gameIDHex for registry/broadcast usage.
	oid, err := bson.ObjectIDFromHex(idOrJoin)
	if err == nil {
		g, err := r.FindByID(ctx, oid)
		if err != nil {
			return models.Game{}, "", err
		}
		return g, oid.Hex(), nil
	}
	g, err := r.FindByJoinCode(ctx, idOrJoin)
	if err != nil {
		return models.Game{}, "", err
	}
	return g, g.ID.Hex(), nil
}

// UpdateWithVersion atomically replaces the game document if the stored version matches.
func (r *Repository) UpdateWithVersion(ctx context.Context, id any, expectedVersion int64, next models.Game) error {
	next.Version = expectedVersion + 1
	filter := bson.M{"_id": id, "version": expectedVersion}
	res, err := r.coll.ReplaceOne(ctx, filter, next, options.Replace().SetUpsert(false))
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return fmt.Errorf("version conflict")
	}
	return nil
}
