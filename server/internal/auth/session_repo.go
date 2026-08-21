package auth

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"zolik/server/internal/db"
	"zolik/server/internal/models"
)

type SessionRepository struct {
	coll *mongo.Collection
}

func NewSessionRepository(m *db.Mongo) *SessionRepository {
	return &SessionRepository{coll: m.Collections().Sessions}
}

func (r *SessionRepository) CreateGuestSession(ctx context.Context, token string, guestName string, ttl time.Duration) error {
	now := time.Now().UTC()
	s := models.Session{
		Token:     token,
		GuestName: guestName,
		UserID:    "",
		CreatedAt: now,
		ExpiresAt: now.Add(ttl),
	}
	_, err := r.coll.InsertOne(ctx, s)
	return err
}

func (r *SessionRepository) CreateSession(ctx context.Context, s models.Session) error {
	_, err := r.coll.InsertOne(ctx, s)
	return err
}

func (r *SessionRepository) FindByToken(ctx context.Context, token string) (models.Session, error) {
	var s models.Session
	err := r.coll.FindOne(ctx, bson.M{"token": token}).Decode(&s)
	return s, err
}

func (r *SessionRepository) DeleteByToken(ctx context.Context, token string) error {
	_, err := r.coll.DeleteOne(ctx, bson.M{"token": token})
	return err
}
