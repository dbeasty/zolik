package rummytiles

import "zolik/server/internal/module"

var _ module.Rounded = (*Module)(nil)

// Rounds is Rummy Tiles' round-by-round history.
func (m *Module) Rounds(raw module.State) (module.RoundLog, error) {
	s, err := decode(raw)
	if err != nil {
		return module.RoundLog{}, err
	}
	log := module.RoundLog{
		LabelKey:   "rummytiles.round.round",
		Rounds:     []module.RoundResult{},
		Paused:     s.Intermission.Open,
		WaitingFor: s.Intermission.Waiting(s.Players),
	}
	for _, rr := range s.Rounds {
		r := module.RoundResult{Number: rr.Number + 1}
		if rr.Winner != "" {
			r.Winners = []string{rr.Winner}
		}
		r.Facts = []module.Fact{{LabelKey: "rummytiles.round.kind", Value: rr.Kind}}
		for _, p := range s.Players {
			r.Scores = append(r.Scores, module.RoundScore{
				PlayerID: p, Delta: rr.Deltas[p], Total: rr.Totals[p],
			})
		}
		log.Rounds = append(log.Rounds, r)
	}
	return log, nil
}
