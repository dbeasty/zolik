package zolikmod

import "zolik/server/internal/module"

// Žolíky's rounds are its deals.
//
// The history costs nothing to produce: the engine has kept per-deal scores in
// GameScores since long before anything asked to see them, so this is a
// straight map over what is already stored rather than a recomputation. That is
// also why a Žolíky round table is complete for matches that were already in
// flight when it shipped — only the per-deal winner, which had never been
// recorded, starts empty.
var _ module.Rounded = (*Module)(nil)

func (m *Module) Rounds(raw module.State) (module.RoundLog, error) {
	s, err := decode(raw)
	if err != nil {
		return module.RoundLog{}, err
	}
	gs := s.Rules
	cfg := gs.Rules

	log := module.RoundLog{LabelKey: "zolik.round.deal", Rounds: []module.RoundResult{}}

	// Deals are only complete once every seat has been scored for them, which
	// is what the shortest score array says. A deal in progress is the board,
	// not history.
	complete := -1
	for _, pid := range gs.TurnOrder {
		if n := len(gs.GameScores[pid]); complete < 0 || n < complete {
			complete = n
		}
	}
	if complete < 0 {
		complete = 0
	}

	running := map[string]int{}
	for n := 0; n < complete; n++ {
		r := module.RoundResult{Number: n + 1}
		if n < len(gs.DealWinners) && gs.DealWinners[n] != "" {
			r.Winners = []string{gs.DealWinners[n]}
		}
		// What this deal asked of a player before they could lay down. It
		// rotates per deal in Continental, so it is a fact about the round and
		// not about the match, and no client could reconstruct it.
		c := cfg.ContractFor(n + 1)
		r.Facts = []module.Fact{{
			LabelKey: "zolik.round.contract",
			Params: map[string]any{
				"sets": c.Sets, "runs": c.Runs, "cleanRun": c.RequireCleanRun,
			},
		}}

		for _, pid := range gs.TurnOrder {
			penalty := gs.GameScores[pid][n]
			running[pid] += penalty
			delta, total := penalty, running[pid]
			r.Scores = append(r.Scores, module.RoundScore{
				PlayerID: pid,
				// Negated, like Standing.Score, so the runtime never has to
				// know which way rummy counts; Shown is what a player reads.
				Delta:      -delta,
				Total:      -total,
				Shown:      &delta,
				ShownTotal: &total,
			})
		}
		log.Rounds = append(log.Rounds, r)
	}
	return log, nil
}
