package pinochle

import (
	"slices"
	"sort"
	"testing"
)

func kinds(m Meld) []string {
	var out []string
	for _, it := range m.Items {
		out = append(out, it.Kind)
	}
	sort.Strings(out)
	return out
}

func TestMeldTable(t *testing.T) {
	single := tableOf(VarPartnership)
	double := tableOf(VarDoubleDeck)
	cases := []struct {
		name  string
		hand  []string
		trump byte
		t     table
		total int
		kinds []string
	}{
		{"a run is not also a royal marriage", []string{"AH", "TH", "KH", "QH", "JH", "9C"}, 'H', single, 150, []string{MeldRun}},
		{"a run and a second royal marriage", []string{"AH", "TH", "KH", "QH", "JH", "KH", "QH"}, 'H', single, 190, []string{MeldRoyalMarriage, MeldRun}},
		{"a run's spare king alone is nothing", []string{"AH", "TH", "KH", "KH", "QH", "JH"}, 'H', single, 150, []string{MeldRun}},
		{"a plain marriage outside trumps", []string{"KC", "QC", "9D"}, 'H', single, 20, []string{MeldMarriage}},
		{"a royal marriage in trumps", []string{"KS", "QS"}, 'S', single, 40, []string{MeldRoyalMarriage}},
		{"two dixes", []string{"9H", "9H", "9C"}, 'H', single, 20, []string{MeldDix, MeldDix}},
		{"no dix on the double pack", []string{"QH", "JH"}, 'H', double, 0, nil},
		{"pinochle", []string{"QS", "JD"}, 'H', single, 40, []string{MeldPinochle}},
		{"double pinochle is one meld", []string{"QS", "JD", "QS", "JD"}, 'H', single, 300, []string{MeldPinochle}},
		{"aces around", []string{"AH", "AC", "AD", "AS"}, 'H', single, 100, []string{MeldAces}},
		{"aces twice around", []string{"AH", "AC", "AD", "AS", "AH", "AC", "AD", "AS"}, 'C', single, 1000, []string{MeldAces}},
		{"three aces are nothing", []string{"AH", "AC", "AD"}, 'H', single, 0, nil},
		{
			// The queen of spades in three classes: queens around, a spade
			// marriage, and the pinochle.
			"one card across classes", []string{"QH", "QC", "QD", "QS", "KS", "JD"}, 'H', single,
			60 + 20 + 40, []string{MeldMarriage, MeldPinochle, MeldQueens},
		},
		{
			"a double run", []string{"AH", "TH", "KH", "QH", "JH", "AH", "TH", "KH", "QH", "JH"}, 'H', single,
			1500, []string{MeldDoubleRun},
		},
		{
			"triple pinochle on the double pack", []string{"QS", "JD", "QS", "JD", "QS", "JD"}, 'H', double,
			600, []string{MeldPinochle},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := BestMeld(c.hand, c.trump, c.t)
			if m.Total != c.total {
				t.Errorf("total %d, want %d (%v)", m.Total, c.total, m.Items)
			}
			if got := kinds(m); !slices.Equal(got, c.kinds) {
				t.Errorf("kinds %v, want %v", got, c.kinds)
			}
		})
	}
}

func TestMeldCardsCountsASharedCardOnce(t *testing.T) {
	m := BestMeld([]string{"QH", "QC", "QD", "QS", "KS", "JD"}, 'H', tableOf(VarPartnership))
	got := m.MeldCards()
	want := []string{"JD", "KS", "QC", "QD", "QH", "QS"}
	if !slices.Equal(got, want) {
		t.Errorf("meld cards %v, want %v", got, want)
	}
}

func TestTwoHandedMeldsReuseOnlyAcrossClasses(t *testing.T) {
	hand := []string{"KS", "QS", "JD", "QH", "QC", "QD", "9C"}
	used := Used{}
	ids := func() []string {
		var out []string
		for _, o := range twoHandOptions(hand, used, 'H') {
			out = append(out, o.ID)
		}
		sort.Strings(out)
		return out
	}
	if got := ids(); !slices.Equal(got, []string{"meld.marriage.S", "meld.pinochle", "meld.queens"}) {
		t.Fatalf("fresh hand offers %v", got)
	}
	// Lay the pinochle: the queen of spades may still go into a marriage
	// and into queens around, but not into another pinochle.
	for c, n := range countOf([]string{"QS", "JD"}) {
		used.of(classPinochle)[c] += n
	}
	if got := ids(); !slices.Equal(got, []string{"meld.marriage.S", "meld.queens"}) {
		t.Fatalf("after the pinochle: %v", got)
	}
	for c, n := range countOf([]string{"KS", "QS"}) {
		used.of(classSequence)[c] += n
	}
	if got := ids(); !slices.Equal(got, []string{"meld.queens"}) {
		t.Fatalf("after the marriage: %v", got)
	}
	// Playing the queen of spades takes it out of every meld it was in.
	hand = removeCards(hand, []string{"QS"})
	used.release(hand, "QS")
	if used.of(classPinochle)["QS"] != 0 || used.of(classSequence)["QS"] != 0 {
		t.Fatalf("a played card still counted as melded: %v", used)
	}
}

func TestHandTotals(t *testing.T) {
	for _, c := range []struct {
		v       string
		scoring int
		want    int
	}{
		{VarPartnership, ScoringCounters, 250},
		{VarPartnership, ScoringClassic, 250},
		{VarCutthroat, ScoringCounters, 250},
		{VarTwoHanded, ScoringClassic, 250},
		{VarDoubleDeck, ScoringCounters, 500},
		{VarDoubleDeck, ScoringClassic, 500},
	} {
		if got := tableOf(c.v).handTotal(c.scoring); got != c.want {
			t.Errorf("%s scoring %d: %d, want %d", c.v, c.scoring, got, c.want)
		}
	}
}

func TestPacks(t *testing.T) {
	for _, v := range allVariations {
		tb := tableOf(v)
		deck := tb.buildDeck()
		if want := tb.seats*tb.handSize + tb.widow; v != VarTwoHanded && len(deck) != want {
			t.Errorf("%s: %d cards for %d dealt", v, len(deck), want)
		}
		for c, n := range countOf(deck) {
			if n != tb.copies {
				t.Errorf("%s: %d of %s", v, n, c)
			}
		}
	}
}
