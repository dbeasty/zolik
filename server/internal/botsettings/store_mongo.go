package botsettings

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"zolik/server/internal/db"
)

type mongoStore struct{ c *mongo.Collection }

// NewMongoStore builds the Mongo-backed store.
func NewMongoStore(m *db.Mongo) Store { return &mongoStore{c: m.Collections().Settings} }

func (s *mongoStore) HardModels(ctx context.Context) (map[string]HardModel, error) {
	var d Doc
	err := s.c.FindOne(ctx, bson.M{"_id": DocID}).Decode(&d)
	if db.IsNotFound(err) {
		return map[string]HardModel{}, nil
	}
	if err != nil {
		return nil, err
	}
	if d.HardModel == nil {
		d.HardModel = map[string]HardModel{}
	}
	return d.HardModel, nil
}

func (s *mongoStore) SetHardModel(ctx context.Context, game string, v HardModel) error {
	if err := checkGame(game); err != nil {
		return err
	}
	// $set on the one field, so two games changed at once cannot lose each
	// other's write.
	_, err := s.c.UpdateOne(ctx,
		bson.M{"_id": DocID},
		bson.M{"$set": bson.M{"hardModel." + game: v}},
		options.UpdateOne().SetUpsert(true))
	return err
}
