package holdem

import "zolik/server/internal/module"

// Hold'em's rounds are its hands.
//
// It is the game least in need of stopping between them — a hand is four to ten
// actions and a freezeout is a hundred hands — and the game most in need of the
// table at the end of one. A stack says where you are; it says nothing about
// how you got there, and "which hand did that" is the whole question a poker
// player asks afterwards.
var _ module.Rounded = (*Module)(nil)

func (m *Module) Rounds(raw module.State) (module.RoundLog, error) {
	s, err := decode(raw)
	if err != nil {
		return module.RoundLog{}, err
	}

	log := module.RoundLog{
		LabelKey:   "holdem.round.hand",
		Rounds:     []module.RoundResult{},
		Paused:     s.Break.Open,
		WaitingFor: s.Break.Waiting(order(s)),
	}
	for _, h := range s.Hands {
		r := module.RoundResult{
			Number:  h.Number,
			Winners: append([]string(nil), h.Winners...),
		}
		if h.Pot > 0 {
			r.Facts = append(r.Facts, module.Fact{
				LabelKey: "holdem.round.pot", Params: map[string]any{"n": h.Pot},
			})
		}
		if h.Uncontested {
			r.Facts = append(r.Facts, module.Fact{LabelKey: "holdem.round.uncontested"})
		}
		// Never the board and never a hole card: this goes onto the wire and
		// into a permanent record, and a hand nobody called is never shown.

		for i := range s.Seats {
			pid := s.Seats[i].PlayerID
			stack, played := h.Stacks[pid]
			if !played {
				continue
			}
			r.Scores = append(r.Scores, module.RoundScore{
				PlayerID: pid,
				// Chips already read upwards, so nothing is negated here.
				Delta: h.Deltas[pid],
				Total: stack,
			})
		}
		log.Rounds = append(log.Rounds, r)
	}
	return log, nil
}
