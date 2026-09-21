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
	return r.k.UpdateMulti([]string{db.NSMatches, db.NSMatchLog}, func(tx *db.MultiTx) error {
		cur, err := envelopeLocked(tx, id, expected)
		if err != nil {
			return err
		}
		// Every move, found by counting on from the newest snapshot, every
		// snapshot, and the match — one transaction, so a match is never left
		// half deleted.
		from, _ := latestSnapshot(cur)
		head := from
		for {
			if _, err := tx.Get(db.NSMatchLog, logKey(id, kindMove, head+1)); err != nil {
				if db.IsNotFound(err) {
					break
				}
				return err
			}
			head++
		}
		for seq := 1; seq <= head; seq++ {
			tx.Delete(db.NSMatchLog, logKey(id, kindMove, seq))
		}
		for _, seq := range cur.Snapshots {
			tx.Delete(db.NSMatchLog, logKey(id, kindSnapshot, seq))
		}
		tx.Delete(db.NSMatches, id.Hex())
		return nil
	})
}

// envelopeLocked reads the stored match inside a transaction and checks it is
// still at version expected. A missing match is a conflict too: the same
// "someone else won" Mongo's filtered writes answer.
func envelopeLocked(tx *db.MultiTx, id bson.ObjectID, expected int64) (models.Match, error) {
	raw, err := tx.Get(db.NSMatches, id.Hex())
	if err != nil {
		if db.IsNotFound(err) {
			return models.Match{}, ErrVersionConflict
		}
		return models.Match{}, err
	}
	var cur models.Match
	if err := db.UnmarshalDoc(raw, &cur); err != nil {
		return models.Match{}, err
	}
	if cur.Version != expected {
		return models.Match{}, ErrVersionConflict
	}
	return cur, nil
}

// AppendMove is one insert: the key names the seq, so a second move N finds
// the first one there and is refused.
func (r *kdbRepository) AppendMove(ctx context.Context, id bson.ObjectID, move models.MatchAction) error {
	doc, err := db.MarshalDoc(toMoveRecord(id, move))
	if err != nil {
		return err
	}
	if err := r.k.Insert(db.NSMatchLog, logKey(id, kindMove, move.Seq), doc); err != nil {
		if errors.Is(err, db.ErrDuplicateKey) {
			return ErrVersionConflict
		}
		return err
	}
	return nil
}

// CommitSnapshot is one transaction across the match and its log: the move,
// the snapshot and the envelope that lists it land together or not at all.
func (r *kdbRepository) CommitSnapshot(ctx context.Context, id bson.ObjectID, expected int64, next models.Match,
	moves []models.MatchAction, seq int, state models.JSONDoc) error {
	return r.k.UpdateMulti([]string{db.NSMatches, db.NSMatchLog}, func(tx *db.MultiTx) error {
		if _, err := envelopeLocked(tx, id, expected); err != nil {
			return err
		}
		for _, mv := range moves {
			key := logKey(id, kindMove, mv.Seq)
			if _, err := tx.Get(db.NSMatchLog, key); err == nil {
				return ErrVersionConflict
			} else if !db.IsNotFound(err) {
				return err
			}
			doc, err := db.MarshalDoc(toMoveRecord(id, mv))
			if err != nil {
				return err
			}
			tx.Put(db.NSMatchLog, key, doc)
		}
		doc, err := db.MarshalDoc(snapshotRecord{Match: id, Kind: kindSnapshot, Seq: seq, State: state})
		if err != nil {
			return err
		}
		tx.Put(db.NSMatchLog, logKey(id, kindSnapshot, seq), doc)

		next.ID, next.Version, next.UpdatedAt = id, expected+1, time.Now().UTC()
		env, err := db.MarshalDoc(next)
		if err != nil {
			return err
		}
		tx.Put(db.NSMatches, id.Hex(), env)
		return nil
	})
}

// Moves reads by key, counting up from from+1: the store has no range scan,
// and does not need one when the keys are the seqs.
func (r *kdbRepository) Moves(ctx context.Context, id bson.ObjectID, from, to int) ([]models.MatchAction, error) {
	var out []models.MatchAction
	for seq := from + 1; to < 0 || seq <= to; seq++ {
		raw, err := r.k.Get(db.NSMatchLog, logKey(id, kindMove, seq))
		if db.IsNotFound(err) {
			break
		}
		if err != nil {
			return nil, err
		}
		var rec moveRecord
		if err := db.UnmarshalDoc(raw, &rec); err != nil {
			return nil, err
		}
		out = append(out, rec.action())
	}
	return out, nil
}

func (r *kdbRepository) Snapshot(ctx context.Context, id bson.ObjectID, seq int) (models.JSONDoc, error) {
	raw, err := r.k.Get(db.NSMatchLog, logKey(id, kindSnapshot, seq))
	if err != nil {
		return nil, err
	}
	var snap snapshotRecord
	if err := db.UnmarshalDoc(raw, &snap); err != nil {
		return nil, err
	}
	return snap.State, nil
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

// FindStranded scans for active matches untouched since idleBefore.
func (r *kdbRepository) FindStranded(ctx context.Context, idleBefore time.Time, limit int) ([]models.Match, error) {
	var out []models.Match
	err := r.k.Scan(db.NSMatches, func(raw []byte) error {
		var m models.Match
		if err := db.UnmarshalDoc(raw, &m); err != nil {
			return err
		}
		if m.Status != "active" || lastActivity(m).After(idleBefore) {
			return nil
		}
		out = append(out, m)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return lastActivity(out[i]).Before(lastActivity(out[j])) })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// FindForPlayer scans for matches a seat id sits at.
//
// A scan, like every other cross-document read on this backend: KDB has no
// secondary index, so "every match with this player" costs a full pass over
// the namespace regardless of how the caller phrases it — a cheap pass, now
// that a match document holds the envelope and not the board or the moves.
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
