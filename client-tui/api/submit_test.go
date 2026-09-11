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
