package ginrummy

import "zolik/server/internal/module"

// Scoring a hand, and where match end sits inside it.
//
// A hand's raw score is what the rules define — the deadwood difference, an
// undercut, or a gin bonus — and it is what decides whether a hand is won, so
// it has to be computed before anything checks the target. The line bonuses
// (boxes and the game bonus) are settled only once, on the hand that actually
// crosses the target, and they are folded into that hand's own Deltas so the
// round log's arithmetic (Total == previous Total + Delta) holds even on the
// hand the match ends on — see allmodules_test.go's TestARoundLogIsArithmetic,
// which is exactly the kind of thing a plan text does not warn you about.

// scoreHand settles the hand that just ended in a knock or gin. The
// defender's hand is never re-melded — only the knocker's is, by rule — so
// the defender's deadwood is simply whatever is left in their hand after any
// lay-offs.
func scoreHand(s *GameState) HandResult {
	knocker := s.Knocker
	defender := other(s.Players, knocker)
	defenderDeadwood := handValue(s.Hands[defender])
	knockerDeadwood := s.KnockerDeadwood

	var winner, kind string
	var delta int
	switch {
	case s.KnockGin:
		winner, kind, delta = knocker, "gin", defenderDeadwood+25
	case knockerDeadwood < defenderDeadwood:
		winner, kind, delta = knocker, "knock", defenderDeadwood-knockerDeadwood
	default:
		winner, kind, delta = defender, "undercut", (knockerDeadwood-defenderDeadwood)+25
	}

	s.Scores[winner] += delta
	s.HandsWon[winner]++

	return HandResult{
		Number:           s.HandNumber,
		Kind:             kind,
		Winner:           winner,
		HandDelta:        delta,
		KnockerDeadwood:  knockerDeadwood,
		DefenderDeadwood: defenderDeadwood,
		Deltas:           map[string]int{winner: delta, other(s.Players, winner): 0},
	}
}

// applyLineBonuses settles the boxes and the game bonus, once, on the hand
// that crosses the target. "The loser scored nothing" is read from the raw
// hand score, before any bonus — a player who never won a hand has zero boxes
// regardless, so the order only matters for the shutout check.
func applyLineBonuses(s *GameState, winner string, res *HandResult) {
	if !s.LineBonuses {
		return
	}
	loser := other(s.Players, winner)
	shutout := s.Scores[loser] == 0

	game := 100
	if shutout {
		game = 200
	}
	winnerBonus := 25*s.HandsWon[winner] + game
	loserBonus := 25 * s.HandsWon[loser]

	s.Scores[winner] += winnerBonus
	s.Scores[loser] += loserBonus
	res.Deltas[winner] += winnerBonus
	res.Deltas[loser] += loserBonus
}

func matchWinner(s *GameState) (string, bool) {
	for _, p := range s.Players {
		if s.Scores[p] >= s.TargetScore {
			return p, true
		}
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

// endHand scores the hand just knocked or ginned shut, and either deals the
// next one or ends the match.
func endHand(s *GameState) []module.Event {
	res := scoreHand(s)
	events := []module.Event{{Type: "hand_ended", Data: map[string]any{
		"handNumber": s.HandNumber, "winner": res.Winner, "kind": res.Kind, "delta": res.HandDelta,
	}}}

	if winner, ok := matchWinner(s); ok {
		applyLineBonuses(s, winner, &res)
		res.Totals = snapshotScores(s)
		s.Rounds = append(s.Rounds, res)

		s.Status = "completed"
		s.WinnerID = winner
		s.Current = ""
		s.Phase = ""
		s.Knocker = ""
		s.KnockerMelds = nil
		return append(events, module.Event{Type: "match_ended", Data: map[string]any{"winnerId": winner}})
	}

	res.Totals = snapshotScores(s)
	s.Rounds = append(s.Rounds, res)
	s.Dealer = other(s.Players, s.Dealer)
	s.HandNumber++
	s.Knocker = ""
	s.KnockerMelds = nil

	if s.Pause {
		s.Intermission.Begin(s.HandNumber)
		s.Current = ""
		s.Phase = ""
		return events
	}
	dealHand(s)
	return append(events, module.Event{Type: "hand_started", Data: map[string]any{"handNumber": s.HandNumber}})
}

// deadHand ends a hand nobody scores: the stock ran down to its last two
// cards and the player who faced that did not knock. The same dealer redeals.
func deadHand(s *GameState) []module.Event {
	s.Rounds = append(s.Rounds, HandResult{
		Number: s.HandNumber, Kind: "dead",
		Deltas: map[string]int{s.Players[0]: 0, s.Players[1]: 0},
		Totals: snapshotScores(s),
	})
	events := []module.Event{{Type: "hand_dead", Data: map[string]any{"handNumber": s.HandNumber}}}
	s.HandNumber++
	s.Knocker = ""
	s.KnockerMelds = nil

	if s.Pause {
		s.Intermission.Begin(s.HandNumber)
		s.Current = ""
		s.Phase = ""
		return events
	}
	dealHand(s)
	return append(events, module.Event{Type: "hand_started", Data: map[string]any{"handNumber": s.HandNumber}})
}
