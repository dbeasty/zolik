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
	if !slices.Contains(o.RuleIDs, "zolik.rules.pickup.useful") {
		t.Errorf("ruleIds = %v, want the useful-pickup rule", o.RuleIDs)
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

// Once down, a card is offered only if it goes somewhere: QH with nothing on
// the table and nothing in hand to meld it with would just sit in the hand.
func TestPickupIsWithheldOnceDownWhenTheCardFitsNowhere(t *testing.T) {
	o := pickupOffer(t, pickupPosition([]string{"4S", "3D", "9C"}, []string{"QH"}, true))
	if o.Enabled {
		t.Fatalf("pickup enabled for a down player with nowhere to put QH")
	}
	if o.WhyNot != string(errPickupUnplayable) {
		t.Errorf("whyNot = %q", o.WhyNot)
	}
	if !slices.Contains(o.RuleIDs, "zolik.rules.pickup.useful") {
		t.Errorf("ruleIds = %v, want the useful-pickup rule", o.RuleIDs)
	}
	if o.RemedyOfferID != rules.OfferDrawDeck {
		t.Errorf("remedy offer = %q, want the stock", o.RemedyOfferID)
	}
}

func TestPickupIsOfferedOnceDownWhenItLaysOff(t *testing.T) {
	gs := pickupPosition([]string{"4S", "3D", "9C"}, []string{"QH"}, true)
	gs.Melds = map[string][][]string{"p2": {{"9H", "TH", "JH"}}}
	gs.MeldMeta = map[string][]rules.MeldInfo{"p2": {{MeldID: "m1", Type: rules.MeldRun}}}
	o := pickupOffer(t, gs)
	if !o.Enabled {
		t.Fatalf("pickup disabled (%s) though QH extends 9-10-J of hearts", o.WhyNot)
	}
}

func TestPickupIsOfferedOnceDownWhenItMakesANewMeld(t *testing.T) {
	o := pickupOffer(t, pickupPosition([]string{"QS", "QD", "9C"}, []string{"QH"}, true))
	if !o.Enabled {
		t.Fatalf("pickup disabled (%s) though QH makes a set of queens", o.WhyNot)
	}
}

// A new meld that empties the hand leaves nothing to discard, so it is no
// use on a deal that has to end on a discard.
func TestPickupIsWithheldOnceDownWhenTheMeldLeavesNoDiscard(t *testing.T) {
	o := pickupOffer(t, pickupPosition([]string{"QS", "QD"}, []string{"QH"}, true))
	if o.Enabled {
		t.Fatalf("pickup enabled though melding QH would leave nothing to discard")
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

// Before going down the taken card has to be in the opening, as the written
// rule says. A hand that opens on its own gains nothing from a card the
// opening leaves behind, so the take is not offered.
func TestPickupIsWithheldWhenTheOpeningDoesNotUseTheCard(t *testing.T) {
	o := pickupOffer(t, pickupPosition([]string{"JH", "QH", "KH", "4S"}, []string{"2C"}, false))
	if o.Enabled {
		t.Fatalf("pickup enabled though 2C has no place in the J-Q-K opening")
	}
}
