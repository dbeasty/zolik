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
	rnd         *rand.Rand
}

func NewHeuristicAgent(difficulty string) *HeuristicAgent {
	return &HeuristicAgent{
		difficulty: difficulty,
		rnd:         rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (a *HeuristicAgent) Difficulty() string { return a.difficulty }

func (a *HeuristicAgent) ChooseAction(visible VisibleState, hand []string) rules.Action {
	// Priority order (simplified v1):
	// 1) Offer: accept only if it helps us lay any valid meld next.
	if visible.Phase == string(rules.PhaseOffer) && visible.Offer != nil {
		return a.chooseOfferAction(visible, hand)
	}
	// 2) Meld phase: try to lay any valid meld if we haven't met round requirement.
	if visible.Phase == string(rules.PhaseMeld) {
		actor := visible.CurrentTurn
		if !visible.RoundReqMet[actor] {
			st := rulesStateForAI(visible, actor)
			if meld, ok := findContributingMeld(st, actor, hand); ok {
				return rules.Action{Type: rules.ActionLayMeld, Cards: meld}
			}
		} else if meld, ok := findAnyValidMeld(hand); ok && len(hand) > len(meld) {
			return rules.Action{Type: rules.ActionLayMeld, Cards: meld}
		}
		// Otherwise discard.
		return rules.Action{Type: rules.ActionDiscard, Card: pickWorstDiscard(hand)}
	}
	// 3) Draw phase: prefer discard if available, else deck.
	if visible.Phase == string(rules.PhaseDraw) {
		if len(visible.DiscardPile) > 0 {
			return rules.Action{Type: rules.ActionDrawCard, DrawFrom: rules.DrawFromDiscard}
		}
		return rules.Action{Type: rules.ActionDrawCard, DrawFrom: rules.DrawFromDeck}
	}
	// Fallback: discard.
	if len(hand) > 0 {
		return rules.Action{Type: rules.ActionDiscard, Card: pickWorstDiscard(hand)}
	}
	return rules.Action{Type: rules.ActionDiscard, Card: ""}
}

func (a *HeuristicAgent) chooseOfferAction(visible VisibleState, hand []string) rules.Action {
	// Accept if adding offered card + one random penalty card could enable a meld.
	// v1 simplification: accept if we can lay a meld with current hand + offered card.
	candidate := append([]string(nil), hand...)
	candidate = append(candidate, visible.Offer.Card)
	if len(candidate) >= 3 {
		if _, ok := findAnyValidMeld(candidate); ok {
			// easy makes intentional suboptimal choices sometimes.
			if a.difficulty == "easy" && a.rnd.Intn(100) < 30 {
				return rules.Action{Type: rules.ActionDeclineOffer}
			}
			return rules.Action{Type: rules.ActionAcceptOffer}
		}
	}
	return rules.Action{Type: rules.ActionDeclineOffer}
}

func rulesStateForAI(visible VisibleState, playerID string) rules.GameState {
	return rules.GameState{
		Round:       visible.Round,
		RoundReqMet: visible.RoundReqMet,
		Melds:       visible.Melds,
		MeldMeta:    visible.MeldMeta,
	}
}

func findContributingMeld(state rules.GameState, playerID string, hand []string) ([]string, bool) {
	n := len(hand)
	if n < 3 {
		return nil, false
	}
	try := func(cand []string) ([]string, bool) {
		mv, err := rules.ValidateMeld(cand)
		if err != nil {
			return nil, false
		}
		if !rules.MeldContributesTowardRequirement(state, playerID, mv.Type, len(cand)) {
			return nil, false
		}
		if state.Round < 7 && len(hand) == len(cand) {
			return nil, false
		}
		return cand, true
	}
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			for k := j + 1; k < n; k++ {
				if cand, ok := try([]string{hand[i], hand[j], hand[k]}); ok {
					return cand, true
				}
			}
		}
	}
	if n < 4 {
		return nil, false
	}
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			for k := j + 1; k < n; k++ {
				for l := k + 1; l < n; l++ {
					if cand, ok := try([]string{hand[i], hand[j], hand[k], hand[l]}); ok {
						return cand, true
					}
				}
			}
		}
	}
	return nil, false
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

