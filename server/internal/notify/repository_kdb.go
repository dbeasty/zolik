package notify

import (
	"context"
	"errors"

	"zolik/server/internal/db"
)

// kdbRepository keys every document by its natural id — the subject key for
// a profile, "owner>member" for an edge, the derived id for a device — so the
// reads that matter most are direct gets. The by-owner, by-member and
// by-friend-code reads scan, as every secondary lookup on this engine does;
// a circle is a few dozen rows per person.
type kdbRepository struct {
	k *db.KDB
}

func NewKDBRepository(k *db.KDB) Repository { return &kdbRepository{k: k} }

var _ Repository = (*kdbRepository)(nil)

var errStop = errors.New("stop scan")

func getDoc[T any](k *db.KDB, ns, key string) (T, error) {
	var out T
	raw, err := k.Get(ns, key)
	if err != nil {
		if db.IsNotFound(err) {
			return out, ErrNotFound
		}
		return out, err
	}
	if raw == nil {
		return out, ErrNotFound
	}
	return out, db.UnmarshalDoc(raw, &out)
}

func putDoc(k *db.KDB, ns, key string, v any) error {
	doc, err := db.MarshalDoc(v)
	if err != nil {
		return err
	}
	return k.Put(ns, key, doc)
}

func scanDocs[T any](k *db.KDB, ns string, keep func(T) bool) ([]T, error) {
	var out []T
	err := k.Scan(ns, func(raw []byte) error {
		var v T
		if err := db.UnmarshalDoc(raw, &v); err != nil {
			return err
		}
		if keep(v) {
			out = append(out, v)
		}
		return nil
	})
	return out, err
}

func (r *kdbRepository) GetProfile(_ context.Context, key string) (Profile, error) {
	return getDoc[Profile](r.k, db.NSNotifyProfiles, key)
}

func (r *kdbRepository) GetProfileByFriendCode(_ context.Context, code string) (Profile, error) {
	var out Profile
	found := false
	err := r.k.Scan(db.NSNotifyProfiles, func(raw []byte) error {
		var p Profile
		if err := db.UnmarshalDoc(raw, &p); err != nil {
			return err
		}
		if p.FriendCode == code {
			out, found = p, true
			return errStop
		}
		return nil
	})
	if err != nil && !errors.Is(err, errStop) {
		return Profile{}, err
	}
	if !found {
		return Profile{}, ErrNotFound
	}
	return out, nil
}

func (r *kdbRepository) PutProfile(_ context.Context, p Profile) error {
	return putDoc(r.k, db.NSNotifyProfiles, p.Key, p)
}

func (r *kdbRepository) GetEdge(_ context.Context, owner, member string) (Edge, error) {
	return getDoc[Edge](r.k, db.NSNotifyCircle, edgeID(owner, member))
}

func (r *kdbRepository) PutEdge(_ context.Context, e Edge) error {
	e.ID = edgeID(e.OwnerKey, e.MemberKey)
	return putDoc(r.k, db.NSNotifyCircle, e.ID, e)
}

func (r *kdbRepository) DeleteEdge(_ context.Context, owner, member string) error {
	_, err := r.k.Delete(db.NSNotifyCircle, edgeID(owner, member))
	return err
}

func (r *kdbRepository) EdgesByOwner(_ context.Context, owner string) ([]Edge, error) {
	return scanDocs(r.k, db.NSNotifyCircle, func(e Edge) bool { return e.OwnerKey == owner })
}

func (r *kdbRepository) EdgesByMember(_ context.Context, member string) ([]Edge, error) {
	return scanDocs(r.k, db.NSNotifyCircle, func(e Edge) bool { return e.MemberKey == member })
}

func (r *kdbRepository) PutDevice(_ context.Context, d Device) error {
	return putDoc(r.k, db.NSNotifyDevices, d.ID, d)
}

func (r *kdbRepository) DeleteDevice(_ context.Context, id string) error {
	_, err := r.k.Delete(db.NSNotifyDevices, id)
	return err
}

func (r *kdbRepository) DevicesFor(_ context.Context, key string) ([]Device, error) {
	return scanDocs(r.k, db.NSNotifyDevices, func(d Device) bool { return d.SubjectKey == key })
}

func (r *kdbRepository) Rekey(ctx context.Context, from, to string) error {
	return rekey(ctx, r, from, to)
}
