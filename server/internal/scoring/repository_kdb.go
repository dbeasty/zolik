package scoring

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/db"
)

// kdbRepository stores each scoring session as one KDB document keyed by its
// ObjectID hex.
//
// A scorepad belonging to an account is written twice: into the scoring
// namespace, which is what this server reads, and into that account's own
// namespace, which is what their devices sync. A scorepad with no owner - the
// pen-and-paper pad anybody can open without signing in - has nowhere to be
// mirrored to and stays where it is.
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
	owner := ownerSubject(s)
	namespaces := append([]string{db.NSScoring}, db.UserNamespacesOf(owner)...)
	return r.k.UpdateMulti(namespaces, func(tx *db.MultiTx) error {
		if _, err := tx.Get(db.NSScoring, s.ID.Hex()); err == nil {
			return db.ErrDuplicateKey
		} else if !db.IsNotFound(err) {
			return err
		}
		tx.Put(db.NSScoring, s.ID.Hex(), doc)
		return db.MirrorUserDoc(tx, owner, scoringKey(s.ID.Hex()), db.KindScoring, doc)
	})
}

// ownerSubject is the subject key of a scorepad's owner, as internal/notify
// and internal/stats spell one, or empty for a pad nobody signed in to make.
func ownerSubject(s ScoringSession) string {
	if s.OwnerUser == "" {
		return ""
	}
	return "user:" + s.OwnerUser
}

func scoringKey(id string) string { return "scoring/" + id }

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
	owner := ownerSubject(s)
	namespaces := append([]string{db.NSScoring}, db.UserNamespacesOf(owner)...)
	return r.k.UpdateMulti(namespaces, func(tx *db.MultiTx) error {
		if _, err := tx.Get(db.NSScoring, s.ID.Hex()); err != nil {
			if db.IsNotFound(err) {
				// ReplaceOne without upsert writes nothing for a missing
				// document, and the handlers do not distinguish that.
				return nil
			}
			return err
		}
		tx.Put(db.NSScoring, s.ID.Hex(), doc)
		return db.MirrorUserDoc(tx, owner, scoringKey(s.ID.Hex()), db.KindScoring, doc)
	})
}
