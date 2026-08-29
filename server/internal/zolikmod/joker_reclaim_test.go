package zolikmod

import (
	"strings"
	"testing"

	"zolik/server/internal/module"
	"zolik/server/internal/rules"
)

// The take-and-replay rule end to end through the module: buy a joker back,
// and the discard control must refuse with the written rule, a remedy naming
// the joker, a working way out, a prompt, and a badge on the joker itself.
func TestReclaimedJokerRefusalCarriesEverythingAPlayerNeeds(t *testing.T) {
	m := &Module{}
	cfg := resolveConfig(module.MatchConfig{})
	if !cfg.JokerReclaimMustPlay {
		t.Fatal("setup: the default table should enforce take-and-replay")
	}

	gs := rules.GameState{
		Status:      rules.StatusActive,
		Rules:       cfg,
		GameNumber:  1,
		Round:       2,
		Phase:       rules.PhaseDraw,
		CurrentTurn: "p1",
		TurnOrder:   []string{"p1", "p2"},
		Hands:       map[string][]string{"p1": {"7C", "KS"}, "p2": {"2D", "2H"}},
		Melds: map[string][][]string{
			"p2": {{"5C", "6C", "JOKER1", "8C"}, {"9D", "9H", "9S"}},
		},
		MeldMeta: map[string][]rules.MeldInfo{"p2": {
			{MeldID: "m1", Type: rules.MeldRun, OwnerID: "p2", WildCount: 1},
			{MeldID: "m2", Type: rules.MeldSet, OwnerID: "p2"},
		}},
		RoundReqMet: map[string]bool{"p1": true, "p2": true},
		DrawPile:    []string{"3S", "4D"},
		DiscardPile: []string{"9C"},
		GameScores:  map[string][]int{},
		TotalScores: map[string]int{},
		NextMeldSeq: 2,
	}
	players := []module.PlayerRef{{ID: "p1", Name: "A"}, {ID: "p2", Name: "B"}}
	st, err := encode(&matchState{Rules: gs, Players: players})
	if err != nil {
		t.Fatal(err)
	}

	// Draw (arming the whole-turn undo), then buy the joker back.
	st, _, err = m.Apply(st, "p1", module.Action{Verb: "draw", OfferID: "draw:deck"})
	if err != nil {
		t.Fatalf("draw: %v", err)
	}
	st, _, err = m.Apply(st, "p1", module.Action{
		Verb: "swap_joker", OfferID: "swap_joker:m1", Target: "m1", Cards: []string{"7C"},
	})
	if err != nil {
		t.Fatalf("swap: %v", err)
	}

	offers, err := m.LegalActions(st, "p1")
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
		t.Fatal("expected the discard refused while the joker debt stands")
	}
	if discard.WhyNot != string(rules.ErrReclaimedJokerNotMelded) {
		t.Fatalf("whyNot = %q", discard.WhyNot)
	}
	sections, _ := m.Rules(module.MatchConfig{})
	stated := module.RuleIDsIn(sections)
	if len(discard.RuleIDs) == 0 {
		t.Fatal("no rule points at this refusal")
	}
	for _, id := range discard.RuleIDs {
		if !stated[id] {
			t.Errorf("ruleId %q is not stated by this table", id)
		}
	}
	if discard.Remedy == nil {
		t.Fatal("no remedy")
	}
	if discard.Remedy.Params["card"] != "JOKER1" {
		t.Errorf("the remedy does not name the joker that is owed: %v", discard.Remedy.Params)
	}
	// The way out on offer must actually be on the table — after a swap the
	// one-action undos are gone, so it is the whole-turn undo.
	if discard.RemedyOfferID != rules.OfferUndoTurn {
		t.Errorf("remedyOfferId = %q, want %q", discard.RemedyOfferID, rules.OfferUndoTurn)
	}
	enabled := false
	for _, o := range offers {
		if o.ID == discard.RemedyOfferID && o.Enabled {
			enabled = true
		}
	}
	if !enabled {
		t.Errorf("remedyOfferId %q is not an enabled offer", discard.RemedyOfferID)
	}

	// The obligation is said in a prompt and marked on the joker itself.
	vm, err := m.View(st, "p1")
	if err != nil {
		t.Fatal(err)
	}
	prompted := false
	for _, p := range vm.Prompts {
		if p.LabelKey == "prompt.jokerMustBePlayed" && p.Value == "JOKER1" {
			prompted = true
		}
	}
	if !prompted {
		t.Errorf("no prompt names the joker debt: %v", vm.Prompts)
	}
	badged := false
	for _, z := range vm.Zones {
		if !strings.HasPrefix(z.ID, "hand:p1") {
			continue
		}
		for _, c := range z.Cards {
			if c.Card == "JOKER1" && len(c.BadgeKeys) > 0 {
				badged = true
			}
		}
	}
	if !badged {
		t.Error("the reclaimed joker is not marked in the hand")
	}

	// An opponent's view carries neither the prompt nor a badged card.
	vm2, err := m.View(st, "p2")
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range vm2.Prompts {
		if p.LabelKey == "prompt.jokerMustBePlayed" {
			t.Error("the joker debt prompt leaked to an opponent")
		}
	}
}
