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
	// Replace overwrites an existing session document wholesale, provided
	// the stored copy is still at version expect; otherwise it writes
	// nothing and returns ErrVersionConflict. Replacing a session that does
	// not exist is not an error the handlers care to distinguish; it simply
	// writes nothing.
	Replace(ctx context.Context, s ScoringSession, expect int) error
	// DeleteForOwner removes every scorepad an account made, for an account
	// that is deleted. A pad nobody signed in to make has no owner and is
	// never touched.
	DeleteForOwner(ctx context.Context, userID string) error
}

// ErrVersionConflict is a write against a scorepad somebody has changed since
// it was read.
var ErrVersionConflict = errors.New("scorepad changed since it was read")

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

func (r *mongoRepository) Replace(ctx context.Context, s ScoringSession, expect int) error {
	filter := bson.M{"_id": s.ID, "version": expect}
	if expect == 0 {
		// A pad from before versions has no field at all.
		filter = bson.M{"_id": s.ID, "$or": bson.A{bson.M{"version": 0}, bson.M{"version": bson.M{"$exists": false}}}}
	}
	res, err := r.coll.ReplaceOne(ctx, filter, s, options.Replace().SetUpsert(false))
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		// Gone, or moved on. Only the second is worth telling apart.
		if n, err := r.coll.CountDocuments(ctx, bson.M{"_id": s.ID}); err == nil && n > 0 {
			return ErrVersionConflict
		}
	}
	return nil
}

func (r *mongoRepository) DeleteForOwner(ctx context.Context, userID string) error {
	if userID == "" {
		return nil
	}
	_, err := r.coll.DeleteMany(ctx, bson.M{"ownerUser": userID})
	return err
}
