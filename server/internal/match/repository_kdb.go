package match

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/db"
	"zolik/server/internal/models"
)

// kdbRepository stores each match as one KDB document keyed by its ObjectID
// hex. The version check UpdateWithVersion promises is enforced inside the
// namespace's critical section — the engine itself has no conditional
// replace, and does not need one when every writer in the (single) process
// goes through the same lock.
type kdbRepository struct {
	k *db.KDB
}

func NewKDBRepository(k *db.KDB) Repository {
	return &kdbRepository{k: k}
}

var _ Repository = (*kdbRepository)(nil)

// errStopScan aborts a scan that found what it wanted.
var errStopScan = errors.New("stop scan")

func (r *kdbRepository) Insert(ctx context.Context, m models.Match) (models.Match, error) {
	if m.CreatedAt.IsZero() {
		m.CreatedAt = time.Now().UTC()
	}
	m.Version = 1
	if m.ID.IsZero() {
		m.ID = bson.NewObjectID()
	}
	doc, err := db.MarshalDoc(m)
	if err != nil {
		return models.Match{}, err
	}
	if err := r.k.Insert(db.NSMatches, m.ID.Hex(), doc); err != nil {
		return models.Match{}, err
	}
	return m, nil
}

func (r *kdbRepository) FindByID(ctx context.Context, id bson.ObjectID) (models.Match, error) {
	doc, err := r.k.Get(db.NSMatches, id.Hex())
	if err != nil {
		return models.Match{}, err
	}
	var m models.Match
	if err := db.UnmarshalDoc(doc, &m); err != nil {
		return models.Match{}, err
	}
	return m, nil
}

func (r *kdbRepository) FindByJoinCode(ctx context.Context, code string) (models.Match, error) {
	var out models.Match
	found := false
	err := r.k.Scan(db.NSMatches, func(doc []byte) error {
		var m models.Match
		if err := db.UnmarshalDoc(doc, &m); err != nil {
			return err
		}
		if m.JoinCode == code {
			out, found = m, true
			return errStopScan
		}
		return nil
	})
	if err != nil && !errors.Is(err, errStopScan) {
		return models.Match{}, err
	}
	if !found {
		return models.Match{}, db.ErrNotFound
	}
	return out, nil
}

// Resolve accepts either an object id or a join code, so a URL can carry
// whichever the player has.
func (r *kdbRepository) Resolve(ctx context.Context, idOrCode string) (models.Match, error) {
	if oid, err := bson.ObjectIDFromHex(idOrCode); err == nil {
		return r.FindByID(ctx, oid)
	}
	return r.FindByJoinCode(ctx, idOrCode)
}

func (r *kdbRepository) UpdateWithVersion(ctx context.Context, id bson.ObjectID, expected int64, next models.Match) error {
	next.Version = expected + 1
	next.ID = id
	doc, err := db.MarshalDoc(next)
	if err != nil {
		return err
	}
	return r.k.Update(db.NSMatches, func(tx *db.Tx) error {
		cur, err := tx.Get(id.Hex())
		if err != nil {
			if db.IsNotFound(err) {
				// Same shape Mongo's filtered replace gives: a missing
				// document and a stale version are both "someone else won".
				return ErrVersionConflict
			}
			return err
		}
		var probe struct {
			Version int64 `bson:"version"`
		}
		if err := db.UnmarshalDoc(cur, &probe); err != nil {
			return err
		}
		if probe.Version != expected {
			return ErrVersionConflict
		}
		return tx.Put(id.Hex(), doc)
	})
}
