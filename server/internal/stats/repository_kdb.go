package stats

import (
	"context"
	"errors"
	"log"
	"sort"
	"strings"
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

// resultKey is the document key of a match's result: derived from the match,
// not from the record's own id.
//
// The unique matchId index is what makes recording a match idempotent under
// Mongo. Here the match id *is* the key, so a second recording of the same
// match — a retry, a replayed import from another node, two nodes recording
// the same finished match — collides on one document rather than being looked
// for by a scan each node can only run over its own copy.
func resultKey(matchID bson.ObjectID) string { return "mr:" + matchID.Hex() }

// Keys inside a player's read-only namespace, u/<hex>/ro. That namespace is
// written by the cloud and only read by that person's devices, which is what
// makes it the right place for anything a device must not be able to award
// itself: its rating, and the list of matches it is entitled to sync.
func indexedMatchKey(matchID bson.ObjectID) string { return "matches/" + matchID.Hex() }

const playerStatsKey = "stats"

// matchIndexEntry is what a device reads to know a match is theirs: enough to
// list it while offline, and the namespace to ask the hub for when they want
// to open it.
type matchIndexEntry struct {
	Kind        string        `bson:"_kind"`
	MatchID     bson.ObjectID `bson:"matchId"`
	ModuleID    string        `bson:"moduleId"`
	Variation   string        `bson:"variation,omitempty"`
	CompletedAt time.Time     `bson:"completedAt"`
	UpdatedAt   time.Time     `bson:"updatedAt"`
	// Finished is false for a match still being played, so a device can tell
	// the history from the game it could still go back to.
	Finished bool `bson:"finished"`
}

// indexMatchForPlayers records a finished match in each participating
// account's read-only namespace, so that account's devices can list it and
// know to sync the match itself.
func (r *kdbRepository) indexMatchForPlayers(m MatchResult) error {
	for _, key := range m.SubjectKeys {
		hex, ok := strings.CutPrefix(key, "user:")
		if !ok || hex == "" {
			// Guests and AI have no account to index against. A guest's
			// matches become indexable when they claim them by signing in.
			continue
		}
		entry := matchIndexEntry{
			Kind:        db.KindMatch,
			MatchID:     m.MatchID,
			ModuleID:    m.ModuleID,
			Variation:   m.Variation,
			CompletedAt: m.CompletedAt,
			UpdatedAt:   time.Now().UTC(),
			Finished:    true,
		}
		doc, err := db.MarshalDoc(entry)
		if err != nil {
			return err
		}
		if err := r.k.Put(db.UserReadOnlyNS(hex), indexedMatchKey(m.MatchID), doc); err != nil {
			return err
		}
	}
	return nil
}

func (r *kdbRepository) InsertMatch(ctx context.Context, m MatchResult) (MatchResult, error) {
	if m.ID.IsZero() {
		m.ID = bson.NewObjectID()
	}
	doc, err := db.MarshalDoc(m)
	if err != nil {
		return MatchResult{}, err
	}
	err = r.k.Update(db.NSMatchResults, func(tx *db.Tx) error {
		if err := tx.Insert(resultKey(m.MatchID), doc); err != nil {
			if db.IsDuplicateKey(err) {
				return ErrAlreadyRecorded
			}
			return err
		}
		return nil
	})
	if err == nil {
		// The index is written after the record rather than with it: a
		// participant's namespace is one per player, and a match of six would
		// otherwise make one transaction span seven namespaces for no gain.
		// An index entry that fails to land is re-derivable from the record,
		// which is the one document that must not be lost.
		if indexErr := r.indexMatchForPlayers(m); indexErr != nil {
			log.Printf("stats: indexing match %s for its players: %v", m.MatchID.Hex(), indexErr)
		}
	}
	if err != nil {
		return MatchResult{}, err
	}
	return m, nil
}

func (r *kdbRepository) FindMatchByMatchID(ctx context.Context, matchID bson.ObjectID) (MatchResult, error) {
	switch doc, err := r.k.Get(db.NSMatchResults, resultKey(matchID)); {
	case err == nil:
		var m MatchResult
		if err := db.UnmarshalDoc(doc, &m); err != nil {
			return MatchResult{}, err
		}
		return m, nil
	case !db.IsNotFound(err):
		return MatchResult{}, err
	}
	// Records written before results were keyed by their match; the scan goes
	// once cmd/migrate-namespaces has run everywhere.
	return r.scanMatchByMatchID(matchID)
}

func (r *kdbRepository) scanMatchByMatchID(matchID bson.ObjectID) (MatchResult, error) {
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
		key := resultKey(m.MatchID)
		doc, err := tx.Get(key)
		if db.IsNotFound(err) {
			// A record written before results were keyed by their match still
			// sits under its own id.
			key = m.ID.Hex()
			doc, err = tx.Get(key)
		}
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
		return tx.Put(key, next)
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
	if err := r.k.Put(db.NSPlayerStats, ps.SubjectKey, doc); err != nil {
		return err
	}
	// A copy for the account's own devices to read offline. It goes in the
	// read-only namespace because a rating a device could write is a rating a
	// device could invent.
	hex, ok := strings.CutPrefix(ps.SubjectKey, "user:")
	if !ok || hex == "" {
		return nil
	}
	mirrored, err := db.WithKind(doc, db.KindStats)
	if err != nil {
		return err
	}
	return r.k.Put(db.UserReadOnlyNS(hex), playerStatsKey, mirrored)
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
