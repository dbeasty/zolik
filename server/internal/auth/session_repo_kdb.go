package auth

import (
	"context"
	"time"

	"zolik/server/internal/db"
	"zolik/server/internal/models"
)

// kdbSessionRepository stores each refresh-token session as one KDB document
// keyed by the token itself — the token is the only lookup key the interface
// has, which makes every read a direct document get.
//
// Mongo expires sessions with a TTL index; here FindByToken refuses an
// expired session itself and the engine's sweeper reclaims the document. The
// read-side check is the load-bearing half: Mongo's TTL monitor also only
// sweeps periodically, so "present but past expiresAt" was always a state
// the system could observe — Mongo just happened to never be asked, because
// nothing looked up sessions any way but by token after checking expiry
// upstream. Here the check is explicit.
type kdbSessionRepository struct {
	k *db.KDB
}

func NewKDBSessionRepository(k *db.KDB) SessionRepository {
	return &kdbSessionRepository{k: k}
}

var _ SessionRepository = (*kdbSessionRepository)(nil)

func (r *kdbSessionRepository) CreateGuestSession(ctx context.Context, token string, guestName string, ttl time.Duration) error {
	now := time.Now().UTC()
	return r.CreateSession(ctx, models.Session{
		Token:     token,
		GuestName: guestName,
		UserID:    "",
		CreatedAt: now,
		ExpiresAt: now.Add(ttl),
	})
}

func (r *kdbSessionRepository) CreateSession(ctx context.Context, s models.Session) error {
	doc, err := db.MarshalDoc(s)
	if err != nil {
		return err
	}
	// Insert, not Put: the token carries a unique index in Mongo, and a
	// colliding token must fail loudly rather than adopt someone's session.
	return r.k.Insert(db.NSSessions, s.Token, doc)
}

func (r *kdbSessionRepository) FindByToken(ctx context.Context, token string) (models.Session, error) {
	doc, err := r.k.Get(db.NSSessions, token)
	if err != nil {
		return models.Session{}, err
	}
	var s models.Session
	if err := db.UnmarshalDoc(doc, &s); err != nil {
		return models.Session{}, err
	}
	if !s.ExpiresAt.IsZero() && s.ExpiresAt.Before(time.Now().UTC()) {
		return models.Session{}, db.ErrNotFound
	}
	return s, nil
}

func (r *kdbSessionRepository) SetGuestID(ctx context.Context, token, guestID string) error {
	return r.k.Update(db.NSSessions, func(tx *db.Tx) error {
		doc, err := tx.Get(token)
		if err != nil {
			if db.IsNotFound(err) {
				// UpdateOne on a missing document matches nothing and is not
				// an error; keep that shape.
				return nil
			}
			return err
		}
		var s models.Session
		if err := db.UnmarshalDoc(doc, &s); err != nil {
			return err
		}
		s.GuestID = guestID
		next, err := db.MarshalDoc(s)
		if err != nil {
			return err
		}
		return tx.Put(token, next)
	})
}

func (r *kdbSessionRepository) DeleteByToken(ctx context.Context, token string) error {
	_, err := r.k.Delete(db.NSSessions, token)
	return err
}
