package match

import (
	"context"
	"errors"
	"sort"
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

// FindRetired scans for resolved matches past their retention window.
//
// A scan rather than a query because the engine has no secondary indexes; the
// predicate is the same one the sweeper re-applies before deleting, and lives
// in retention.go so the two can never disagree about what "retired" means.
func (r *kdbRepository) FindRetired(ctx context.Context, now time.Time, w RetentionWindows, limit int) ([]models.Match, error) {
	if !w.Any() {
		return nil, nil
	}
	var out []models.Match
	err := r.k.Scan(db.NSMatches, func(raw []byte) error {
		var m models.Match
		if err := db.UnmarshalDoc(raw, &m); err != nil {
			return err
		}
		if !retired(m, now, w) {
			return nil
		}
		out = append(out, m)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// DeleteIfUnchanged removes a match inside the namespace's critical section,
// which is where every other conditional write in this repository is enforced —
// the engine has no conditional delete, and does not need one while kdb mode is
// single-process.
func (r *kdbRepository) DeleteIfUnchanged(ctx context.Context, id bson.ObjectID, expected int64) error {
	return r.k.Update(db.NSMatches, func(tx *db.Tx) error {
		cur, err := tx.Get(id.Hex())
		if err != nil {
			if db.IsNotFound(err) {
				// Already gone. Same shape Mongo's filtered delete gives:
				// somebody else won, and there is nothing left to do.
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
		_, err = tx.Delete(id.Hex())
		return err
	})
}

// FindAbandonable scans for suspended matches past their deadline.
//
// A scan, like every other cross-document query this backend answers: KDB has
// no secondary index, and the alternative — keeping a separate namespace of
// deadlines — would be a second thing to keep in step with the match document
// for a sweep that runs twice a minute over a collection holding live games
// only.
func (r *kdbRepository) FindAbandonable(ctx context.Context, now time.Time, limit int) ([]models.Match, error) {
	var out []models.Match
	err := r.k.Scan(db.NSMatches, func(raw []byte) error {
		var m models.Match
		if err := db.UnmarshalDoc(raw, &m); err != nil {
			return err
		}
		if m.Status != "suspended" || m.AbandonAt == nil || m.AbandonAt.After(now) {
			return nil
		}
		out = append(out, m)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].AbandonAt.Before(*out[j].AbandonAt) })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}
