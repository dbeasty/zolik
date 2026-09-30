package ai

import "zolik/server/internal/rules"

// InitialMeldPlan is the heuristic's own opening search, for callers outside
// this package that have to plan a first meld the way the agent does — the
// learning adapter (zolikmod/learn.go), which offers a trained network whole
// openings and seeds its search with this one.
//
// It is findInitialMeldPlanRequiring, answered from the table as the rules
// engine holds it: the melds already down, who is down, the deal and the
// ruleset. Nothing else of the state is read, so a caller cannot peek through
// it. mustInclude is a card the plan has to spend ("" for none) — the card a
// discard-pile pickup obliges the opening to use.
//
// A plan is a proposal, never a permission: the caller still puts every meld
// of it to the engine, which is the only judge of whether it is legal.
func InitialMeldPlan(gs rules.GameState, playerID string, hand []string, mustInclude string) ([][]string, bool) {
	st := rules.GameState{
		Rules:       rules.ResolveConfig(gs.Rules),
		GameNumber:  gs.GameNumber,
		RoundReqMet: gs.RoundReqMet,
		Melds:       gs.Melds,
		MeldMeta:    gs.MeldMeta,
	}
	return findInitialMeldPlanRequiring(st, playerID, hand, mustInclude)
}
