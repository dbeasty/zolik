package user

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/db"
	"zolik/server/internal/models"
)

// kdbRepository stores each account as one KDB document keyed by its
// ObjectID hex. The unique username/email constraints Mongo enforces with
// indexes are enforced by db.KDBUserClash inside the namespace's critical
// section — the same guarantee, held one level up, and correct because the
// KDB deployment shape is single-process. The shared users-namespace logic
// lives in internal/db (see kdbusers.go) because auth's store works the same
// namespace and cannot import this package back.
type kdbRepository struct {
	k *db.KDB
}

func NewKDBRepository(k *db.KDB) Repository {
	return &kdbRepository{k: k}
}

var _ Repository = (*kdbRepository)(nil)

func (r *kdbRepository) CreateUser(ctx context.Context, u models.User) (models.User, error) {
	now := time.Now().UTC()
	if u.CreatedAt.IsZero() {
		u.CreatedAt = now
	}
	u.LastSeenAt = now
	if u.ID.IsZero() {
		u.ID = bson.NewObjectID()
	}
	doc, err := db.MarshalDoc(u)
	if err != nil {
		return models.User{}, err
	}
	err = r.k.Update(db.NSUsers, func(tx *db.Tx) error {
		if err := db.KDBUserClash(tx, u.ID, u.Username, u.Email); err != nil {
			return err
		}
		return tx.Insert(u.ID.Hex(), doc)
	})
	if err != nil {
		return models.User{}, err
	}
	return u, nil
}

func (r *kdbRepository) FindByUsername(ctx context.Context, username string) (models.User, error) {
	return db.KDBFindUserByUsername(r.k, username)
}

func (r *kdbRepository) FindByID(ctx context.Context, id bson.ObjectID) (models.User, error) {
	doc, err := r.k.Get(db.NSUsers, id.Hex())
	if err != nil {
		return models.User{}, err
	}
	var u models.User
	if err := db.UnmarshalDoc(doc, &u); err != nil {
		return models.User{}, err
	}
	return u, nil
}

func (r *kdbRepository) UpdateByID(ctx context.Context, id bson.ObjectID, update bson.M) error {
	return db.KDBUpdateUserFields(r.k, id, update)
}
