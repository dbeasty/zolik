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
			if meldID, card, ok := findLayOff(visible.MeldMeta, visible.Melds, hand, visible.GameNumber); ok {
				return rules.Action{Type: rules.ActionLayOff, MeldID: meldID, Card: card}
			}
			if meld, ok := findAnyValidMeld(hand); ok && len(hand) > len(meld) {
				return rules.Action{Type: rules.ActionLayMeld, Cards: meld}
			}
		}
		// Otherwise discard.
		return rules.Action{Type: rules.ActionDiscard, Card: pickWorstDiscard(hand)}
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
		return rules.Action{Type: rules.ActionDiscard, Card: pickWorstDiscard(hand)}
	}
	return rules.Action{Type: rules.ActionDiscard, Card: ""}
}

func rulesStateForAI(visible VisibleState, playerID string) rules.GameState {
	return rules.GameState{
		GameNumber:         visible.GameNumber,
		RoundReqMet:        visible.RoundReqMet,
		Melds:              visible.Melds,
		MeldMeta:           visible.MeldMeta,
		InitialMeldMinimum: visible.InitialMeldMinimum,
	}
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
	req := rules.RoundRequirementFor(state.GameNumber)
	setsBefore, runsBefore := rules.PlayerMeldCounts(state, playerID)
	needSets := req.Sets - setsBefore
	needRuns := req.Runs - runsBefore
	if needSets < 0 {
		needSets = 0
	}
	if needRuns < 0 {
		needRuns = 0
	}
	if needSets == 0 && needRuns == 0 {
		return nil, false
	}
	minValue := 0
	if state.InitialMeldMinimum > 0 {
		minValue = state.InitialMeldMinimum
	}
	alreadyValue := rules.PlayerInitialMeldNaturalValue(state, playerID)

	budget := &searchBudget{remaining: 200000}
	satisfied := mustInclude == ""
	combo, ok := searchMeldCombo(hand, needSets, needRuns, alreadyValue, minValue, satisfied, mustInclude, budget)
	if !ok {
		return nil, false
	}
	if state.GameNumber < 7 {
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
	needSets, needRuns, valueSoFar, minValue int,
	satisfied bool,
	mustInclude string,
	budget *searchBudget,
) ([][]string, bool) {
	if needSets == 0 && needRuns == 0 {
		if valueSoFar >= minValue && satisfied {
			return [][]string{}, true
		}
		return nil, false
	}
	n := len(hand)
	if needSets > 0 && n >= 3 {
		for i := 0; i < n; i++ {
			for j := i + 1; j < n; j++ {
				for k := j + 1; k < n; k++ {
					if budget.remaining <= 0 {
						return nil, false
					}
					budget.remaining--
					cand := []string{hand[i], hand[j], hand[k]}
					mv, err := rules.ValidateMeld(cand)
					if err != nil || mv.Type != rules.MeldSet {
						continue
					}
					rest := removeAtIndices(hand, i, j, k)
					candSatisfied := satisfied || containsCard(cand, mustInclude)
					if combo, ok := searchMeldCombo(rest, needSets-1, needRuns, valueSoFar+mv.NaturalValue, minValue, candSatisfied, mustInclude, budget); ok {
						return append([][]string{cand}, combo...), true
					}
				}
			}
		}
	}
	if needRuns > 0 && n >= 4 {
		for i := 0; i < n; i++ {
			for j := i + 1; j < n; j++ {
				for k := j + 1; k < n; k++ {
					for l := k + 1; l < n; l++ {
						if budget.remaining <= 0 {
							return nil, false
						}
						budget.remaining--
						cand := []string{hand[i], hand[j], hand[k], hand[l]}
						mv, err := rules.ValidateMeld(cand)
						if err != nil || mv.Type != rules.MeldRun {
							continue
						}
						rest := removeAtIndices(hand, i, j, k, l)
						candSatisfied := satisfied || containsCard(cand, mustInclude)
						if combo, ok := searchMeldCombo(rest, needSets, needRuns-1, valueSoFar+mv.NaturalValue, minValue, candSatisfied, mustInclude, budget); ok {
							return append([][]string{cand}, combo...), true
						}
					}
				}
			}
		}
	}
	return nil, false
}

func removeAtIndices(hand []string, idx ...int) []string {
	skip := make(map[int]bool, len(idx))
	for _, i := range idx {
		skip[i] = true
	}
	out := make([]string, 0, len(hand)-len(idx))
	for i, c := range hand {
		if skip[i] {
			continue
		}
		out = append(out, c)
	}
	return out
}

// findLayOff looks for a card in hand that can extend any table meld (own
// or another player's). Skips a lay-off that would empty the hand before
// game 7, since the server requires going out via discard until then.
func findLayOff(meldMeta map[string][]rules.MeldInfo, melds map[string][][]string, hand []string, gameNumber int) (meldID string, card string, ok bool) {
	if gameNumber < 7 && len(hand) == 1 {
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
				if _, err := rules.ValidateMeld(cand); err == nil {
					return mi.MeldID, c, true
				}
			}
		}
	}
	return "", "", false
}

func findAnyValidMeld(hand []string) ([]string, bool) {
	// Brute-force subsets of size 3 (set) and 4 (run) to find the first valid meld.
	n := len(hand)
	if n < 3 {
		return nil, false
	}
	// size 3
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			for k := j + 1; k < n; k++ {
				cand := []string{hand[i], hand[j], hand[k]}
				if _, err := rules.ValidateMeld(cand); err == nil {
					return cand, true
				}
			}
		}
	}
	// size 4
	if n < 4 {
		return nil, false
	}
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			for k := j + 1; k < n; k++ {
				for l := k + 1; l < n; l++ {
					cand := []string{hand[i], hand[j], hand[k], hand[l]}
					if _, err := rules.ValidateMeld(cand); err == nil {
						return cand, true
					}
				}
			}
		}
	}
	return nil, false
}

// PickWorstDiscard exposes pickWorstDiscard for callers that need an emergency
// fallback discard outside of ChooseAction (e.g. when a chosen action was rejected).
func PickWorstDiscard(hand []string) string {
	return pickWorstDiscard(hand)
}

func pickWorstDiscard(hand []string) string {
	if len(hand) == 0 {
		return ""
	}
	worst := hand[0]
	worstPts := rules.PenaltyPoints(worst, false)
	for _, c := range hand[1:] {
		pts := rules.PenaltyPoints(c, false)
		if pts > worstPts {
			worst = c
			worstPts = pts
		}
	}
	return worst
}
