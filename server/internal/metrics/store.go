package metrics

import (
	"context"
	"sort"
	"time"
)

// Store is the persistence behind the recorder and the console.
//
// Merge is the only write, and it is an increment: two processes, or one
// process retrying, can call it for the same day without either losing the
// other's counts. That property is what lets the recorder hold a failed flush
// and try again rather than dropping it.
type Store interface {
	// Merge folds counters and newly-seen players into a day's document,
	// creating it if this is the day's first write. Counters are added, not
	// replaced. truncated is sticky: once a day has saturated its player set
	// it stays flagged.
	Merge(ctx context.Context, day string, counters map[string]int64, players []string, truncated bool) error

	// Days returns the documents for a closed range of UTC days, oldest
	// first. Days with nothing recorded are absent from the result — filling
	// the gaps is the reporting layer's job, and it needs to know which days
	// were genuinely empty in order to do it.
	Days(ctx context.Context, from, to string) ([]Day, error)

	// InsertBoot records a process start.
	InsertBoot(ctx context.Context, b Boot) error
	// LatestOpenBoot returns the most recent boot row with no StoppedAt,
	// excluding the one named by exceptID — the row this process just wrote
	// for itself. Returns ok=false when there is none, which is the normal
	// case after a clean shutdown.
	LatestOpenBoot(ctx context.Context, exceptID string) (Boot, bool, error)
	// CloseBoot stamps a boot row as stopped. Called from the shutdown path
	// with a reason, and from the next process's startup with Counted set,
	// to mark a dead predecessor as already counted.
	CloseBoot(ctx context.Context, id string, stoppedAt time.Time, reason string, counted bool) error
	// Boots lists boot rows started within a range, oldest first.
	Boots(ctx context.Context, from, to time.Time) ([]Boot, error)
}

// mergeDay applies a merge to a Day value. Shared by both engines so that
// "what merging means" — increments, a capped set, a sticky flag — is written
// once and cannot come out differently depending on which database is
// underneath.
func mergeDay(d Day, day string, counters map[string]int64, players []string, truncated bool) Day {
	d.Date = day
	if d.Counters == nil {
		d.Counters = make(map[string]int64, len(counters))
	}
	for name, n := range counters {
		d.Counters[name] += n
	}

	if len(players) > 0 {
		seen := make(map[string]struct{}, len(d.Players)+len(players))
		for _, k := range d.Players {
			seen[k] = struct{}{}
		}
		for _, k := range players {
			if _, ok := seen[k]; ok {
				continue
			}
			if len(seen) >= PlayerSetCap {
				d.PlayersTruncated = true
				break
			}
			seen[k] = struct{}{}
			d.Players = append(d.Players, k)
		}
		// Sorted so the stored document is stable between writes: a set with
		// no order makes every flush look like a change, which matters to
		// anything diffing backups and to reading one by eye.
		sort.Strings(d.Players)
	}

	d.PlayersTruncated = d.PlayersTruncated || truncated
	return d
}
