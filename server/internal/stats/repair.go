package stats

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sort"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// PartnershipRepairReport is what RepairPartnerships found and did.
type PartnershipRepairReport struct {
	// Draws is every record stored as a draw.
	Draws int
	// Fixed is how many of those were one side winning, now stored as a win.
	Fixed int
	// SidesAdded is how many partnership records had no sides stored, and
	// now do.
	SidesAdded int
	// Rebuilt is how many lifetime records were replayed from the repaired
	// match records.
	Rebuilt int
}

func (r PartnershipRepairReport) String() string {
	return fmt.Sprintf("%d draws on record, %d were partnership wins, %d records given their sides, %d lifetime records rebuilt",
		r.Draws, r.Fixed, r.SidesAdded, r.Rebuilt)
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

// RepairPartnerships brings partnership match records written by older
// scoreboards up to date, and rebuilds the lifetime record of everyone who sat
// at them. Two things were wrong with those records:
//
//   - Until the scoreboard learned about sides, any match with more than one
//     winner was recorded as a draw, so every Canasta partnership's win went
//     down as a draw for both partners.
//   - Until the record kept its sides, a partner counted as an opponent: a
//     head-to-head row against them, a "vs humans" match for sitting beside a
//     friend, a "vs Hard" match for sitting beside a Hard bot.
//
// sidesOf answers who played with whom for one record, or nil where everyone
// played for themselves; it is asked only of a record with no sides stored.
// A record with no sides either way, and a draw between winners on different
// sides, is left exactly as it is.
//
// Safe to run twice: a repaired record has its sides and is no longer a draw,
// and a rebuild replays the same records to the same answer. With dry set it
// counts and writes nothing.
func RepairPartnerships(ctx context.Context, repo Repository, sidesOf func(MatchResult) [][]string, dry bool) (PartnershipRepairReport, error) {
	var (
		report  PartnershipRepairReport
		fix     []MatchResult
		touched = map[string]Subject{}
	)
	err := repo.EachMatch(ctx, func(m MatchResult) error {
		if m.IsDraw {
			report.Draws++
		}
		changed := false
		if m.Sides == nil {
			if m.Sides = copySides(sidesOf(m)); m.Sides == nil {
				return nil
			}
			report.SidesAdded++
			changed = true
		}
		if m.IsDraw && sidesAmong(m.Winners, m.Sides) <= 1 {
			m.IsDraw = false
			m.Participants = append([]Standing(nil), m.Participants...)
			for i := range m.Participants {
				m.Participants[i].Drew = false
			}
			report.Fixed++
			changed = true
		}
		if !changed {
			return nil
		}
		for _, p := range m.Participants {
			if p.Subject.Durable() {
				touched[p.Subject.Key()] = p.Subject
			}
		}
		fix = append(fix, m)
		return nil
	})
	if err != nil {
		return report, err
	}
	if dry {
		return report, nil
	}

	for _, m := range fix {
		if err := repo.ReplaceMatchPartnership(ctx, m); err != nil {
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

// StandInSeat names one seat of one recorded match: the record's match id and
// the seat's player id within it.
type StandInSeat struct {
	MatchID  bson.ObjectID
	PlayerID string
}

// StandInRepairReport is what CreditStandInSeats found and did.
type StandInRepairReport struct {
	// Credited is how many named seats were marked stood in for, and no
	// longer are.
	Credited int
	// Missing is how many named seats had no record, or no such seat.
	Missing int
	// Already is how many named seats were not marked in the first place.
	Already int
	// Rebuilt is how many lifetime records were replayed afterwards.
	Rebuilt int
}

func (r StandInRepairReport) String() string {
	return fmt.Sprintf("%d seats credited, %d already counted, %d not found, %d lifetime records rebuilt",
		r.Credited, r.Already, r.Missing, r.Rebuilt)
}

// CreditStandInSeats gives the named seats their result back and rebuilds the
// lifetime record of everyone at those matches.
//
// Until a stand-in was noted only when it played, putting one in was enough
// to take the result from its player: one that never moved, or only pressed
// Continue between rounds, still marked the seat. The match log does not say
// which moves a stand-in made, so these records cannot be told apart from
// real ones by reading them; the seats to credit are named by somebody who
// knows, the player who played them.
//
// Safe to run twice: a credited seat is no longer marked, and a rebuild
// replays the same records to the same answer. With dry set it counts and
// writes nothing.
func CreditStandInSeats(ctx context.Context, repo Repository, seats []StandInSeat, dry bool) (StandInRepairReport, error) {
	var (
		report  StandInRepairReport
		touched = map[string]Subject{}
	)
	for _, s := range seats {
		m, err := repo.FindMatchByMatchID(ctx, s.MatchID)
		if errors.Is(err, ErrNotFound) {
			report.Missing++
			continue
		}
		if err != nil {
			return report, fmt.Errorf("match %s: %w", s.MatchID.Hex(), err)
		}
		at := -1
		for i, p := range m.Participants {
			if p.PlayerID == s.PlayerID {
				at = i
			}
		}
		switch {
		case at < 0:
			report.Missing++
			continue
		case !m.Participants[at].StandIn:
			report.Already++
			continue
		}
		report.Credited++
		m.Participants = append([]Standing(nil), m.Participants...)
		m.Participants[at].StandIn = false
		for _, p := range m.Participants {
			if p.Subject.Durable() {
				touched[p.Subject.Key()] = p.Subject
			}
		}
		if dry {
			continue
		}
		if err := repo.ReplaceMatchPartnership(ctx, m); err != nil {
			return report, fmt.Errorf("match %s: %w", s.MatchID.Hex(), err)
		}
	}
	if dry {
		return report, nil
	}
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
