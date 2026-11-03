package zolikmod

import (
	"slices"
	"testing"

	"zolik/server/internal/module"
	"zolik/server/internal/rules"
)

// pickupPosition is p1 to draw under the classic rules, which open with a
// clean run and let a pickup start anywhere in the pile.
func pickupPosition(hand, pile []string, down bool) rules.GameState {
	return rules.GameState{
		Status:      rules.StatusActive,
		Rules:       resolveConfig(module.MatchConfig{}),
		GameNumber:  1,
		Round:       2,
		Phase:       rules.PhaseDraw,
		CurrentTurn: "p1",
		TurnOrder:   []string{"p1", "p2"},
		Hands:       map[string][]string{"p1": hand, "p2": {"2D", "2H"}},
		Melds:       map[string][][]string{},
		MeldMeta:    map[string][]rules.MeldInfo{},
		RoundReqMet: map[string]bool{"p1": down, "p2": false},
		DrawPile:    []string{"3S", "4D", "5C"},
		DiscardPile: pile,
		GameScores:  map[string][]int{},
		TotalScores: map[string]int{},
	}
}

func pickupOffer(t *testing.T, gs rules.GameState) module.ActionOffer {
	t.Helper()
	m := &Module{}
	players := []module.PlayerRef{{ID: "p1", Name: "A"}, {ID: "p2", Name: "B"}}
	st, err := encode(&matchState{Rules: gs, Players: players})
	if err != nil {
		t.Fatal(err)
	}
	offers, err := m.LegalActions(st, "p1")
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range offers {
		if o.ID == rules.OfferDrawDiscard {
			return o
		}
	}
	t.Fatal("no pickup offer")
	return module.ActionOffer{}
}

// A player who is not down and takes from the pile cannot discard until they
// are down. With no opening after the take, it is a dead end, so it is not on
// offer — and the refusal says why and points at the stock.
func TestPickupIsWithheldWhenNoOpeningFollows(t *testing.T) {
	o := pickupOffer(t, pickupPosition([]string{"4S", "3D", "9C"}, []string{"QH"}, false))
	if o.Enabled {
		t.Fatalf("pickup enabled with no opening after taking QH: cards %v", o.Source.Cards)
	}
	if o.WhyNot != string(errPickupUnplayable) {
		t.Errorf("whyNot = %q", o.WhyNot)
	}
	if !slices.Contains(o.RuleIDs, "zolik.rules.pickup.obligation") {
		t.Errorf("ruleIds = %v, want the pickup obligation", o.RuleIDs)
	}
	if o.RemedyOfferID != rules.OfferDrawDeck {
		t.Errorf("remedy offer = %q, want the stock", o.RemedyOfferID)
	}
}

func TestPickupIsOfferedWhenAnOpeningFollows(t *testing.T) {
	o := pickupOffer(t, pickupPosition([]string{"JH", "KH", "4S"}, []string{"QH"}, false))
	if !o.Enabled {
		t.Fatalf("pickup disabled (%s) though J-Q-K of hearts opens", o.WhyNot)
	}
	if !slices.Equal(o.Source.Cards, []string{"QH"}) {
		t.Errorf("cards = %v", o.Source.Cards)
	}
}

// Under any-from-pile each card is its own question: taking the queen (with
// the seven above it) gives an opening, taking the seven alone does not.
func TestPickupListsOnlyThePileCardsThatOpen(t *testing.T) {
	o := pickupOffer(t, pickupPosition([]string{"JH", "KH", "4S"}, []string{"QH", "7C"}, false))
	if !o.Enabled {
		t.Fatalf("pickup disabled (%s)", o.WhyNot)
	}
	if !slices.Equal(o.Source.Cards, []string{"QH"}) {
		t.Errorf("cards = %v, want QH alone", o.Source.Cards)
	}
	if len(o.Source.Refused) != 1 || o.Source.Refused[0].Card != "7C" ||
		o.Source.Refused[0].WhyNot != string(errPickupUnplayable) {
		t.Errorf("refused = %+v, want 7C", o.Source.Refused)
	}
}

// Once down a pickup owes nothing, so any card may be taken.
func TestPickupIsFreeOnceDown(t *testing.T) {
	o := pickupOffer(t, pickupPosition([]string{"4S", "3D", "9C"}, []string{"QH"}, true))
	if !o.Enabled {
		t.Fatalf("pickup disabled (%s) for a player already down", o.WhyNot)
	}
}

// A search that runs out of steps has not proved the opening impossible, and
// withholding a legal move on a guess is worse than leaving it open.
func TestPickupStaysOfferedWhenTheSearchGivesUp(t *testing.T) {
	gs := pickupPosition([]string{"4S", "3D", "9C"}, []string{"QH"}, false)
	offers := rules.LegalActions(gs, "p1")
	gatePickupWithin(gs, "p1", offers, 0)
	for _, o := range offers {
		if o.ID == rules.OfferDrawDiscard && !o.Enabled {
			t.Fatalf("pickup withheld (%s) on an exhausted search", o.WhyNot)
		}
	}
}

// The engine clears the debt with any opening, not only one holding the card
// taken, so a pickup that leaves an opening is offered even when the taken
// card has no place in it. Withholding it would refuse a move the engine takes.
func TestPickupIsOfferedWhenTheOpeningDoesNotUseTheCard(t *testing.T) {
	o := pickupOffer(t, pickupPosition([]string{"JH", "QH", "KH", "4S"}, []string{"2C"}, false))
	if !o.Enabled {
		t.Fatalf("pickup disabled (%s) though J-Q-K of hearts opens after it", o.WhyNot)
	}
}
