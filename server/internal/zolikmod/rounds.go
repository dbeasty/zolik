package zolikmod

import (
	"strconv"

	"zolik/server/internal/module"
	"zolik/server/internal/rules"
)

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

	log := module.RoundLog{
		LabelKey:   "zolik.round.deal",
		Rounds:     []module.RoundResult{},
		Paused:     s.Break.Open,
		WaitingFor: s.Break.Waiting(gs.TurnOrder),
	}

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
		// rotates per deal in Continental, so it is a fact about the round
		// rather than about the match, and no client could reconstruct it.
		//
		// Sent as the count keys the bundle already carries, one per component,
		// rather than as a single key with the numbers in its params. Czech
		// inflects the noun by the count — jedna skupina, dvě skupiny, pět
		// skupin — so "{n} set(s)" is a sentence no translation survives, and
		// these keys exist precisely because that was already learned once.
		// Picking one by count is a lookup, not a rule.
		r.Facts = contractFacts(cfg.ContractFor(n + 1))

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

// contractFacts names what a deal required, and says nothing when it required
// nothing — a profile with a static, empty contract would otherwise repeat
// "needs nothing" on every row of the table.
func contractFacts(c rules.ContractRequirement) []module.Fact {
	var out []module.Fact
	// The count travels in the params as well as choosing the key: the small
	// numbers have their own phrasing and ignore it, and the generic key above
	// them places it itself.
	if c.Sets > 0 {
		out = append(out, module.Fact{
			LabelKey: countKey("contract.sets", c.Sets),
			Params:   map[string]any{"n": c.Sets},
		})
	}
	if c.Runs > 0 {
		out = append(out, module.Fact{
			LabelKey: countKey("contract.runs", c.Runs),
			Params:   map[string]any{"n": c.Runs},
		})
	}
	if c.RequireCleanRun {
		out = append(out, module.Fact{LabelKey: "zolik.round.cleanRun"})
	}
	return out
}

// countKey picks the bundle's phrasing for a count: one key each for the small
// numbers a language may inflect differently, and a generic one above them.
func countKey(prefix string, n int) string {
	if n >= 1 && n <= 3 {
		return prefix + "." + strconv.Itoa(n)
	}
	return prefix + ".n"
}
