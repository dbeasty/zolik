package scoring

import (
	"context"
	"errors"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"zolik/server/internal/db"
)

// Repository is the persistence behind pen-and-paper scoring sessions. The
// handlers used to reach straight into the Mongo collection; the interface
// exists so the KDB backend can stand behind the same routes.
type Repository interface {
	Insert(ctx context.Context, s ScoringSession) error
	// FindByID returns db.ErrNotFound (wrapped or not) when the session does
	// not exist; handlers only ever turn that into a 404.
	FindByID(ctx context.Context, id bson.ObjectID) (ScoringSession, error)
	// Replace overwrites an existing session document wholesale. Replacing a
	// session that does not exist is not an error the handlers care to
	// distinguish; it simply writes nothing.
	Replace(ctx context.Context, s ScoringSession) error
}

type mongoRepository struct {
	coll *mongo.Collection
}

func NewRepository(m *db.Mongo) Repository {
	return &mongoRepository{coll: m.Collections().Scoring}
}

var _ Repository = (*mongoRepository)(nil)

func (r *mongoRepository) Insert(ctx context.Context, s ScoringSession) error {
	_, err := r.coll.InsertOne(ctx, s)
	return err
}

func (r *mongoRepository) FindByID(ctx context.Context, id bson.ObjectID) (ScoringSession, error) {
	var s ScoringSession
	err := r.coll.FindOne(ctx, bson.M{"_id": id}).Decode(&s)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return ScoringSession{}, db.ErrNotFound
	}
	return s, err
}

func (r *mongoRepository) Replace(ctx context.Context, s ScoringSession) error {
	_, err := r.coll.ReplaceOne(ctx, bson.M{"_id": s.ID}, s, options.Replace().SetUpsert(false))
	return err
}
