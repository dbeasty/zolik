package auth

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"zolik/server/internal/db"
	"zolik/server/internal/models"
)

// SessionRepository is the persistence behind refresh-token sessions. The
// Mongo-backed implementation is the only one today; the interface exists so
// a future backend can be swapped in behind it without touching any consumer.
type SessionRepository interface {
	CreateGuestSession(ctx context.Context, token string, guestName string, ttl time.Duration) error
	CreateSession(ctx context.Context, s models.Session) error
	FindByToken(ctx context.Context, token string) (models.Session, error)
	// SetGuestID attaches a durable guest identity to a session that predates
	// them, so a guest who has been playing since before this existed gains
	// one on their next refresh rather than staying unattributable forever.
	SetGuestID(ctx context.Context, token, guestID string) error
	DeleteByToken(ctx context.Context, token string) error
	// DeleteByUserID revokes every session an account holds, returning how
	// many went. It is what "sign out everywhere" is built on, and what stops
	// a deleted account's outstanding refresh tokens from going on minting
	// access tokens for an account that no longer exists.
	DeleteByUserID(ctx context.Context, userID string) (int64, error)
	// CountActiveSessions counts unexpired sessions, split by whether they
	// belong to an account or to a guest.
	CountActiveSessions(ctx context.Context, now time.Time) (users, guests int64, err error)
}

type mongoSessionRepository struct {
	coll *mongo.Collection
}

func NewSessionRepository(m *db.Mongo) SessionRepository {
	return &mongoSessionRepository{coll: m.Collections().Sessions}
}

var _ SessionRepository = (*mongoSessionRepository)(nil)

func (r *mongoSessionRepository) CreateGuestSession(ctx context.Context, token string, guestName string, ttl time.Duration) error {
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

func (r *mongoSessionRepository) CreateSession(ctx context.Context, s models.Session) error {
	_, err := r.coll.InsertOne(ctx, s)
	return err
}

func (r *mongoSessionRepository) FindByToken(ctx context.Context, token string) (models.Session, error) {
	var s models.Session
	err := r.coll.FindOne(ctx, bson.M{"token": token}).Decode(&s)
	return s, err
}

// SetGuestID attaches a durable guest identity to a session that predates
// them, so a guest who has been playing since before this existed gains one on
// their next refresh rather than staying unattributable forever.
func (r *mongoSessionRepository) SetGuestID(ctx context.Context, token, guestID string) error {
	_, err := r.coll.UpdateOne(ctx,
		bson.M{"token": token},
		bson.M{"$set": bson.M{"guestId": guestID}},
	)
	return err
}

func (r *mongoSessionRepository) DeleteByToken(ctx context.Context, token string) error {
	_, err := r.coll.DeleteOne(ctx, bson.M{"token": token})
	return err
}

func (r *mongoSessionRepository) DeleteByUserID(ctx context.Context, userID string) (int64, error) {
	if userID == "" {
		// A blank userId is what every guest session carries, so letting this
		// through would revoke every guest on the server at once.
		return 0, errors.New("userID is required")
	}
	res, err := r.coll.DeleteMany(ctx, bson.M{"userId": userID})
	if err != nil {
		return 0, err
	}
	return res.DeletedCount, nil
}

func (r *mongoSessionRepository) CountActiveSessions(ctx context.Context, now time.Time) (int64, int64, error) {
	unexpired := bson.M{"expiresAt": bson.M{"$gt": now}}
	users, err := r.coll.CountDocuments(ctx, bson.M{
		"$and": []bson.M{unexpired, {"userId": bson.M{"$nin": []any{"", nil}}}},
	})
	if err != nil {
		return 0, 0, err
	}
	guests, err := r.coll.CountDocuments(ctx, bson.M{
		"$and": []bson.M{unexpired, {"userId": bson.M{"$in": []any{"", nil}}}},
	})
	if err != nil {
		return 0, 0, err
	}
	return users, guests, nil
}
