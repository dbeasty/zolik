package metrics

import (
	"context"
	"sort"
	"time"

	"zolik/server/internal/db"
)

// kdbStore keeps one document per day, keyed by the day string itself, and one
// per boot keyed by its id.
//
// Merge is a read-modify-write inside the namespace's write lock rather than
// the `$inc` the Mongo store gets. That is the same trade every KDB repository
// here makes: the engine has no update operators, and db.Update's critical
// section is what makes read-check-write atomic. At one flush every thirty
// seconds it is not a contended lock.
type kdbStore struct {
	k *db.KDB
}

// NewKDBStore builds the KDB-backed store.
func NewKDBStore(k *db.KDB) Store { return &kdbStore{k: k} }

var _ Store = (*kdbStore)(nil)

func (s *kdbStore) Merge(ctx context.Context, day string, counters map[string]int64, players []string, truncated bool) error {
	return s.k.Update(db.NSDailyMetrics, func(tx *db.Tx) error {
		var d Day
		raw, err := tx.Get(day)
		switch {
		case err == nil:
			if err := db.UnmarshalDoc(raw, &d); err != nil {
				return err
			}
		case db.IsNotFound(err):
			d = Day{Date: day}
		default:
			return err
		}

		d = mergeDay(d, day, counters, players, truncated)

		doc, err := db.MarshalDoc(d)
		if err != nil {
			return err
		}
		return tx.Put(day, doc)
	})
}

func (s *kdbStore) Days(ctx context.Context, from, to string) ([]Day, error) {
	// Direct gets over the range rather than a scan. The keys are the days
	// themselves, so the caller already knows every key it wants — and a
	// month is 31 lookups against a namespace that would otherwise be walked
	// in full.
	var out []Day
	for _, day := range daysBetween(from, to) {
		raw, err := s.k.Get(db.NSDailyMetrics, day)
		if db.IsNotFound(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		var d Day
		if err := db.UnmarshalDoc(raw, &d); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, nil
}

func (s *kdbStore) InsertBoot(ctx context.Context, b Boot) error {
	doc, err := db.MarshalDoc(b)
	if err != nil {
		return err
	}
	return s.k.Put(db.NSBoots, b.ID, doc)
}

func (s *kdbStore) LatestOpenBoot(ctx context.Context, exceptID string) (Boot, bool, error) {
	var best Boot
	var found bool
	err := s.k.Scan(db.NSBoots, func(raw []byte) error {
		var b Boot
		if err := db.UnmarshalDoc(raw, &b); err != nil {
			return err
		}
		if b.ID == exceptID || b.StoppedAt != nil || b.Counted {
			return nil
		}
		if !found || b.StartedAt.After(best.StartedAt) {
			best, found = b, true
		}
		return nil
	})
	if err != nil {
		return Boot{}, false, err
	}
	return best, found, nil
}

func (s *kdbStore) CloseBoot(ctx context.Context, id string, stoppedAt time.Time, reason string, counted bool) error {
	return s.k.Update(db.NSBoots, func(tx *db.Tx) error {
		raw, err := tx.Get(id)
		if err != nil {
			return err
		}
		var b Boot
		if err := db.UnmarshalDoc(raw, &b); err != nil {
			return err
		}
		b.StoppedAt = &stoppedAt
		if reason != "" {
			b.Reason = reason
		}
		if counted {
			b.Counted = true
		}
		doc, err := db.MarshalDoc(b)
		if err != nil {
			return err
		}
		return tx.Put(id, doc)
	})
}

func (s *kdbStore) Boots(ctx context.Context, from, to time.Time) ([]Boot, error) {
	var out []Boot
	err := s.k.Scan(db.NSBoots, func(raw []byte) error {
		var b Boot
		if err := db.UnmarshalDoc(raw, &b); err != nil {
			return err
		}
		if b.StartedAt.Before(from) || b.StartedAt.After(to) {
			return nil
		}
		out = append(out, b)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartedAt.Before(out[j].StartedAt) })
	return out, nil
}

// daysBetween lists every UTC day key from `from` to `to` inclusive.
//
// Bounded at maxReportDays so a malformed or hostile range cannot turn one
// request into an unbounded number of lookups. The reporting layer clamps the
// range as well; this is the backstop that means the store is safe on its own
// terms rather than only when called correctly.
func daysBetween(from, to string) []string {
	start, err := time.Parse(DayFormat, from)
	if err != nil {
		return nil
	}
	end, err := time.Parse(DayFormat, to)
	if err != nil {
		return nil
	}
	var out []string
	for d := start; !d.After(end) && len(out) < maxReportDays; d = d.AddDate(0, 0, 1) {
		out = append(out, d.Format(DayFormat))
	}
	return out
}

// maxReportDays is the longest range any single read may cover: a little over
// two years, which is longer than this deployment has existed and short enough
// that the worst case is bounded.
const maxReportDays = 800
