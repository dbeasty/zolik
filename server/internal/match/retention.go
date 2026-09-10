package match

import (
	"context"
	"log/slog"
	"time"

	"zolik/server/internal/metrics"
	"zolik/server/internal/models"
	"zolik/server/internal/rules"
)

// How long a resolved match is kept before its row is deleted.
//
// This is the answer to a question the runtime had never actually been asked:
// *at what point do we delete the game?* Nothing ever deleted a finished match,
// so `matches` grew without bound — on the production snapshot it was 7.1 MB of
// a 7.7 MB database, while `match_results`, the permanent record every lifetime
// figure is rebuilt from, was 28 KB. The bulk is `State` and the append-only
// `ActionLog`: the machinery of playing a game, which stops meaning anything
// once nobody is going to open the table again.
//
// The thing that must not be repeated is how the last deletion worked. An
// `abandonAt` TTL treated a field the runtime *decides* on as a field the store
// could *delete* on, so two timers raced and the loser's games vanished. So
// these windows key off `endedAt` and `createdAt` — facts about the past that
// nothing will revise — and a match is only ever a candidate once it has
// reached a status it can never leave on its own.
//
// A zero window disables that class entirely, which is how an operator who
// wants to keep everything says so.
type RetentionWindows struct {
	// Lobby is a table that was opened and never started: no game was played,
	// nobody's history contains it, and it is not in any count of unfinished
	// games. Pure litter, and the shortest window by a long way.
	Lobby time.Duration
	// Completed is a game played to the end. Its permanent record was written
	// to match_results when it finished, so what expires here is the board it
	// finished on, not the fact that it happened or anybody's statistics.
	Completed time.Duration
	// Abandoned is a table the reaper set aside. It writes no MatchResult, so
	// deleting it does lose the only trace of that particular game — but
	// abandonment is a runtime fact carried by the daily counters, and the
	// window has to be long enough that resuming stays a real offer rather
	// than a promise the sweeper quietly breaks.
	Abandoned time.Duration
}

// Any reports whether any class is actually being retired, so a deployment that
// has switched the whole thing off does not run a sweeper that can never act.
func (w RetentionWindows) Any() bool {
	return w.Lobby > 0 || w.Completed > 0 || w.Abandoned > 0
}

// RetentionInterval is how often the sweeper looks for rows to delete.
//
// Far slower than the reaper's tick, because nothing here is time-critical:
// the shortest window is hours, so an hour's lag either way changes nothing a
// player or an operator could notice, and a delete pass is the most expensive
// thing this process does to the database.
const RetentionInterval = time.Hour

// retentionBatch bounds one pass. The first run after this ships meets every
// match ever played, and works through it over successive ticks rather than in
// one long delete burst against a database that is also serving games.
const retentionBatch = 200

// StartRetention deletes resolved matches past their window until ctx is
// cancelled. A windows value with nothing enabled starts nothing.
func (m *Manager) StartRetention(ctx context.Context, w RetentionWindows) {
	if !w.Any() {
		slog.Info("match retention: off, every match is kept")
		return
	}
	slog.Info("match retention: on",
		"lobby", w.Lobby, "completed", w.Completed, "abandoned", w.Abandoned)
	go func() {
		t := time.NewTicker(RetentionInterval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				m.SweepRetired(context.WithoutCancel(ctx), w)
			}
		}
	}()
}

// SweepRetired runs one pass and returns how many matches it deleted.
func (m *Manager) SweepRetired(ctx context.Context, w RetentionWindows) int {
	now := time.Now().UTC()
	due, err := m.repo.FindRetired(ctx, now, w, retentionBatch)
	if err != nil {
		slog.Warn("could not look for matches to retire", "error", err)
		return 0
	}

	var n int
	for _, match := range due {
		// Re-checked against the row as loaded rather than trusted from the
		// query, and then deleted under a version guard. Both matter, and the
		// second one is new: an abandoned table can be *resumed* now, so a row
		// that was a deletion candidate when the scan read it may be a live
		// game by the time this loop reaches it. Losing that race has to mean
		// the player keeps their game.
		if !retired(match, now, w) {
			continue
		}
		if err := m.repo.DeleteIfUnchanged(ctx, match.ID, match.Version); err != nil {
			slog.Debug("retirement skipped, the match moved on",
				"match", match.ID.Hex(), "error", err)
			continue
		}
		n++
		m.metrics.Add(metrics.MatchesDeleted, 1)
	}
	if n > 0 {
		slog.Info("matches retired", "deleted", n)
	}
	return n
}

// retired reports whether a match has reached a status it cannot leave and has
// sat there past that status's window.
//
// Deliberately total over the statuses rather than a default: `active` and
// `suspended` are absent because a table in either can still become something
// else on its own, and a sweeper that deleted one would be deleting a game
// somebody is in the middle of. Anything this does not name is kept.
func retired(m models.Match, now time.Time, w RetentionWindows) bool {
	switch m.Status {
	case "lobby":
		// Keyed off CreatedAt because a lobby has no other date: it never
		// started, so it never ended.
		return w.Lobby > 0 && now.Sub(m.CreatedAt) > w.Lobby
	case "completed":
		return w.Completed > 0 && endedBefore(m, now.Add(-w.Completed))
	case string(rules.StatusAbandoned):
		return w.Abandoned > 0 && endedBefore(m, now.Add(-w.Abandoned))
	default:
		return false
	}
}

// endedBefore is false for a resolved match with no end time rather than true.
//
// Such a row should not exist — both writers set EndedAt — but "we do not know
// when this finished" is not a reason to delete it, and a nil dereference here
// would take out the sweeper for every other row in the batch.
func endedBefore(m models.Match, cutoff time.Time) bool {
	return m.EndedAt != nil && m.EndedAt.Before(cutoff)
}
