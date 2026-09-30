package api

import (
	"reflect"
	"testing"
)

// An offer that names its own combination is sent as named, and one that does
// not keeps the old reading — the first MinCards cards of the list.
func TestSubmissionForHonoursANamedCombination(t *testing.T) {
	named := ActionOffer{
		ID: "lay_meld:Q", Verb: "lay_meld", Enabled: true,
		Source: &Selector{
			Cards:    []string{"QC", "QD", "QH", "QS"},
			Submit:   []string{"QC", "QD", "QH", "QS"},
			MinCards: 3, MaxCards: 4,
		},
	}
	a, ok := SubmissionFor(named)
	if !ok {
		t.Fatal("an offer naming its combination describes no submission")
	}
	if want := []string{"QC", "QD", "QH", "QS"}; !reflect.DeepEqual(a.Cards, want) {
		t.Errorf("cards=%v, want %v — the minimum is what a player may lay, not what a press sends", a.Cards, want)
	}

	unnamed := ActionOffer{
		ID: "play_card", Verb: "play_card", Enabled: true,
		Source: &Selector{Cards: []string{"7H"}, MinCards: 1, MaxCards: 1},
	}
	a, ok = SubmissionFor(unnamed)
	if !ok {
		t.Fatal("a single-card offer describes no submission")
	}
	if want := []string{"7H"}; !reflect.DeepEqual(a.Cards, want) {
		t.Errorf("cards=%v, want %v", a.Cards, want)
	}
}

// A card an enabled offer turns down by name is found with its reason, so the
// terminal can say why rather than dropping it from the pick.
func TestRefusalForNamesTheRefusedCard(t *testing.T) {
	o := ActionOffer{ID: "discard", Verb: "discard", Enabled: true, Source: &Selector{
		Zone: "hand", Cards: []string{"4S", "3D"}, MinCards: 1, MaxCards: 1,
		Refused: []CardRefusal{{Card: "QH", WhyNot: "DISCARD_TAKEN_CARD_FORBIDDEN", RuleIDs: []string{"zolik.rules.pickup.noReturn"}}},
	}}
	r, ok := RefusalFor(o, []string{"QH"})
	if !ok || r.WhyNot != "DISCARD_TAKEN_CARD_FORBIDDEN" || len(r.RuleIDs) != 1 {
		t.Fatalf("RefusalFor(QH) = %+v, %v", r, ok)
	}
	if _, ok := RefusalFor(o, []string{"4S"}); ok {
		t.Fatal("a listed card reported as refused")
	}
}
