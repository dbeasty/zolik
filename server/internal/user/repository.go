package user

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"zolik/server/internal/db"
	"zolik/server/internal/models"
)

type Repository struct {
	users *mongo.Collection
	stats *mongo.Collection
}

func NewRepository(m *db.Mongo) *Repository {
	c := m.Collections()
	return &Repository{users: c.Users, stats: c.Statistics}
}

func (r *Repository) CreateUser(ctx context.Context, u models.User) (models.User, error) {
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

func (r *Repository) FindByUsername(ctx context.Context, username string) (models.User, error) {
	var u models.User
	if err := r.users.FindOne(ctx, bson.M{"username": username}).Decode(&u); err != nil {
		return models.User{}, err
	}
	return u, nil
}

func (r *Repository) FindByID(ctx context.Context, id bson.ObjectID) (models.User, error) {
	var u models.User
	if err := r.users.FindOne(ctx, bson.M{"_id": id}).Decode(&u); err != nil {
		return models.User{}, err
	}
	return u, nil
}

func (r *Repository) UpdateByID(ctx context.Context, id bson.ObjectID, update bson.M) error {
	_, err := r.users.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": update})
	return err
}

func (r *Repository) EnsureStats(ctx context.Context, userID bson.ObjectID) error {
	_, err := r.stats.UpdateOne(ctx,
		bson.M{"userId": userID},
		bson.M{"$setOnInsert": models.Statistics{UserID: userID}},
		nil,
	)
	return err
}

func (r *Repository) FindStatistics(ctx context.Context, userID bson.ObjectID) (models.Statistics, error) {
	var s models.Statistics
	err := r.stats.FindOne(ctx, bson.M{"userId": userID}).Decode(&s)
	return s, err
}


