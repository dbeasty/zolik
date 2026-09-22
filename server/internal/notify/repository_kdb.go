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

// GetProfileByFriendCode reads the code's reservation to find its owner, and
// falls back to a scan for profiles written before codes were reserved (see
// db.KDBReserve); the fallback goes once cmd/migrate-namespaces has run
// everywhere.
func (r *kdbRepository) GetProfileByFriendCode(ctx context.Context, code string) (Profile, error) {
	switch owner, err := db.KDBReservationOwner(r.k, db.ReservationFriendCode, code); {
	case err == nil:
		return r.GetProfile(ctx, owner)
	case !db.IsNotFound(err):
		return Profile{}, err
	}
	return r.scanProfileByFriendCode(code)
}

func (r *kdbRepository) scanProfileByFriendCode(code string) (Profile, error) {
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

// PutProfile stores the profile and claims its friend code in the same
// transaction. The code is what another player types to find this person, so
// two profiles holding one code is the one thing this must not allow — and a
// scan for a free code cannot prevent it once profiles replicate, because
// each node scans only its own copy.
func (r *kdbRepository) PutProfile(_ context.Context, p Profile) error {
	doc, err := db.MarshalDoc(p)
	if err != nil {
		return err
	}
	if p.FriendCode == "" {
		return r.k.Put(db.NSNotifyProfiles, p.Key, doc)
	}
	return r.k.UpdateMulti([]string{db.NSNotifyProfiles, db.NSReservations}, func(tx *db.MultiTx) error {
		var before Profile
		switch cur, err := tx.Get(db.NSNotifyProfiles, p.Key); {
		case err == nil:
			if err := db.UnmarshalDoc(cur, &before); err != nil {
				return err
			}
		case !db.IsNotFound(err):
			return err
		}
		if before.FriendCode != p.FriendCode {
			if err := db.KDBReserve(tx, db.ReservationFriendCode, p.FriendCode, p.Key); err != nil {
				return err
			}
			if err := db.KDBRelease(tx, db.ReservationFriendCode, before.FriendCode, p.Key); err != nil {
				return err
			}
		}
		tx.Put(db.NSNotifyProfiles, p.Key, doc)
		return nil
	})
}

// DeleteProfile removes the profile and the claim on its friend code
// together: a code left reserved for a profile that no longer exists could
// never be taken by anyone again.
func (r *kdbRepository) DeleteProfile(ctx context.Context, key string) error {
	p, err := r.GetProfile(ctx, key)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	return r.k.UpdateMulti([]string{db.NSNotifyProfiles, db.NSReservations}, func(tx *db.MultiTx) error {
		if err := db.KDBRelease(tx, db.ReservationFriendCode, p.FriendCode, key); err != nil {
			return err
		}
		tx.Delete(db.NSNotifyProfiles, key)
		return nil
	})
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
