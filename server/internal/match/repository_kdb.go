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
	next.UpdatedAt = time.Now().UTC()
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

// FindForPlayer scans for matches a seat id sits at.
//
// A scan, like every other cross-document read on this backend: KDB has no
// secondary index, so "every match with this player" costs a full pass over
// the namespace regardless of how the caller phrases it. State and ActionLog
// are dropped after decoding rather than never read — the engine has no
// server-side projection to skip them with — so this saves nothing over the
// wire that a real server doesn't have, but it does keep the returned rows
// the same shape Mongo hands back, which is what callers rely on.
func (r *kdbRepository) FindForPlayer(ctx context.Context, playerID string, f PlayerMatchFilter) ([]models.Match, error) {
	statuses, limit := resolvePlayerMatchFilter(f)
	want := make(map[string]bool, len(statuses))
	for _, s := range statuses {
		want[s] = true
	}
	var out []models.Match
	err := r.k.Scan(db.NSMatches, func(raw []byte) error {
		var m models.Match
		if err := db.UnmarshalDoc(raw, &m); err != nil {
			return err
		}
		if !want[m.Status] {
			return nil
		}
		seated := false
		for _, p := range m.Players {
			if p.ID == playerID {
				seated = true
				break
			}
		}
		if !seated {
			return nil
		}
		m.State = nil
		m.ActionLog = nil
		// Each checkpoint is a stored board; a list row needs none of them.
		m.Checkpoints = nil
		out = append(out, m)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool {
		ui, uj := out[i].UpdatedAt, out[j].UpdatedAt
		if !ui.Equal(uj) {
			return ui.After(uj)
		}
		return out[i].ID.Hex() > out[j].ID.Hex()
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// BoardAfter returns the match's board as it stood after a given number of
// accepted actions, read out of the store's own history.
//
// This is the one thing a fold cannot offer: the board as it *was*, rather
// than the board a module would produce from the same log today. It does not
// depend on Apply still being pure, it does not care that a module's rules
// have moved since, and it cannot be defeated by a state written outside the
// log. Where it answers, a replay frame is evidence rather than reconstruction.
//
// KDB only, and deliberately so — see the BoardSource capability in replay.go
// for how the runtime asks without knowing which engine it has.
//
// The search is over document *versions*, not actions, because they are not
// the same thing: suspending, resuming, seating and joining all write the
// match without touching the log. So the versions are binary-searched on the
// length of the log each one carries, which is monotonic by construction —
// the log is append-only and nothing ever shortens it.
func (r *kdbRepository) BoardAfter(ctx context.Context, id bson.ObjectID, actions int) (models.Match, error) {
	commits, err := r.k.DocumentVersions(db.NSMatches, id.Hex())
	if err != nil {
		return models.Match{}, err
	}
	if len(commits) == 0 {
		return models.Match{}, db.ErrNotFound
	}

	at := func(i int) (models.Match, error) {
		doc, err := r.k.GetAt(db.NSMatches, id.Hex(), commits[i])
		if err != nil {
			return models.Match{}, err
		}
		var m models.Match
		return m, db.UnmarshalDoc(doc, &m)
	}

	// The newest version whose log is no longer than asked for. Everything
	// after it happened later than the frame being looked at.
	lo, hi, best := 0, len(commits)-1, -1
	for lo <= hi {
		mid := (lo + hi) / 2
		m, err := at(mid)
		if err != nil {
			return models.Match{}, err
		}
		if len(m.ActionLog) <= actions {
			best = mid
			lo = mid + 1
			continue
		}
		hi = mid - 1
	}
	if best < 0 {
		return models.Match{}, db.ErrNotFound
	}

	m, err := at(best)
	if err != nil {
		return models.Match{}, err
	}
	if len(m.State) == 0 {
		// A version from before the match was dealt. There is a board later,
		// but not one this far back.
		return models.Match{}, db.ErrNotFound
	}
	return m, nil
}
