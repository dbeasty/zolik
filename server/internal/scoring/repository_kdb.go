package scoring

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/db"
)

// kdbRepository stores each scoring session as one KDB document keyed by its
// ObjectID hex.
type kdbRepository struct {
	k *db.KDB
}

func NewKDBRepository(k *db.KDB) Repository {
	return &kdbRepository{k: k}
}

var _ Repository = (*kdbRepository)(nil)

func (r *kdbRepository) Insert(ctx context.Context, s ScoringSession) error {
	doc, err := db.MarshalDoc(s)
	if err != nil {
		return err
	}
	return r.k.Insert(db.NSScoring, s.ID.Hex(), doc)
}

func (r *kdbRepository) FindByID(ctx context.Context, id bson.ObjectID) (ScoringSession, error) {
	doc, err := r.k.Get(db.NSScoring, id.Hex())
	if err != nil {
		return ScoringSession{}, err
	}
	var s ScoringSession
	if err := db.UnmarshalDoc(doc, &s); err != nil {
		return ScoringSession{}, err
	}
	return s, nil
}

func (r *kdbRepository) Replace(ctx context.Context, s ScoringSession) error {
	doc, err := db.MarshalDoc(s)
	if err != nil {
		return err
	}
	return r.k.Update(db.NSScoring, func(tx *db.Tx) error {
		if _, err := tx.Get(s.ID.Hex()); err != nil {
			if db.IsNotFound(err) {
				// ReplaceOne without upsert writes nothing for a missing
				// document, and the handlers do not distinguish that.
				return nil
			}
			return err
		}
		return tx.Put(s.ID.Hex(), doc)
	})
}
