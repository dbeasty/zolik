package rummytiles

import "zolik/server/internal/module"

// endRoundOut settles a round somebody went out: everyone else scores the
// negated value of their remaining hand, and the winner scores the sum of
// what everyone else lost — the one ending with a transfer.
func endRoundOut(s *GameState, winnerID string) []module.Event {
	deltas := map[string]int{}
	total := 0
	for _, p := range s.Players {
		if p == winnerID {
			continue
		}
		v := handValue(s.Hands[p])
		deltas[p] = -v
		total += v
	}
	deltas[winnerID] = total
	return endRound(s, RoundResult{Number: s.RoundNumber, Kind: "out", Winner: winnerID, Deltas: deltas})
}

// endRoundPoolExhausted settles a round nobody could finish: the pool ran
// dry and a player facing it had nothing to fall back on. Every player scores
// their own hand negated — there is no transfer, since nobody went out — and
// whether anyone is recorded as having "won" the round is the one thing the
// rules disagree on; see PoolExhaustionLowestWins in view.go.
func endRoundPoolExhausted(s *GameState) []module.Event {
	deltas := map[string]int{}
	lowest, lowestValue := "", 1<<30
	for _, p := range s.Players {
		v := handValue(s.Hands[p])
		deltas[p] = -v
		if v < lowestValue {
			lowestValue, lowest = v, p
		}
	}
	winner := ""
	if s.PoolExhaustionLowestWins {
		winner = lowest
	}
	return endRound(s, RoundResult{Number: s.RoundNumber, Kind: "pool_exhausted", Winner: winner, Deltas: deltas})
}

func matchWinner(s *GameState) (string, bool) {
	if s.TargetScore > 0 {
		for _, p := range s.Players {
			if s.Scores[p] >= s.TargetScore {
				return p, true
			}
		}
	}
	if s.RoundLimit > 0 && s.RoundNumber+1 >= s.RoundLimit {
		best, bestScore := "", -(1 << 30)
		for _, p := range s.Players {
			if s.Scores[p] > bestScore {
				bestScore, best = s.Scores[p], p
			}
		}
		return best, true
	}
	return "", false
}

func snapshotScores(s *GameState) map[string]int {
	out := make(map[string]int, len(s.Players))
	for _, p := range s.Players {
		out[p] = s.Scores[p]
	}
	return out
}

// endRound applies a round's deltas, records it, and either ends the match,
// pauses for the table to see it, or deals the next round.
func endRound(s *GameState, res RoundResult) []module.Event {
	for p, delta := range res.Deltas {
		s.Scores[p] += delta
	}
	res.Totals = snapshotScores(s)
	s.Rounds = append(s.Rounds, res)

	events := []module.Event{{Type: "round_ended", Data: map[string]any{
		"roundNumber": s.RoundNumber, "kind": res.Kind, "winner": res.Winner,
	}}}

	if winner, ok := matchWinner(s); ok {
		s.Status = "completed"
		s.WinnerID = winner
		s.Current = ""
		s.Workspace = nil
		return append(events, module.Event{Type: "match_ended", Data: map[string]any{"winnerId": winner}})
	}

	s.RoundNumber++
	if s.Pause {
		s.Intermission.Begin(s.RoundNumber)
		s.Current = ""
		s.Workspace = nil
		return events
	}
	dealRound(s)
	return append(events, module.Event{Type: "round_started", Data: map[string]any{"roundNumber": s.RoundNumber}})
}
