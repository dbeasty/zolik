package sietemedio

import "zolik/server/internal/module"

var _ module.Rounded = (*Module)(nil)

// Rounds is Siete y Medio's round-by-round history.
//
// A stack says where a seat is; it says nothing about how it got there, and
// "which round did that" is the question a player asks after a bad one. The log
// names no card — it goes onto the wire and into a permanent match record — but
// it does name who held the bank and what they made of it, which is the fact
// everybody at the table was watching and nobody can reconstruct afterwards.
func (m *Module) Rounds(raw module.State) (module.RoundLog, error) {
	s, err := decode(raw)
	if err != nil {
		return module.RoundLog{}, err
	}

	log := module.RoundLog{
		LabelKey:   "sietemedio.round.name",
		Rounds:     []module.RoundResult{},
		Paused:     s.Break.Open,
		WaitingFor: s.Break.Waiting(order(s)),
	}
	for _, r := range s.Rounds {
		out := module.RoundResult{
			Number:  r.Number,
			Winners: append([]string(nil), r.Winners...),
			Facts:   []module.Fact{bankFact(r)},
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
			if pid == r.Banker {
				score.Facts = append(score.Facts, module.Fact{LabelKey: "sietemedio.round.heldTheBank"})
			} else if outcome, ok := r.Outcomes[pid]; ok {
				score.Facts = append(score.Facts,
					module.Fact{
						LabelKey: "sietemedio.round.total",
						Value:    halvesText(r.Totals[pid]),
						Params:   map[string]any{"total": halvesText(r.Totals[pid])},
					},
					outcomeFact(outcome),
				)
			}
			out.Scores = append(out.Scores, score)
		}
		log.Rounds = append(log.Rounds, out)
	}
	return log, nil
}

// bankFact is what the bank made of the round.
func bankFact(r RoundSummary) module.Fact {
	switch {
	case r.BankSeven:
		return module.Fact{
			LabelKey: "sietemedio.round.bankSeven",
			Params:   map[string]any{"playerId": r.Banker},
		}
	case r.BankBust:
		return module.Fact{
			LabelKey: "sietemedio.round.bankBust",
			Value:    halvesText(r.BankTotal),
			Params:   map[string]any{"playerId": r.Banker, "total": halvesText(r.BankTotal)},
		}
	default:
		return module.Fact{
			LabelKey: "sietemedio.round.bankTotal",
			Value:    halvesText(r.BankTotal),
			Params:   map[string]any{"playerId": r.Banker, "total": halvesText(r.BankTotal)},
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
	case outcomeSeven:
		return module.Fact{LabelKey: "sietemedio.round.outcome.seven"}
	case outcomeWin:
		return module.Fact{LabelKey: "sietemedio.round.outcome.win"}
	case outcomePush:
		return module.Fact{LabelKey: "sietemedio.round.outcome.push"}
	case outcomeBust:
		return module.Fact{LabelKey: "sietemedio.round.outcome.bust"}
	default:
		return module.Fact{LabelKey: "sietemedio.round.outcome.lose"}
	}
}
