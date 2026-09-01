package ginrummy

import "zolik/server/internal/module"

var _ module.Rounded = (*Module)(nil)

// Rounds is Gin Rummy's hand-by-hand history. Deltas already carry any line
// bonus settled on that hand (see scoring.go), so Total is arithmetic all the
// way to the last one — the same number Standings reports.
func (m *Module) Rounds(raw module.State) (module.RoundLog, error) {
	s, err := decode(raw)
	if err != nil {
		return module.RoundLog{}, err
	}

	log := module.RoundLog{
		LabelKey:   "ginrummy.round.hand",
		Rounds:     []module.RoundResult{},
		Paused:     s.Intermission.Open,
		WaitingFor: s.Intermission.Waiting(s.Players),
	}
	for _, hr := range s.Rounds {
		r := module.RoundResult{Number: hr.Number + 1}
		if hr.Winner != "" {
			r.Winners = []string{hr.Winner}
		}
		r.Facts = []module.Fact{{LabelKey: "ginrummy.round.kind", Value: hr.Kind}}
		for _, p := range s.Players {
			r.Scores = append(r.Scores, module.RoundScore{
				PlayerID: p, Delta: hr.Deltas[p], Total: hr.Totals[p],
			})
		}
		log.Rounds = append(log.Rounds, r)
	}
	return log, nil
}
