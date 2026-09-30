package ai

import (
	"testing"

	"zolik/server/internal/module"
	"zolik/server/internal/rules"
)

// What a joker is worth, and when it stops being worth it.
//
// A joker carries fifty penalty points — more than any other card in the deck
// — and it is also the card that finishes whichever meld the hand turns out to
// need. Every rule in this file is a consequence of those two facts pointing in
// opposite directions, and every bug it guards against was the agent acting on
// only the first of them.

// --- the discard -------------------------------------------------------------

// TestNoStrengthDiscardsALooseJoker.
//
// keepValue has said since it was written that a joker is never shed as though
// it were a loose card, and the check that said so sat *below* the early return
// for the KeepFinished policy — which is the policy Easy and Medium use, which
// is to say nearly every bot anybody plays against. So the two weakest profiles
// scored a joker at zero, and then the ordering put it first, because fifty
// points is the most expensive card in the hand and shedding the expensive card
// is the rule.
//
// It never fired at a shipped table only because both shipped rulesets forbid
// the discard outright (JokerDiscardRestricted). Turn that off — which is what
// a ruleset knob is for — and the agent threw its wild card away on the turn it
// drew it.
func TestNoStrengthDiscardsALooseJoker(t *testing.T) {
	cfg := rules.ProfileZolikClassic
	cfg.JokerDiscardRestricted = false

	for _, skill := range module.Skills {
		gs := rules.GameState{
			Status:      rules.StatusActive,
			Rules:       cfg,
			GameNumber:  1,
			Round:       2,
			Phase:       rules.PhaseMeld,
			CurrentTurn: "ai",
			TurnOrder:   []string{"ai", "p2"},
			Hands: map[string][]string{
				"ai": {"JOKER1", "KS", "QD", "7C", "3H", "5S"},
				"p2": {"2D", "4H"},
			},
			RoundReqMet: map[string]bool{"ai": false, "p2": false},
			DiscardPile: []string{"9C"},
		}
		// No blunder dice: this is about what the agent thinks is best, not
		// about how often it does the second-best thing on purpose.
		agent := NewAgentWithProfile(noFallibility(ProfileFor(skill)), 1)
		action := agent.ChooseAction(visibleFrom(gs), gs.Hands["ai"])
		if action.Type == rules.ActionDiscard && rules.IsJoker(action.Card) {
			t.Errorf("%s discarded its joker with five ordinary cards in hand", skill)
		}
	}
}

// noFallibility is a profile with the dice turned off, so a test can assert
// what a strength *chooses* rather than how often it slips.
func noFallibility(p Profile) Profile {
	p.BlunderRate, p.MissRate, p.MeldDither = 0, 0, 0
	return p
}

// --- the lay-off -------------------------------------------------------------

// layOffTable seats the agent, already down, against a table holding one run
// of clubs that both a natural and a joker would extend.
func layOffTable(hand []string) rules.GameState {
	return rules.GameState{
		Status:      rules.StatusActive,
		Rules:       rules.ProfileZolikClassic,
		GameNumber:  1,
		Round:       3,
		Phase:       rules.PhaseMeld,
		CurrentTurn: "ai",
		TurnOrder:   []string{"ai", "p2"},
		Hands:       map[string][]string{"ai": hand, "p2": {"2D", "4H", "6S", "8D"}},
		Melds:       map[string][][]string{"ai": {{"5C", "6C", "7C"}}},
		MeldMeta: map[string][]rules.MeldInfo{"ai": {
			{MeldID: "m1", Type: rules.MeldRun, OwnerID: "ai"},
		}},
		RoundReqMet: map[string]bool{"ai": true, "p2": false},
		DiscardPile: []string{"9D"},
	}
}

// TestLayOffSpendsTheNaturalBeforeTheJoker.
//
// The four of clubs and the joker both extend the run, and the joker is fifty
// penalty points against the four's four — so "shed the most expensive card
// that fits", which is what LayOffHighestPoints means, named the joker every
// single time. It fits nearly everywhere, which is what a joker is for, so the
// hand's wild card went onto the table at the first opportunity it got and was
// not there for the meld that had no other way of being finished.
func TestLayOffSpendsTheNaturalBeforeTheJoker(t *testing.T) {
	for _, skill := range module.Skills {
		gs := layOffTable([]string{"JOKER1", "4C", "KS", "QD"})
		agent := NewAgentWithProfile(noFallibility(ProfileFor(skill)), 1)
		action := agent.ChooseAction(visibleFrom(gs), gs.Hands["ai"])
		if action.Type == rules.ActionLayOff && rules.IsJoker(action.Card) {
			t.Errorf("%s laid its joker off onto a run the four of clubs also fits", skill)
		}
		if _, err := rules.ApplyAction(gs, "ai", action); err != nil {
			t.Errorf("%s: the engine refused %+v: %v", skill, action, err)
		}
	}
}

// TestLayOffSpendsTheJokerInAWildCrunch is the end of that rule, and the whole
// reason it needs one.
//
// A joker may be discarded only as the card that empties an already-down
// player's hand, so a hand of two jokers has no legal move in it at all: it
// cannot meld, it cannot discard, and the deal wedges for the whole table. A
// hand holding one natural card and one joker is one draw away from being that
// hand, and keeping the joker is what takes it there.
//
// So the wild-last rule reverses at the last natural card. The joker goes onto
// the table while there is still a table to put it on.
func TestLayOffSpendsTheJokerInAWildCrunch(t *testing.T) {
	for _, skill := range module.Skills {
		gs := layOffTable([]string{"JOKER1", "KS"})
		agent := NewAgentWithProfile(noFallibility(ProfileFor(skill)), 1)
		action := agent.ChooseAction(visibleFrom(gs), gs.Hands["ai"])
		if action.Type != rules.ActionLayOff || !rules.IsJoker(action.Card) {
			t.Errorf("%s played %+v holding one joker and one natural card, want the joker onto the run", skill, action)
		}
	}
}

// TestTheAgentNeverDiscardsItselfIntoADeadHand.
//
// The other half of the crunch: given a choice, the card that goes is never the
// one that leaves nothing but jokers behind. Here the king and the joker are
// both unmelded, the king is ten points and the joker is fifty — and throwing
// the king is a hand of one joker, which is a legal position and a bad one.
func TestTheAgentNeverDiscardsItselfIntoADeadHand(t *testing.T) {
	gs := rules.GameState{
		Status:      rules.StatusActive,
		Rules:       rules.ProfileZolikClassic,
		GameNumber:  1,
		Round:       3,
		Phase:       rules.PhaseMeld,
		CurrentTurn: "ai",
		TurnOrder:   []string{"ai", "p2"},
		Hands:       map[string][]string{"ai": {"JOKER1", "KS", "3D"}, "p2": {"2D", "4H"}},
		RoundReqMet: map[string]bool{"ai": false, "p2": false},
		DiscardPile: []string{"9C"},
	}
	agent := NewAgentWithProfile(noFallibility(ProfileFor(module.SkillHard)), 1)
	action := agent.ChooseAction(visibleFrom(gs), gs.Hands["ai"])
	if action.Type != rules.ActionDiscard {
		t.Fatalf("played %+v, want a discard", action)
	}
	rest := removeCardsOnce(gs.Hands["ai"], []string{action.Card})
	if !handCanStillDiscard(rest, gs.Rules, false) {
		t.Errorf("discarded %s, leaving %v — a hand with no legal move in it", action.Card, rest)
	}
}

// --- building the meld -------------------------------------------------------

// TestTheMeldSearchSpendsNoJokerItDoesNotNeed.
//
// Three natural kings and a joker, against a contract that wants one set. Both
// {K,K,K} and {K,K,JOKER} are legal sets and the old search returned whichever
// combinations() produced first out of hand order — so the joker, which
// validates beside almost anything, turned up in the first plan the search
// stumbled over and was gone.
func TestTheMeldSearchSpendsNoJokerItDoesNotNeed(t *testing.T) {
	cfg := continentalNoFloor()
	hand := []string{"JOKER1", "KS", "KD", "KH", "4C", "7D", "9S"}
	meld, ok := findAnyValidMeld(hand, cfg)
	if !ok {
		t.Fatalf("found no meld in %v", hand)
	}
	if containsWild(meld) {
		t.Errorf("laid %v, spending a joker on a set three natural kings already make", meld)
	}
}

// And the plan search, which is the one that actually gets used when a contract
// or a point floor is in play.
func TestTheContractPlanSpendsNoJokerItDoesNotNeed(t *testing.T) {
	// A point floor and no per-type quota, so the search is topping a total up
	// rather than filling a shape — which is the case a king set answers and a
	// joker is not needed for.
	cfg := rules.ProfileZolikClassic
	cfg.StaticContract = rules.ContractRequirement{}
	cfg.InitialMeldMinimum = 30
	st := rules.GameState{
		Rules:       cfg,
		GameNumber:  1,
		RoundReqMet: map[string]bool{"ai": false},
		Melds:       map[string][][]string{},
		MeldMeta:    map[string][]rules.MeldInfo{},
	}
	hand := []string{"JOKER1", "KS", "KD", "KH", "4C", "7D", "9S", "2H", "3C"}
	plan, ok := findInitialMeldPlan(st, "ai", hand)
	if !ok {
		t.Fatalf("found no plan in %v", hand)
	}
	for _, m := range plan {
		if containsWild(m) {
			t.Errorf("planned %v, spending a joker a natural set makes unnecessary", m)
		}
	}
}
