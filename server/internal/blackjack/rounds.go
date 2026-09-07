package blackjack

import "zolik/server/internal/module"

var _ module.Rounded = (*Module)(nil)

// Rounds is Blackjack's round-by-round history.
//
// A stack says where a seat is; it says nothing about how it got there, and
// "which round did that" is the question a player asks after a bad one. The
// log names no card — it goes onto the wire and into a permanent record — but
// it does name the dealer's total, which is the fact everybody at the table
// was watching and nobody can reconstruct afterwards.
func (m *Module) Rounds(raw module.State) (module.RoundLog, error) {
	s, err := decode(raw)
	if err != nil {
		return module.RoundLog{}, err
	}

	log := module.RoundLog{
		LabelKey:   "blackjack.round.name",
		Rounds:     []module.RoundResult{},
		Paused:     s.Break.Open,
		WaitingFor: s.Break.Waiting(order(s)),
	}
	for _, r := range s.Rounds {
		out := module.RoundResult{
			Number:  r.Number,
			Winners: append([]string(nil), r.Winners...),
			Facts:   []module.Fact{dealerFact(r)},
		}
		for i := range s.Seats {
			pid := s.Seats[i].PlayerID
			stack, played := r.Stacks[pid]
			if !played {
				continue
			}
			score := module.RoundScore{
				// Chips already read upwards, so nothing is negated here.
				PlayerID: pid, Delta: r.Deltas[pid], Total: stack,
			}
			for _, outcome := range r.Outcomes[pid] {
				score.Facts = append(score.Facts, outcomeFact(outcome))
			}
			out.Scores = append(out.Scores, score)
		}
		log.Rounds = append(log.Rounds, out)
	}
	return log, nil
}

// dealerFact is what the dealer made of the round.
func dealerFact(r RoundSummary) module.Fact {
	switch {
	case r.DealerBlackjack:
		return module.Fact{LabelKey: "blackjack.round.dealerBlackjack"}
	case r.DealerBust:
		return module.Fact{
			LabelKey: "blackjack.round.dealerBust",
			Params:   map[string]any{"n": r.DealerTotal},
		}
	default:
		return module.Fact{
			LabelKey: "blackjack.round.dealerTotal",
			Params:   map[string]any{"n": r.DealerTotal},
		}
	}
}

// outcomeFact is one hand's result, as a key.
//
// A switch rather than a key built from the outcome string: a key assembled at
// runtime is invisible to the manifest that proves every key a server can send
// has wording, and an unworded key reaches a player as SCREAMING_SNAKE.
func outcomeFact(outcome string) module.Fact {
	switch outcome {
	case outcomeBlackjack:
		return module.Fact{LabelKey: "blackjack.round.outcome.blackjack"}
	case outcomeWin:
		return module.Fact{LabelKey: "blackjack.round.outcome.win"}
	case outcomePush:
		return module.Fact{LabelKey: "blackjack.round.outcome.push"}
	case outcomeBust:
		return module.Fact{LabelKey: "blackjack.round.outcome.bust"}
	case outcomeSurrender:
		return module.Fact{LabelKey: "blackjack.round.outcome.surrender"}
	default:
		return module.Fact{LabelKey: "blackjack.round.outcome.lose"}
	}
}
