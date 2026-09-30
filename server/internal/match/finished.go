package match

import (
	"context"
	"sort"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"zolik/server/internal/db"
	"zolik/server/internal/models"
)

// Reading finished games in bulk, for the offline tools that learn from them.
//
// Nothing in the server asks this. It exists for cmd/export-games, which turns
// the stored move logs of finished matches into training records, and it is
// shaped for that: one game at a time, a window of end dates, oldest first so
// that a run cut short and started again with a later -since picks up where
// it stopped. It only reads — the tool that calls it must never be the reason
// a match changes — so there is no version, no lock, and nothing here that a
// write path could come to depend on.

// FinishedFilter narrows EachFinished.
type FinishedFilter struct {
	// ModuleID is the one game to list. Required: a training set is always
	// one game's, and a scan that returned every game would be a way of
	// reading everything by accident.
	ModuleID string
	// Statuses to include. Empty means FinishedStatuses — played to the end.
	// "abandoned" is the other resolved status, a game somebody walked away
	// from, and a caller has to ask for it.
	Statuses []string
	// EndedFrom and EndedBefore bound EndedAt, inclusive and exclusive. Zero
	// leaves that side open. A match with no EndedAt is only listed when
	// neither is set, because "we do not know when it ended" is not inside
	// any window.
	EndedFrom, EndedBefore time.Time
	// Limit caps how many matches are visited. Zero is no cap.
	Limit int
}

func (f FinishedFilter) statuses() []string {
	if len(f.Statuses) == 0 {
		return FinishedStatuses
	}
	return f.Statuses
}

// admits is the filter as a predicate on one envelope: the whole of it for
// the KDB scan, and the same rule the Mongo query spells as a document, so the
// two engines cannot disagree about which matches a window holds.
func (f FinishedFilter) admits(m models.Match) bool {
	if m.ModuleID != f.ModuleID {
		return false
	}
	ok := false
	for _, s := range f.statuses() {
		if m.Status == s {
			ok = true
			break
		}
	}
	if !ok {
		return false
	}
	if f.EndedFrom.IsZero() && f.EndedBefore.IsZero() {
		return true
	}
	if m.EndedAt == nil {
		return false
	}
	if !f.EndedFrom.IsZero() && m.EndedAt.Before(f.EndedFrom) {
		return false
	}
	if !f.EndedBefore.IsZero() && !m.EndedAt.Before(f.EndedBefore) {
		return false
	}
	return true
}

// endedFirst orders matches by when they ended, then by id, so two matches
// ending in the same millisecond still come back in a stable order.
func endedFirst(a, b models.Match) bool {
	var ta, tb time.Time
	if a.EndedAt != nil {
		ta = *a.EndedAt
	}
	if b.EndedAt != nil {
		tb = *b.EndedAt
	}
	if !ta.Equal(tb) {
		return ta.Before(tb)
	}
	return a.ID.Hex() < b.ID.Hex()
}

// EachFinished streams the matching envelopes off a cursor, so an export of a
// year of games holds one envelope at a time rather than a year of them.
func (r *mongoRepository) EachFinished(ctx context.Context, f FinishedFilter, visit func(models.Match) error) error {
	q := bson.M{"moduleId": f.ModuleID, "status": bson.M{"$in": f.statuses()}}
	ended := bson.M{}
	if !f.EndedFrom.IsZero() {
		ended["$gte"] = f.EndedFrom
	}
	if !f.EndedBefore.IsZero() {
		ended["$lt"] = f.EndedBefore
	}
	if len(ended) > 0 {
		q["endedAt"] = ended
	}
	opts := options.Find().SetSort(bson.D{{Key: "endedAt", Value: 1}, {Key: "_id", Value: 1}})
	if f.Limit > 0 {
		opts.SetLimit(int64(f.Limit))
	}
	cur, err := r.coll.Find(ctx, q, opts)
	if err != nil {
		return err
	}
	defer cur.Close(ctx)
	for cur.Next(ctx) {
		var m models.Match
		if err := cur.Decode(&m); err != nil {
			return err
		}
		if err := visit(m); err != nil {
			return err
		}
	}
	return cur.Err()
}

// EachFinished scans the index, like every cross-match read on this backend.
//
// The matches are collected and then visited, rather than visited from inside
// the scan: a visitor reads each match's moves, and doing that while the scan
// still holds the index would be a read nested inside another for the length
// of an export. Envelopes are small — the board and the moves are not on them
// — so holding the window's worth is cheap.
func (r *kdbRepository) EachFinished(ctx context.Context, f FinishedFilter, visit func(models.Match) error) error {
	var out []models.Match
	err := r.k.Scan(db.NSMatches, func(raw []byte) error {
		var m models.Match
		if err := db.UnmarshalDoc(raw, &m); err != nil {
			return err
		}
		if f.admits(m) {
			out = append(out, m)
		}
		return nil
	})
	if err != nil {
		return err
	}
	sort.Slice(out, func(i, j int) bool { return endedFirst(out[i], out[j]) })
	if f.Limit > 0 && len(out) > f.Limit {
		out = out[:f.Limit]
	}
	for _, m := range out {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := visit(m); err != nil {
			return err
		}
	}
	return nil
}
