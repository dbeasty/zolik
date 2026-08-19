package ai

import (
	"math/rand"
	"time"

	"zolik/server/internal/rules"
)

var AINames = map[string][]string{
	"easy":   {"Rookie Rita", "Lucky Lukáš", "Wobbly Wanda", "Slow Štefan"},
	"medium": {"Clever Karel", "Sharp Šárka", "Steady Stanislav", "Crafty Klára"},
	"hard":   {"Master Miroslav", "Shark Soňa", "Iron Ivan", "Relentless Radka"},
}

type HeuristicAgent struct {
	difficulty string
	rnd        *rand.Rand
}

func NewHeuristicAgent(difficulty string) *HeuristicAgent {
	return &HeuristicAgent{
		difficulty: difficulty,
		rnd:        rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (a *HeuristicAgent) Difficulty() string { return a.difficulty }

func (a *HeuristicAgent) ChooseAction(visible VisibleState, hand []string) rules.Action {
	visible.Rules = rules.ResolveConfig(visible.Rules)
	// Priority order (simplified v1):
	// 1) Meld phase: only start laying toward the round requirement if the
	// current hand can complete it entirely (pattern + minimum value) this
	// turn — a player who starts but can't finish gets stuck (the server
	// won't let them discard until they finish), so never begin unless a
	// full plan exists. Re-derived fresh each call so it naturally picks up
	// where a previous meld this turn left off.
	if visible.Phase == string(rules.PhaseMeld) {
		actor := visible.CurrentTurn
		if !visible.RoundReqMet[actor] {
			st := rulesStateForAI(visible, actor)
			if combo, ok := findInitialMeldPlan(st, actor, hand); ok && len(combo) > 0 {
				return rules.Action{Type: rules.ActionLayMeld, Cards: combo[0]}
			}
		} else {
			// Already down: shed cards one at a time onto any table meld
			// (own or another player's) before trying a brand-new meld —
			// otherwise a hand with no full new meld left in it can never
			// shrink to zero and the deal never ends.
			if meldID, card, ok := findLayOff(visible.MeldMeta, visible.Melds, hand, visible.Rules, visible.GameNumber); ok {
				return rules.Action{Type: rules.ActionLayOff, MeldID: meldID, Card: card}
			}
			if meld, ok := findAnyValidMeld(hand, visible.Rules); ok && len(hand) > len(meld) {
				return rules.Action{Type: rules.ActionLayMeld, Cards: meld}
			}
		}
		// Otherwise discard.
		canDiscardJoker := len(hand) == 1 && visible.RoundReqMet[actor]
		return rules.Action{Type: rules.ActionDiscard, Card: pickWorstDiscard(hand, visible.Rules, canDiscardJoker)}
	}
	// 2) Draw phase: prefer discard if available and allowed this round, else deck.
	if visible.Phase == string(rules.PhaseDraw) {
		discardLocked := visible.DiscardDrawMinRound > 1 && visible.Round < visible.DiscardDrawMinRound
		if len(visible.DiscardPile) > 0 && !discardLocked {
			actor := visible.CurrentTurn
			if visible.RoundReqMet[actor] {
				// Already down: a discard pickup is unrestricted.
				return rules.Action{Type: rules.ActionDrawCard, DrawFrom: rules.DrawFromDiscard}
			}
			// Not yet down: picking up the discard obligates laying that
			// exact card into the initial meld this turn (server-enforced).
			// Only take it if a full plan using it actually exists —
			// otherwise take the deck instead, which carries no obligation.
			st := rulesStateForAI(visible, actor)
			topDiscard := visible.DiscardPile[len(visible.DiscardPile)-1]
			candidateHand := append(append([]string(nil), hand...), topDiscard)
			if _, ok := findInitialMeldPlanRequiring(st, actor, candidateHand, topDiscard); ok {
				return rules.Action{Type: rules.ActionDrawCard, DrawFrom: rules.DrawFromDiscard}
			}
		}
		return rules.Action{Type: rules.ActionDrawCard, DrawFrom: rules.DrawFromDeck}
	}
	// Fallback: discard.
	if len(hand) > 0 {
		canDiscardJoker := len(hand) == 1 && visible.RoundReqMet[visible.CurrentTurn]
		return rules.Action{Type: rules.ActionDiscard, Card: pickWorstDiscard(hand, visible.Rules, canDiscardJoker)}
	}
	return rules.Action{Type: rules.ActionDiscard, Card: ""}
}

func rulesStateForAI(visible VisibleState, playerID string) rules.GameState {
	return rules.GameState{
		Rules:              visible.Rules,
		GameNumber:         visible.GameNumber,
		RoundReqMet:        visible.RoundReqMet,
		Melds:              visible.Melds,
		MeldMeta:           visible.MeldMeta,
		InitialMeldMinimum: visible.InitialMeldMinimum,
	}
}

// combinations returns every k-length subset of items, as index tuples
// materialized into card slices. Used so the meld search works under any
// profile's MinSetSize/MinRunSize instead of hardcoding 3/4.
func combinations(items []string, k int) [][]string {
	n := len(items)
	if k <= 0 || k > n {
		return nil
	}
	var out [][]string
	idx := make([]int, k)
	for i := range idx {
		idx[i] = i
	}
	for {
		cand := make([]string, k)
		for i, v := range idx {
			cand[i] = items[v]
		}
		out = append(out, cand)

		i := k - 1
		for i >= 0 && idx[i] == n-k+i {
			i--
		}
		if i < 0 {
			break
		}
		idx[i]++
		for j := i + 1; j < k; j++ {
			idx[j] = idx[j-1] + 1
		}
	}
	return out
}

// findInitialMeldPlan looks for a full decomposition of the player's hand
// that completes their remaining round requirement (accounting for any
// sets/runs they've already laid, e.g. earlier this same turn) with total
// natural value at least the round's minimum. It returns the melds to lay,
// in order — the caller lays them one at a time (across successive
// ChooseAction calls), but only ever starts if a full plan already exists,
// so the server's "must finish what you start" rule never strands it.
func findInitialMeldPlan(state rules.GameState, playerID string, hand []string) ([][]string, bool) {
	return findInitialMeldPlanRequiring(state, playerID, hand, "")
}

// findInitialMeldPlanRequiring is findInitialMeldPlan, but when mustInclude
// is non-empty, only returns a plan that actually uses that specific card in
// one of its melds — used to check whether picking up a discard card (which
// obligates melding that exact card this turn) is actually going to work
// before committing to the draw.
func findInitialMeldPlanRequiring(state rules.GameState, playerID string, hand []string, mustInclude string) ([][]string, bool) {
	cfg := state.Rules
	req := cfg.ContractFor(state.GameNumber)
	setsBefore, runsBefore, hasCleanRun := rules.PlayerMeldCounts(state, playerID)
	needSets := req.Sets - setsBefore
	needRuns := req.Runs - runsBefore
	if needSets < 0 {
		needSets = 0
	}
	if needRuns < 0 {
		needRuns = 0
	}
	needCleanRun := req.RequireCleanRun && !hasCleanRun
	if needSets == 0 && needRuns == 0 && !needCleanRun {
		return nil, false
	}
	minValue := 0
	if state.InitialMeldMinimum > 0 {
		minValue = state.InitialMeldMinimum
	}
	alreadyValue := rules.PlayerInitialMeldNaturalValue(state, playerID)

	budget := &searchBudget{remaining: 200000}
	satisfied := mustInclude == ""
	combo, ok := searchMeldCombo(hand, cfg, needSets, needRuns, needCleanRun, alreadyValue, minValue, satisfied, mustInclude, budget)
	if !ok {
		return nil, false
	}
	if !cfg.IsFinalDeal(state.GameNumber) {
		used := 0
		for _, m := range combo {
			used += len(m)
		}
		if len(hand)-used < 1 {
			// Would meld away the entire hand with no card left to discard.
			return nil, false
		}
	}
	return combo, true
}

type searchBudget struct{ remaining int }

func containsCard(cards []string, card string) bool {
	for _, c := range cards {
		if c == card {
			return true
		}
	}
	return false
}

func searchMeldCombo(
	hand []string,
	cfg rules.RulesConfig,
	needSets, needRuns int,
	needCleanRun bool,
	valueSoFar, minValue int,
	satisfied bool,
	mustInclude string,
	budget *searchBudget,
) ([][]string, bool) {
	if needSets == 0 && needRuns == 0 && !needCleanRun {
		if valueSoFar >= minValue && satisfied {
			return [][]string{}, true
		}
		return nil, false
	}
	n := len(hand)
	minSet := cfg.MinSetSize
	if minSet == 0 {
		minSet = 3
	}
	minRun := cfg.MinRunSize
	if minRun == 0 {
		minRun = 4
	}
	if needSets > 0 && n >= minSet {
		for _, cand := range combinations(hand, minSet) {
			if budget.remaining <= 0 {
				return nil, false
			}
			budget.remaining--
			mv, err := rules.ValidateMeld(cand, cfg)
			if err != nil || mv.Type != rules.MeldSet {
				continue
			}
			rest := removeCardsOnce(hand, cand)
			candSatisfied := satisfied || containsCard(cand, mustInclude)
			if combo, ok := searchMeldCombo(rest, cfg, needSets-1, needRuns, needCleanRun, valueSoFar+mv.NaturalValue, minValue, candSatisfied, mustInclude, budget); ok {
				return append([][]string{cand}, combo...), true
			}
		}
	}
	if (needRuns > 0 || needCleanRun) && n >= minRun {
		for _, cand := range combinations(hand, minRun) {
			if budget.remaining <= 0 {
				return nil, false
			}
			budget.remaining--
			mv, err := rules.ValidateMeld(cand, cfg)
			if err != nil || mv.Type != rules.MeldRun {
				continue
			}
			rest := removeCardsOnce(hand, cand)
			candSatisfied := satisfied || containsCard(cand, mustInclude)
			nextNeedRuns := needRuns
			if nextNeedRuns > 0 {
				nextNeedRuns--
			}
			nextNeedCleanRun := needCleanRun && mv.WildCount > 0 // still need one if this run wasn't clean
			if combo, ok := searchMeldCombo(rest, cfg, needSets, nextNeedRuns, nextNeedCleanRun, valueSoFar+mv.NaturalValue, minValue, candSatisfied, mustInclude, budget); ok {
				return append([][]string{cand}, combo...), true
			}
		}
	}
	return nil, false
}

// removeCardsOnce removes exactly one occurrence of each card in remove from
// hand (order-preserving), matching removeAtIndices' semantics but keyed off
// a variable-length candidate slice instead of fixed index arguments.
func removeCardsOnce(hand []string, remove []string) []string {
	toRemove := append([]string(nil), remove...)
	out := make([]string, 0, len(hand)-len(remove))
	for _, c := range hand {
		removed := false
		for i, r := range toRemove {
			if r == c {
				toRemove = append(toRemove[:i], toRemove[i+1:]...)
				removed = true
				break
			}
		}
		if !removed {
			out = append(out, c)
		}
	}
	return out
}

// findLayOff looks for a card in hand that can extend any table meld (own
// or another player's). Skips a lay-off that would empty the hand on a deal
// that requires a final discard to go out (see RulesConfig.IsFinalDeal).
func findLayOff(meldMeta map[string][]rules.MeldInfo, melds map[string][][]string, hand []string, cfg rules.RulesConfig, gameNumber int) (meldID string, card string, ok bool) {
	if !cfg.IsFinalDeal(gameNumber) && len(hand) == 1 {
		return "", "", false
	}
	for owner, metas := range meldMeta {
		ownerMelds := melds[owner]
		for i, mi := range metas {
			if i >= len(ownerMelds) {
				continue
			}
			existing := ownerMelds[i]
			for _, c := range hand {
				cand := append(append([]string(nil), existing...), c)
				if _, err := rules.ValidateMeld(cand, cfg); err != nil {
					continue
				}
				// Never dirty the owner's only clean run with a wild — the
				// server rejects it, and the card belongs in its own meld.
				if rules.LayOffBreaksCleanRun(cfg, gameNumber, ownerMelds, i, c) {
					continue
				}
				return mi.MeldID, c, true
			}
		}
	}
	return "", "", false
}

func findAnyValidMeld(hand []string, cfg rules.RulesConfig) ([]string, bool) {
	minSet := cfg.MinSetSize
	if minSet == 0 {
		minSet = 3
	}
	minRun := cfg.MinRunSize
	if minRun == 0 {
		minRun = 4
	}
	n := len(hand)
	if n >= minSet {
		for _, cand := range combinations(hand, minSet) {
			if _, err := rules.ValidateMeld(cand, cfg); err == nil {
				return cand, true
			}
		}
	}
	if n >= minRun {
		for _, cand := range combinations(hand, minRun) {
			if _, err := rules.ValidateMeld(cand, cfg); err == nil {
				return cand, true
			}
		}
	}
	return nil, false
}

// PickWorstDiscard exposes pickWorstDiscard for callers that need an emergency
// fallback discard outside of ChooseAction (e.g. when a chosen action was rejected).
func PickWorstDiscard(hand []string, cfg rules.RulesConfig, canDiscardJoker bool) string {
	return pickWorstDiscard(hand, cfg, canDiscardJoker)
}

// pickWorstDiscard picks the highest-penalty card, but a joker is the
// highest-penalty card there is (rules.PenaltyPoints: 50) — under a profile
// with JokerDiscardRestricted, blindly picking it produces a discard the
// server always rejects, and the caller has no fallback, so the AI loop
// just retries the same illegal move until it gives up (made no progress).
// canDiscardJoker mirrors the server's own exception: true only when this
// would be the player's last card and they're already down.
func pickWorstDiscard(hand []string, cfg rules.RulesConfig, canDiscardJoker bool) string {
	if len(hand) == 0 {
		return ""
	}
	allowJoker := canDiscardJoker || !cfg.JokerDiscardRestricted
	worst := ""
	worstPts := -1
	for _, c := range hand {
		if !allowJoker && rules.IsJoker(c) {
			continue
		}
		pts := rules.PenaltyPoints(c, false)
		if pts > worstPts {
			worst = c
			worstPts = pts
		}
	}
	if worst == "" {
		// Every card is a forbidden joker (pathological, but has to return
		// something) — the caller/server is left to reject it.
		return hand[0]
	}
	return worst
}
