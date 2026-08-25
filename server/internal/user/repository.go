package user

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"zolik/server/internal/db"
	"zolik/server/internal/models"
)

// Repository is the persistence behind accounts. The Mongo-backed
// implementation is the only one today; the interface exists so a future
// backend can be swapped in behind it without touching any consumer.
type Repository interface {
	CreateUser(ctx context.Context, u models.User) (models.User, error)
	FindByUsername(ctx context.Context, username string) (models.User, error)
	FindByID(ctx context.Context, id bson.ObjectID) (models.User, error)
	UpdateByID(ctx context.Context, id bson.ObjectID, update bson.M) error
}

type mongoRepository struct {
	users *mongo.Collection
}

func NewRepository(m *db.Mongo) Repository {
	c := m.Collections()
	return &mongoRepository{users: c.Users}
}

var _ Repository = (*mongoRepository)(nil)

func (r *mongoRepository) CreateUser(ctx context.Context, u models.User) (models.User, error) {
	now := time.Now().UTC()
	if u.CreatedAt.IsZero() {
		u.CreatedAt = now
	}
	u.LastSeenAt = now
	res, err := r.users.InsertOne(ctx, u)
	if err != nil {
		return models.User{}, err
	}
	u.ID = res.InsertedID.(bson.ObjectID)
	return u, nil
}

func (r *mongoRepository) FindByUsername(ctx context.Context, username string) (models.User, error) {
	var u models.User
	if err := r.users.FindOne(ctx, bson.M{"username": username}).Decode(&u); err != nil {
		return models.User{}, err
	}
	return u, nil
}

func (r *mongoRepository) FindByID(ctx context.Context, id bson.ObjectID) (models.User, error) {
	var u models.User
	if err := r.users.FindOne(ctx, bson.M{"_id": id}).Decode(&u); err != nil {
		return models.User{}, err
	}
	return u, nil
}

func (r *mongoRepository) UpdateByID(ctx context.Context, id bson.ObjectID, update bson.M) error {
	_, err := r.users.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": update})
	return err
}
