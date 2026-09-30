package zolikmod

import (
	"testing"

	"zolik/server/internal/module"
	"zolik/server/internal/rules"
)

// A down player who takes the top of the pile may not hand it straight back,
// and says so through the module end to end: the discard stays enabled, the
// taken card is refused by name with the written rule and a remedy, and the
// card itself is marked in hand.
//
// The taken card is the first in hand on purpose: that is the order the
// engine once probed in and read the card's refusal as the verb's, leaving a
// player with two legal discards shown none.
func TestTakenCardIsRefusedByNameWhileTheDiscardStaysOpen(t *testing.T) {
	m := &Module{}
	cfg := resolveConfig(module.MatchConfig{})
	gs := rules.GameState{
		Status:      rules.StatusActive,
		Rules:       cfg,
		GameNumber:  1,
		Round:       2,
		Phase:       rules.PhaseDraw,
		CurrentTurn: "p1",
		TurnOrder:   []string{"p1", "p2"},
		Hands:       map[string][]string{"p1": {"4S", "3D"}, "p2": {"2D", "2H"}},
		Melds:       map[string][][]string{},
		MeldMeta:    map[string][]rules.MeldInfo{},
		RoundReqMet: map[string]bool{"p1": true, "p2": true},
		DrawPile:    []string{"3S", "4D"},
		DiscardPile: []string{"QH"},
		GameScores:  map[string][]int{},
		TotalScores: map[string]int{},
	}
	players := []module.PlayerRef{{ID: "p1", Name: "A"}, {ID: "p2", Name: "B"}}
	st, err := encode(&matchState{Rules: gs, Players: players})
	if err != nil {
		t.Fatal(err)
	}
	st, _, err = m.Apply(st, "p1", module.Action{Verb: "draw", OfferID: rules.OfferDrawDiscard})
	if err != nil {
		t.Fatalf("take from the pile: %v", err)
	}
	s, _ := decode(st)
	s.Rules.Hands["p1"] = []string{"QH", "4S", "3D"}
	if st, err = encode(s); err != nil {
		t.Fatal(err)
	}

	offers, err := m.LegalActions(st, "p1")
	if err != nil {
		t.Fatal(err)
	}
	var discard *module.ActionOffer
	for i := range offers {
		if offers[i].ID == rules.OfferDiscard {
			discard = &offers[i]
		}
	}
	if discard == nil {
		t.Fatal("no discard offer")
	}
	if !discard.Enabled {
		t.Fatalf("discard disabled (%s) with 4S and 3D both legal", discard.WhyNot)
	}
	for _, c := range discard.Source.Cards {
		if c == "QH" {
			t.Fatalf("taken card listed as discardable: %v", discard.Source.Cards)
		}
	}
	if len(discard.Source.Refused) != 1 || discard.Source.Refused[0].Card != "QH" {
		t.Fatalf("refused = %+v, want QH alone", discard.Source.Refused)
	}
	r := discard.Source.Refused[0]
	if r.WhyNot != string(rules.ErrDiscardTakenCard) {
		t.Errorf("whyNot = %q", r.WhyNot)
	}
	sections, _ := m.Rules(module.MatchConfig{})
	stated := module.RuleIDsIn(sections)
	if len(r.RuleIDs) == 0 {
		t.Fatal("no rule points at the refused card")
	}
	for _, id := range r.RuleIDs {
		if !stated[id] {
			t.Errorf("ruleId %q is not stated by this table", id)
		}
	}
	if r.Remedy == nil || r.Remedy.Params["card"] != "QH" {
		t.Errorf("remedy does not name the taken card: %+v", r.Remedy)
	}

	vm, err := m.View(st, "p1")
	if err != nil {
		t.Fatal(err)
	}
	marked := 0
	for _, z := range vm.Zones {
		if z.ID != handZoneID("p1") {
			continue
		}
		for _, c := range z.Cards {
			for _, k := range c.BadgeKeys {
				if k != "zolik.badge.noReturn" {
					continue
				}
				if c.Card != "QH" {
					t.Errorf("%s marked as held back", c.Card)
				}
				marked++
			}
		}
	}
	if marked != 1 {
		t.Errorf("taken card marked %d times, want once", marked)
	}
}
