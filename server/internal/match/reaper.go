package match

import (
	"context"
	"log/slog"
	"time"

	"zolik/server/internal/metrics"
	"zolik/server/internal/models"
	"zolik/server/internal/rules"
)

// ReapInterval is how often the sweeper looks for tables to abandon. Well
// under AbandonWindow, so a match is abandoned within about half a minute of
// becoming eligible rather than at some arbitrary point inside the window.
const ReapInterval = 30 * time.Second

// reapBatch bounds one pass. A backlog — the first run after this ships, which
// meets every table ever stranded — is worked through over several ticks
// rather than in one long write burst against a database that is also serving
// games.
const reapBatch = 100

// StartReaper sweeps abandoned matches until ctx is cancelled.
//
// This closes a path that was never finished. SuspendOnDisconnect has always
// written AbandonAt — now plus AbandonWindow — and nothing has ever read it;
// rules.StatusAbandoned has existed as a constant that nothing assigns. So a
// table whose player closed the tab stayed `suspended` for ever: still listed
// as a live game, never resolved, and counted as neither finished nor
// abandoned. "How many games were not finished" had no answer because the
// runtime never decided that any game wasn't.
//
// The deliberate omission here is the statistics. An abandoned match writes no
// stats.MatchResult, and that is a rule rather than an oversight: the lifetime
// aggregates are rebuilt from those rows, so folding in a half-played table
// would corrupt every average it touched, for people who never agreed to play
// it out. Abandonment is a runtime fact and belongs to the daily counters
// alone.
func (m *Manager) StartReaper(ctx context.Context) {
	go func() {
		t := time.NewTicker(ReapInterval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				m.ReapAbandoned(context.WithoutCancel(ctx))
			}
		}
	}()
}

// ReapAbandoned runs one sweep and returns how many matches it abandoned.
func (m *Manager) ReapAbandoned(ctx context.Context) int {
	now := time.Now().UTC()
	due, err := m.repo.FindAbandonable(ctx, now, reapBatch)
	if err != nil {
		slog.Warn("could not look for abandoned matches", "error", err)
		return 0
	}

	var n int
	for _, match := range due {
		// Re-checked against the row as loaded rather than trusted from the
		// query: a player can return between the read and the write, and
		// ResumeIfReturning will have moved the match back to active. Losing
		// that race must mean the player keeps their game, so the version
		// check below is what actually decides it.
		if match.Status != "suspended" || match.AbandonAt == nil || match.AbandonAt.After(now) {
			continue
		}

		// Somebody is still sitting here. A table with a player at it is not
		// a stranded one, whatever the clock says — they are waiting for
		// their opponent, watching a board that says so, and sweeping it up
		// underneath them ends a game they were still in.
		//
		// This is what made a two-minute absence fatal to a game between two
		// people: their friend closes a laptop lid, and two minutes later the
		// player who never moved is told the table was set aside because
		// "nobody came back". Left to run, the table stays suspended — which
		// is the truth — and is swept on a later tick once the last person
		// has gone too, so nothing is leaked, only deferred.
		//
		// AbandonAt is not pushed out while they wait. The window measures
		// how long the missing player has been missing, and that is still the
		// right answer the moment the room empties.
		if m.attended(match) {
			slog.Debug("abandon deferred, somebody is still at the table",
				"match", match.ID.Hex(), "suspendedFor", suspendedFor(match, now))
			continue
		}

		expected := match.Version
		ended := now
		match.Status = string(rules.StatusAbandoned)
		match.EndedAt = &ended
		// AbandonAt is cleared so a row that somehow stays in this state is
		// not swept again on the next tick.
		match.AbandonAt = nil

		if err := m.repo.UpdateWithVersion(ctx, match.ID, expected, match); err != nil {
			// Overwhelmingly this is the version check refusing because the
			// player came back. Logged at debug rather than warn: it is the
			// system working, and at one line per returning player on a busy
			// evening it would drown the log.
			slog.Debug("abandon skipped, the match moved on",
				"match", match.ID.Hex(), "error", err)
			continue
		}
		match.Version = expected + 1
		n++

		m.metrics.Add(metrics.MatchesAbandoned, 1)
		m.metrics.Add(metrics.MatchesAbandonedFor(match.ModuleID), 1)
		slog.Info("match abandoned",
			"match", match.ID.Hex(),
			"module", match.ModuleID,
			"waitingOn", match.SuspendedPlayer,
			"suspendedFor", suspendedFor(match, now))

		// Told, not left to be discovered. Anyone still holding the table open
		// — a player who stayed while the others left, a spectator — sees it
		// resolve rather than sitting on a game that silently stopped being
		// one.
		m.Broadcast(match)
	}
	return n
}

func suspendedFor(match models.Match, now time.Time) time.Duration {
	if match.SuspendedAt == nil {
		return 0
	}
	return now.Sub(*match.SuspendedAt).Round(time.Second)
}
