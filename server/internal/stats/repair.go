package stats

import (
	"context"
	"fmt"
	"log"
	"sort"
)

// DrawRepairReport is what RepairPartnershipDraws found and did.
type DrawRepairReport struct {
	// Draws is every record stored as a draw.
	Draws int
	// Fixed is how many of those were one side winning, now stored as a win.
	Fixed int
	// Rebuilt is how many lifetime records were replayed from the repaired
	// match records.
	Rebuilt int
}

func (r DrawRepairReport) String() string {
	return fmt.Sprintf("%d draws on record, %d were partnership wins, %d lifetime records rebuilt",
		r.Draws, r.Fixed, r.Rebuilt)
}

// SeatOrder is a record's player ids in the order they sat — the order a
// module's Seated.Sides answers in.
func SeatOrder(m MatchResult) []string {
	seats := append([]Standing(nil), m.Participants...)
	sort.SliceStable(seats, func(i, j int) bool { return seats[i].Seat < seats[j].Seat })
	out := make([]string, len(seats))
	for i, s := range seats {
		out[i] = s.PlayerID
	}
	return out
}

// RepairPartnershipDraws finds match records stored as draws whose winners all
// played on one side, stores them as the win they were, and rebuilds the
// lifetime record of everyone who sat at them.
//
// Until the scoreboard learned about sides, any match with more than one
// winner was recorded as a draw, so every Canasta partnership's win went down
// as a draw for both partners. sidesOf answers who played with whom for one
// record, or nil where everyone played for themselves; such a record, and a
// draw between winners on different sides, is left exactly as it is.
//
// Safe to run twice: a repaired record is no longer a draw, and a rebuild
// replays the same records to the same answer. With dry set it counts and
// writes nothing.
func RepairPartnershipDraws(ctx context.Context, repo Repository, sidesOf func(MatchResult) [][]string, dry bool) (DrawRepairReport, error) {
	var (
		report  DrawRepairReport
		fix     []MatchResult
		touched = map[string]Subject{}
	)
	err := repo.EachMatch(ctx, func(m MatchResult) error {
		if !m.IsDraw {
			return nil
		}
		report.Draws++
		sides := sidesOf(m)
		if sides == nil || sidesAmong(m.Winners, sides) > 1 {
			return nil
		}
		m.IsDraw = false
		m.Participants = append([]Standing(nil), m.Participants...)
		for i := range m.Participants {
			m.Participants[i].Drew = false
			if s := m.Participants[i].Subject; s.Durable() {
				touched[s.Key()] = s
			}
		}
		fix = append(fix, m)
		return nil
	})
	if err != nil {
		return report, err
	}
	report.Fixed = len(fix)
	if dry {
		return report, nil
	}

	for _, m := range fix {
		if err := repo.ReplaceMatchDraw(ctx, m); err != nil {
			return report, fmt.Errorf("match %s: %w", m.MatchID.Hex(), err)
		}
	}
	// The aggregates are derived, so they are replayed from the records rather
	// than patched — the same path a claimed guest history takes.
	claimer := NewClaimer(repo)
	keys := make([]string, 0, len(touched))
	for k := range touched {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if err := claimer.RebuildPlayerStats(ctx, touched[k]); err != nil {
			log.Printf("subject=%s: rebuild failed: %v", k, err)
			continue
		}
		report.Rebuilt++
	}
	return report, nil
}
