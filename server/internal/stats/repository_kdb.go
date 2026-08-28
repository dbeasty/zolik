package stats

import (
	"context"
	"errors"
	"sort"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/db"
)

// kdbRepository stores one KDB document per match record (keyed by record id)
// and one per lifetime aggregate (keyed by the subject key itself, which is
// what makes UpsertPlayerStats and FindPlayerStats direct document
// operations, exactly as unique-indexed lookups are in Mongo).
//
// Subject-history queries scan the match_results namespace and filter in Go.
// That is not a shortcut taken behind the interface's back: Mongo's own
// Leaderboard already reads-and-sorts in the application for the same reason
// — at the scale a card game reaches, the simple plan is the right one, and
// the perf benchmarks in internal/dbperf exist to say when that stops being
// true.
type kdbRepository struct {
	k *db.KDB
}

func NewKDBRepository(k *db.KDB) Repository {
	return &kdbRepository{k: k}
}

var _ Repository = (*kdbRepository)(nil)

var errStopScan = errors.New("stop scan")

func (r *kdbRepository) InsertMatch(ctx context.Context, m MatchResult) (MatchResult, error) {
	if m.ID.IsZero() {
		m.ID = bson.NewObjectID()
	}
	doc, err := db.MarshalDoc(m)
	if err != nil {
		return MatchResult{}, err
	}
	err = r.k.Update(db.NSMatchResults, func(tx *db.Tx) error {
		// The unique matchId index is what makes recording idempotent under
		// Mongo; this scan inside the critical section is its stand-in.
		clash := false
		err := tx.Scan(func(doc []byte) error {
			var probe struct {
				MatchID bson.ObjectID `bson:"matchId"`
			}
			if err := db.UnmarshalDoc(doc, &probe); err != nil {
				return err
			}
			if probe.MatchID == m.MatchID {
				clash = true
				return errStopScan
			}
			return nil
		})
		if err != nil && !errors.Is(err, errStopScan) {
			return err
		}
		if clash {
			return ErrAlreadyRecorded
		}
		return tx.Insert(m.ID.Hex(), doc)
	})
	if err != nil {
		return MatchResult{}, err
	}
	return m, nil
}

func (r *kdbRepository) FindMatchByMatchID(ctx context.Context, matchID bson.ObjectID) (MatchResult, error) {
	var out MatchResult
	found := false
	err := r.k.Scan(db.NSMatchResults, func(doc []byte) error {
		var m MatchResult
		if err := db.UnmarshalDoc(doc, &m); err != nil {
			return err
		}
		if m.MatchID == matchID {
			out, found = m, true
			return errStopScan
		}
		return nil
	})
	if err != nil && !errors.Is(err, errStopScan) {
		return MatchResult{}, err
	}
	if !found {
		return MatchResult{}, ErrNotFound
	}
	return out, nil
}

// matchesForSubject loads every record naming the subject. Sorting happens
// at the call sites, which need opposite orders.
func (r *kdbRepository) matchesForSubject(key string) ([]MatchResult, error) {
	var out []MatchResult
	err := r.k.Scan(db.NSMatchResults, func(doc []byte) error {
		var m MatchResult
		if err := db.UnmarshalDoc(doc, &m); err != nil {
			return err
		}
		for _, k := range m.SubjectKeys {
			if k == key {
				out = append(out, m)
				break
			}
		}
		return nil
	})
	return out, err
}

func (r *kdbRepository) ListMatchesForSubject(ctx context.Context, key string, before time.Time, limit int) ([]MatchResult, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	rows, err := r.matchesForSubject(key)
	if err != nil {
		return nil, err
	}
	filtered := rows[:0]
	for _, m := range rows {
		if before.IsZero() || m.CompletedAt.Before(before) {
			filtered = append(filtered, m)
		}
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		if !filtered[i].CompletedAt.Equal(filtered[j].CompletedAt) {
			return filtered[i].CompletedAt.After(filtered[j].CompletedAt)
		}
		return filtered[i].ID.Hex() > filtered[j].ID.Hex()
	})
	if len(filtered) > limit {
		filtered = filtered[:limit]
	}
	out := make([]MatchResult, len(filtered))
	copy(out, filtered)
	return out, nil
}

func (r *kdbRepository) EachMatchForSubject(ctx context.Context, key string, fn func(MatchResult) error) error {
	if key == "" {
		return nil
	}
	rows, err := r.matchesForSubject(key)
	if err != nil {
		return err
	}
	// Oldest first: the aggregates this feeds are order-dependent.
	sort.SliceStable(rows, func(i, j int) bool {
		if !rows[i].CompletedAt.Equal(rows[j].CompletedAt) {
			return rows[i].CompletedAt.Before(rows[j].CompletedAt)
		}
		return rows[i].ID.Hex() < rows[j].ID.Hex()
	})
	for _, m := range rows {
		if err := fn(m); err != nil {
			return err
		}
	}
	return nil
}

func (r *kdbRepository) CountMatchesForSubject(ctx context.Context, key string) (int, error) {
	if key == "" {
		return 0, nil
	}
	rows, err := r.matchesForSubject(key)
	if err != nil {
		return 0, err
	}
	return len(rows), nil
}

func (r *kdbRepository) ReplaceMatchAttribution(ctx context.Context, m MatchResult) error {
	return r.k.Update(db.NSMatchResults, func(tx *db.Tx) error {
		doc, err := tx.Get(m.ID.Hex())
		if err != nil {
			if db.IsNotFound(err) {
				// UpdateOne on a missing document matches nothing; keep that.
				return nil
			}
			return err
		}
		var cur MatchResult
		if err := db.UnmarshalDoc(doc, &cur); err != nil {
			return err
		}
		// Only the two attribution fields move — the record stays otherwise
		// immutable, same promise as the Mongo $set.
		cur.Participants = m.Participants
		cur.SubjectKeys = m.SubjectKeys
		next, err := db.MarshalDoc(cur)
		if err != nil {
			return err
		}
		return tx.Put(m.ID.Hex(), next)
	})
}

func (r *kdbRepository) FindPlayerStats(ctx context.Context, key string) (PlayerStats, error) {
	doc, err := r.k.Get(db.NSPlayerStats, key)
	if err != nil {
		if db.IsNotFound(err) {
			return PlayerStats{}, ErrNotFound
		}
		return PlayerStats{}, err
	}
	var ps PlayerStats
	if err := db.UnmarshalDoc(doc, &ps); err != nil {
		return PlayerStats{}, err
	}
	return ps, nil
}

func (r *kdbRepository) FindManyPlayerStats(ctx context.Context, keys []string) (map[string]PlayerStats, error) {
	out := map[string]PlayerStats{}
	for _, key := range keys {
		ps, err := r.FindPlayerStats(ctx, key)
		if errors.Is(err, ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		out[key] = ps
	}
	return out, nil
}

func (r *kdbRepository) UpsertPlayerStats(ctx context.Context, ps PlayerStats) error {
	ps.ID = bson.ObjectID{}
	doc, err := db.MarshalDoc(ps)
	if err != nil {
		return err
	}
	return r.k.Put(db.NSPlayerStats, ps.SubjectKey, doc)
}

func (r *kdbRepository) Leaderboard(ctx context.Context, q LeaderboardQuery) ([]LeaderboardRow, error) {
	kind := q.Kind
	if kind == "" {
		kind = SubjectUser
	}
	limit := q.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	var rows []PlayerStats
	err := r.k.Scan(db.NSPlayerStats, func(doc []byte) error {
		var ps PlayerStats
		if err := db.UnmarshalDoc(doc, &ps); err != nil {
			return err
		}
		if ps.Subject.Kind == kind {
			rows = append(rows, ps)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return rankLeaderboard(rows, q.Scope, q.MinMatches, limit), nil
}
