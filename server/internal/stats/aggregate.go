package stats

import (
	"sort"
	"strconv"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// BuildMatchResult turns a completed scoreboard into the permanent record.
// Pure — the timestamps are passed in rather than read from the clock, so the
// result is reproducible and testable.
func BuildMatchResult(sb Scoreboard, gameID bson.ObjectID, startedAt, completedAt, now time.Time) MatchResult {
	keys := map[string]bool{}
	for _, s := range sb.Standings {
		if k := s.Subject.Key(); k != "" {
			keys[k] = true
		}
	}
	flat := make([]string, 0, len(keys))
	for k := range keys {
		flat = append(flat, k)
	}
	sort.Strings(flat)

	duration := 0
	if !startedAt.IsZero() && completedAt.After(startedAt) {
		duration = int(completedAt.Sub(startedAt).Seconds())
	}

	return MatchResult{
		GameID:          gameID,
		RulesProfile:    sb.RulesProfile,
		MatchEndMode:    sb.MatchEndMode,
		TargetScore:     sb.TargetScore,
		DealCount:       sb.DealCount,
		StartedAt:       startedAt,
		CompletedAt:     completedAt,
		DurationSeconds: duration,
		DealsPlayed:     sb.DealsPlayed,
		Composition:     sb.Composition,
		Participants:    sb.Standings,
		SubjectKeys:     flat,
		WinnerPlayerID:  sb.WinnerID,
		IsDraw:          sb.IsDraw,
		RecordedAt:      now,
	}
}

// ApplyMatch folds one finished match into a subject's lifetime record and
// returns the updated copy. It is pure and idempotent-unsafe by design: the
// caller guarantees each match is applied exactly once, via the unique index
// on MatchResult.GameID.
//
// seat identifies which participant of the match this record belongs to. It is
// passed explicitly rather than looked up by subject key because an AI subject
// can hold more than one seat at the same table, and each seat is folded in
// separately.
func ApplyMatch(ps PlayerStats, m MatchResult, seat Standing, now time.Time) PlayerStats {
	if ps.SubjectKey == "" {
		ps.SubjectKey = seat.Subject.Key()
	}
	// The stored subject tracks the latest display name, so a renamed user's
	// leaderboard row does not keep showing the name they had on their first
	// ever match.
	ps.Subject = seat.Subject

	opponents := opponentsOf(m, seat)

	ps.Overall.Add(seat, m.IsDraw)

	if anyHuman(opponents) {
		ps.VsHumans.Add(seat, m.IsDraw)
	}
	if anyAI(opponents) {
		ps.VsAI.Add(seat, m.IsDraw)
	}

	for _, diff := range opponentAIDifficulties(opponents) {
		ps.ByAIDifficulty = addTo(ps.ByAIDifficulty, diff, seat, m.IsDraw)
	}
	if m.RulesProfile != "" {
		ps.ByProfile = addTo(ps.ByProfile, m.RulesProfile, seat, m.IsDraw)
	}
	ps.ByPlayerCount = addTo(ps.ByPlayerCount, strconv.Itoa(m.Composition.Players), seat, m.IsDraw)

	ps.HeadToHead = applyHeadToHead(ps.HeadToHead, seat, opponents, m.CompletedAt)

	applyStreak(&ps, seat, m.IsDraw)

	if ps.FirstMatchAt.IsZero() || m.CompletedAt.Before(ps.FirstMatchAt) {
		ps.FirstMatchAt = m.CompletedAt
	}
	if m.CompletedAt.After(ps.LastMatchAt) {
		ps.LastMatchAt = m.CompletedAt
	}

	ps.RecentMatches = prependRecent(ps.RecentMatches, MatchRef{
		MatchID:       m.ID,
		GameID:        m.GameID,
		PlayedAt:      m.CompletedAt,
		RulesProfile:  m.RulesProfile,
		Players:       m.Composition.Players,
		Rank:          seat.Rank,
		Total:         seat.Total,
		Won:           seat.Won,
		Drew:          m.IsDraw && seat.Drew,
		Outcome:       outcomeOf(seat, m.IsDraw),
		AgainstAI:     anyAI(opponents),
		AgainstHumans: anyHuman(opponents),
	})

	ps.UpdatedAt = now
	return ps
}

// opponentsOf returns every participant other than the given seat. It matches
// on player ID, not subject, so that when one AI difficulty holds two seats
// each bot correctly counts as the other's opponent.
func opponentsOf(m MatchResult, seat Standing) []Standing {
	out := make([]Standing, 0, len(m.Participants))
	for _, p := range m.Participants {
		if p.PlayerID == seat.PlayerID {
			continue
		}
		out = append(out, p)
	}
	return out
}

func anyHuman(ss []Standing) bool {
	for _, s := range ss {
		if s.Subject.IsHuman() {
			return true
		}
	}
	return false
}

func anyAI(ss []Standing) bool {
	for _, s := range ss {
		if s.Subject.Kind == SubjectAI {
			return true
		}
	}
	return false
}

// opponentAIDifficulties lists the distinct bot difficulties faced, so a table
// holding two hard bots credits "vs hard" once rather than twice.
func opponentAIDifficulties(ss []Standing) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, s := range ss {
		if s.Subject.Kind != SubjectAI || seen[s.Subject.ID] {
			continue
		}
		seen[s.Subject.ID] = true
		out = append(out, s.Subject.ID)
	}
	sort.Strings(out)
	return out
}

func addTo(m map[string]Tally, key string, seat Standing, isDraw bool) map[string]Tally {
	if m == nil {
		m = map[string]Tally{}
	}
	t := m[key]
	t.Add(seat, isDraw)
	m[key] = t
	return m
}

// applyHeadToHead records the pairwise result against every durable opponent.
// "Ahead" is decided on final total rather than on who won the match, so in a
// four-player game the two players who finished third and fourth still get a
// meaningful record against each other.
func applyHeadToHead(h map[string]HeadToHead, seat Standing, opponents []Standing, at time.Time) map[string]HeadToHead {
	for _, o := range opponents {
		key := o.Subject.Key()
		if key == "" || key == seat.Subject.Key() {
			// Guests have no durable identity, and a subject facing itself
			// (two bots of one difficulty) would only record a wash.
			continue
		}
		if h == nil {
			h = map[string]HeadToHead{}
		}
		rec := h[key]
		rec.Subject = o.Subject
		rec.Matches++
		switch {
		case seat.Total < o.Total:
			rec.Ahead++
		case seat.Total > o.Total:
			rec.Behind++
		default:
			rec.Level++
		}
		rec.PointsFor += seat.Total
		rec.PointsAgainst += o.Total
		if at.After(rec.LastPlayedAt) {
			rec.LastPlayedAt = at
		}
		h[key] = rec
	}
	return h
}

// applyStreak advances the win/loss streak. A draw breaks both directions
// rather than extending either — it is neither a win nor a loss, and letting
// it extend a win streak would overstate the run.
func applyStreak(ps *PlayerStats, seat Standing, isDraw bool) {
	switch {
	case isDraw && seat.Drew:
		ps.CurrentStreak = 0
	case seat.Won:
		if ps.CurrentStreak < 0 {
			ps.CurrentStreak = 0
		}
		ps.CurrentStreak++
		if ps.CurrentStreak > ps.LongestWinStreak {
			ps.LongestWinStreak = ps.CurrentStreak
		}
	default:
		if ps.CurrentStreak > 0 {
			ps.CurrentStreak = 0
		}
		ps.CurrentStreak--
		if -ps.CurrentStreak > ps.LongestLossStreak {
			ps.LongestLossStreak = -ps.CurrentStreak
		}
	}
}

func outcomeOf(seat Standing, isDraw bool) string {
	switch {
	case isDraw && seat.Drew:
		return "draw"
	case seat.Won:
		return "win"
	default:
		return "loss"
	}
}

func prependRecent(list []MatchRef, ref MatchRef) []MatchRef {
	out := append([]MatchRef{ref}, list...)
	if len(out) > recentMatchesKept {
		out = out[:recentMatchesKept]
	}
	return out
}
