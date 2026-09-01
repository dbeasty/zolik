package ginrummy

import (
	"reflect"
	"sort"
	"testing"
)

func totalMeldedCards(melds []Meld) []string {
	var out []string
	for _, m := range melds {
		out = append(out, m.Cards...)
	}
	sort.Strings(out)
	return out
}

// TestDeadwood_NoMeldsAtAll is the trivial case: nothing can meld, so
// everything is deadwood.
func TestDeadwood_NoMeldsAtAll(t *testing.T) {
	hand := []string{"AH", "3D", "5C", "7S", "9H", "JD", "KC", "2S", "4H", "6D"}
	want := handValue(hand)
	dw, melds := Deadwood(hand)
	if dw != want {
		t.Errorf("Deadwood = %d, want %d (no melds possible)", dw, want)
	}
	if len(melds) != 0 {
		t.Errorf("found melds %v in a hand with none", melds)
	}
}

// TestDeadwood_OneSetAndOneRun is the ordinary case: a clean split with
// nothing left to fight over.
func TestDeadwood_OneSetAndOneRun(t *testing.T) {
	hand := []string{
		"5H", "5D", "5C", // a set
		"7S", "8S", "9S", // a run
		"AH", "TD", // deadwood: 1 + 10 = 11
	}
	dw, melds := Deadwood(hand)
	if dw != 11 {
		t.Fatalf("Deadwood = %d, want 11", dw)
	}
	if len(melds) != 2 {
		t.Fatalf("got %d melds, want 2: %+v", len(melds), melds)
	}
	got := totalMeldedCards(melds)
	want := []string{"5C", "5D", "5H", "7S", "8S", "9S"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("melded cards = %v, want %v", got, want)
	}
}

// TestDeadwood_OverlappingCandidatesPicksTheBetterCover is the case a greedy
// pick gets wrong: 5H can join either the set of 5s or the heart run, and only
// one of the two choices leaves nothing important stranded.
func TestDeadwood_OverlappingCandidatesPicksTheBetterCover(t *testing.T) {
	hand := []string{
		"5H", "5D", "5C", // a set, if 5H stays out of the run
		"3H", "4H", // needs 5H to become a run
		"9S", "TC", "JD", "QH", "KC", // isolated deadwood, fixed regardless
	}
	// Greedy taking the run first (3H-4H-5H) leaves 5D and 5C stranded (10
	// deadwood) plus the fixed cards. Packing the set instead leaves 3H+4H
	// stranded (7) plus the same fixed cards — strictly better, and only the
	// exhaustive search finds it because it has to try both.
	withSet := handValue([]string{"3H", "4H"})
	fixed := handValue([]string{"9S", "TC", "JD", "QH", "KC"})
	want := withSet + fixed

	dw, melds := Deadwood(hand)
	if dw != want {
		t.Fatalf("Deadwood = %d, want %d (should keep the set, not the run)", dw, want)
	}
	found := false
	for _, m := range melds {
		if m.Kind == "set" && reflect.DeepEqual(m.Cards, []string{"5C", "5D", "5H"}) {
			found = true
		}
	}
	if !found {
		t.Errorf("expected the 5s kept as a set, got %+v", melds)
	}
}

// TestDeadwood_Gin is a whole hand covered with nothing left over.
func TestDeadwood_Gin(t *testing.T) {
	hand := []string{
		"2H", "2D", "2C", "2S",
		"6H", "7H", "8H", "9H",
		"TD", "JD", "QD",
	}
	dw, melds := Deadwood(hand)
	if dw != 0 {
		t.Fatalf("Deadwood = %d, want 0 (gin)", dw)
	}
	if len(totalMeldedCards(melds)) != len(hand) {
		t.Errorf("melds cover %d cards, want all %d", len(totalMeldedCards(melds)), len(hand))
	}
}

// TestDeadwood_AceIsLowAndDoesNotWrap: a run of Q-K-A is not legal, so the
// three cards stay deadwood at their low-ace value.
func TestDeadwood_AceIsLowAndDoesNotWrap(t *testing.T) {
	hand := []string{"QH", "KH", "AH", "3D", "5C", "7S", "9D", "JC", "2H", "4S"}
	dw, melds := Deadwood(hand)
	if dw != handValue(hand) {
		t.Errorf("Deadwood = %d, want %d — Q-K-A must not meld", dw, handValue(hand))
	}
	for _, m := range melds {
		t.Errorf("unexpected meld %+v in a hand with no legal melds", m)
	}
}

func TestExtendsMeld_SetAcceptsMatchingRankUntilFour(t *testing.T) {
	m := Meld{ID: "m0", Kind: "set", Cards: []string{"5C", "5D", "5H"}}
	if !extendsMeld(m, "5S") {
		t.Error("a fourth 5 should extend the set")
	}
	if extendsMeld(m, "6S") {
		t.Error("a 6 should not extend a set of 5s")
	}
	full := Meld{ID: "m0", Kind: "set", Cards: []string{"5C", "5D", "5H", "5S"}}
	if extendsMeld(full, "5S") {
		t.Error("a set already holding all four of a rank should not accept a duplicate")
	}
}

func TestExtendsMeld_RunAcceptsEitherEndOnly(t *testing.T) {
	m := Meld{ID: "m0", Kind: "run", Cards: []string{"5H", "6H", "7H"}}
	if !extendsMeld(m, "4H") {
		t.Error("4H should extend the low end")
	}
	if !extendsMeld(m, "8H") {
		t.Error("8H should extend the high end")
	}
	if extendsMeld(m, "9H") {
		t.Error("9H should not extend past the end")
	}
	if extendsMeld(m, "6D") {
		t.Error("a card of the wrong suit should never extend a run")
	}
}

func TestExtendsMeld_RunNeverWrapsRoundTheCorner(t *testing.T) {
	m := Meld{ID: "m0", Kind: "run", Cards: []string{"JH", "QH", "KH"}}
	if extendsMeld(m, "AH") {
		t.Error("an ace should not wrap onto a King-high run")
	}
}

func TestInsertIntoMeld_RunStaysSortedFromEitherEnd(t *testing.T) {
	m := Meld{ID: "m0", Kind: "run", Cards: []string{"5H", "6H", "7H"}}
	got := insertIntoMeld(m, "4H")
	want := []string{"4H", "5H", "6H", "7H"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("insertIntoMeld(low) = %v, want %v", got, want)
	}
	got = insertIntoMeld(m, "8H")
	want = []string{"5H", "6H", "7H", "8H"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("insertIntoMeld(high) = %v, want %v", got, want)
	}
}
