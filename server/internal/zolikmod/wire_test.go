package zolikmod

import (
	"encoding/json"
	"strings"
	"testing"

	"zolik/server/internal/module"
)

// The whole path, on a real deal: pick a card off the discard pile, then look
// at what the discard control says about itself.
func TestDiscardRefusalCarriesEverythingAPlayerNeeds(t *testing.T) {
	m := &Module{}
	players := []module.PlayerRef{{ID: "p1", Name: "A"}, {ID: "p2", Name: "B"}}
	st, err := m.NewMatch(module.MatchConfig{}, players, 42)
	if err != nil {
		t.Fatal(err)
	}

	s, _ := decode(st)
	turn := s.Rules.CurrentTurn

	// Put a known card on the discard pile and take it.
	st2, _, err := m.Apply(st, turn, module.Action{Verb: "draw", OfferID: "draw:discard"})
	if err != nil {
		t.Skipf("this seed's opening does not allow a discard-pile draw: %v", err)
	}

	offers, err := m.LegalActions(st2, turn)
	if err != nil {
		t.Fatal(err)
	}
	var discard *module.ActionOffer
	for i := range offers {
		if offers[i].ID == "discard" {
			discard = &offers[i]
		}
	}
	if discard == nil {
		t.Fatal("no discard offer")
	}
	if discard.Enabled {
		t.Fatal("expected the discard to be refused while a pickup is owed")
	}
	if discard.WhyNot != "DISCARD_CARD_NOT_MELDED" {
		t.Fatalf("whyNot = %q", discard.WhyNot)
	}
	if len(discard.RuleIDs) == 0 {
		t.Fatal("no rule points at this refusal")
	}
	// Every id must be a sentence this table actually states.
	sections, _ := m.Rules(module.MatchConfig{})
	stated := module.RuleIDsIn(sections)
	for _, id := range discard.RuleIDs {
		if !stated[id] {
			t.Errorf("ruleId %q is not stated by this table", id)
		}
	}
	if discard.Remedy == nil {
		t.Fatal("no remedy")
	}
	if discard.Remedy.Params["card"] == "" {
		t.Error("the remedy does not name the card that is owed")
	}
	// The remedy must name an offer that is actually on the table.
	found := false
	for _, o := range offers {
		if o.ID == discard.RemedyOfferID && o.Enabled {
			found = true
		}
	}
	if !found {
		t.Errorf("remedyOfferId %q is not an enabled offer", discard.RemedyOfferID)
	}

	// And the owed card is marked in the hand, not only described in a prompt.
	vm, err := m.View(st2, turn)
	if err != nil {
		t.Fatal(err)
	}
	badged := ""
	for _, z := range vm.Zones {
		if !strings.HasPrefix(z.ID, "hand:"+turn) {
			continue
		}
		for _, c := range z.Cards {
			if len(c.BadgeKeys) > 0 {
				badged = c.Card
			}
		}
	}
	if badged == "" {
		t.Error("the card the player owes their lay-down is not marked in their hand")
	}

	// It all has to survive the wire, since that is where a client reads it.
	raw, _ := json.Marshal(discard)
	for _, want := range []string{`"ruleIds"`, `"remedy"`, `"remedyOfferId"`} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("%s missing from the JSON: %s", want, raw)
		}
	}
}
