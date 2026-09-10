package canasta

import (
	"strings"
	"testing"

	"zolik/server/internal/module"
)

// sequenceRules is a ruleset with runs turned on, for the tests that are about
// the sequence rules themselves rather than about Samba.
//
// Built from classic rather than from samba so these keep testing what they say
// they test: if Samba later changes its wild limits or its deck, a run is still
// three or more consecutive naturals of one suit and these cases still pin that.
func sequenceRules() ruleset {
	r := variations["classic"]
	r.Sequences = true
	return r
}

func TestValidateRun(t *testing.T) {
	cases := []struct {
		name  string
		cards []string
		want  string
	}{
		{"three in suit", []string{"4H", "5H", "6H"}, ""},
		{"seven is a samba", []string{"4H", "5H", "6H", "7H", "8H", "9H", "TH"}, ""},
		{"out of order is still a run", []string{"6H", "4H", "5H"}, ""},
		{"up to the ace", []string{"QS", "KS", "AS"}, ""},

		{"two is not a run", []string{"4H", "5H"}, ErrMeldTooSmall},
		{"eight cards is not a longer samba", []string{"4H", "5H", "6H", "7H", "8H", "9H", "TH", "JH"}, ErrMeldTooLarge},
		{"mixed suits", []string{"4H", "5D", "6H"}, ErrSequenceNeedsOneSuit},
		{"a gap", []string{"4H", "5H", "7H"}, ErrRunNotConsecutive},
		{"a duplicate rank is not a step", []string{"4H", "5H", "5H"}, ErrRunNotConsecutive},
		{"no joker may stand in", []string{"4H", "JOKER1", "6H"}, ErrSequenceNoWilds},
		{"and no two either, wild being the point", []string{"4H", "2H", "6H"}, ErrSequenceNoWilds},
		{"threes are never in a sequence", []string{"3S", "4S", "5S"}, ErrCannotMeldThree},
		// The ace is high and only high, so this is a gap and not a wrap: were
		// it allowed, A-2-3 would be a sequence in a game where the 2 is wild.
		{"no round the corner", []string{"KH", "AH", "4H"}, ErrRunNotConsecutive},
	}

	r := sequenceRules()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateRun(r, tc.cards)
			got := ""
			if err != nil {
				got = module.CodeOf(err)
			}
			if got != tc.want {
				t.Errorf("validateRun(%v) = %q, want %q", tc.cards, got, tc.want)
			}
		})
	}
}

// A variation without sequences must read a mixed list the way it always has:
// as a group whose ranks do not match, which is the refusal Canasta gives today.
func TestARunIsNotAMeldWhereThereAreNoSequences(t *testing.T) {
	if err := validateMeld(variations["classic"], []string{"4H", "5H", "6H"}); module.CodeOf(err) != ErrMeldMixedRanks {
		t.Errorf("classic read 4-5-6 of hearts as %v, want %s", err, ErrMeldMixedRanks)
	}
	if err := validateMeld(sequenceRules(), []string{"4H", "5H", "6H"}); err != nil {
		t.Errorf("a sequence variation refused 4-5-6 of hearts: %v", err)
	}
}

// The dispatch is on the cards, not on a flag a caller passes, so this is the
// test that a group is still read as a group once runs exist.
func TestMeldKindIsReadFromTheCards(t *testing.T) {
	r := sequenceRules()
	cases := []struct {
		cards []string
		want  string
	}{
		{[]string{"KH", "KD", "KS"}, meldSet},
		{[]string{"KH", "KD", "JOKER1"}, meldSet},
		{[]string{"4H", "5H", "6H"}, meldRun},
		{[]string{"4H", "JOKER1", "6H"}, meldRun}, // asks to be a run, and is refused as one
	}
	for _, tc := range cases {
		if got := meldKindOf(r, tc.cards); got != tc.want {
			t.Errorf("meldKindOf(%v) = %q, want %q", tc.cards, got, tc.want)
		}
	}
}

// runCandidates is what the offer list ships, so what matters is that it is
// concrete, complete and never proposes a card the hand does not hold.
func TestRunCandidates(t *testing.T) {
	r := sequenceRules()

	t.Run("finds the maximal block in each suit", func(t *testing.T) {
		hand := []string{"4H", "5H", "6H", "7H", "9S", "TS", "JS", "KD"}
		got := runCandidates(r, hand, &Team{})
		if len(got) != 2 {
			t.Fatalf("got %d candidates, want 2: %v", len(got), got)
		}
		want := map[string]bool{"4H 5H 6H 7H": true, "9S TS JS": true}
		for _, c := range got {
			if !want[strings.Join(c.Cards, " ")] {
				t.Errorf("unexpected candidate %v", c.Cards)
			}
		}
	})

	t.Run("caps at seven, keeping the top", func(t *testing.T) {
		hand := []string{"4H", "5H", "6H", "7H", "8H", "9H", "TH", "JH", "QH"}
		got := runCandidates(r, hand, &Team{})
		if len(got) != 1 {
			t.Fatalf("got %d candidates, want 1", len(got))
		}
		if len(got[0].Cards) != canastaSize {
			t.Fatalf("candidate is %d cards, want %d", len(got[0].Cards), canastaSize)
		}
		if got[0].Cards[len(got[0].Cards)-1] != "QH" {
			t.Errorf("candidate is %v — a capped block should keep the high cards", got[0].Cards)
		}
	})

	t.Run("two of a rank do not make two runs", func(t *testing.T) {
		hand := []string{"4H", "5H", "5H", "6H"}
		got := runCandidates(r, hand, &Team{})
		if len(got) != 1 || len(got[0].Cards) != 3 {
			t.Fatalf("got %v, want one three-card run", got)
		}
	})

	t.Run("a block already on the table is a lay-off, not a new meld", func(t *testing.T) {
		team := &Team{Melds: []Meld{{
			ID: "t0-seq1", Kind: meldRun, Suit: "H", Cards: []string{"4H", "5H", "6H"},
		}}}
		got := runCandidates(r, []string{"7H", "8H", "9H"}, team)
		if len(got) != 0 {
			t.Errorf("offered %v as a new meld, but it extends the run already down", got)
		}
	})

	t.Run("nothing is offered where the variation has no sequences", func(t *testing.T) {
		hand := []string{"4H", "5H", "6H", "7H"}
		if got := runCandidates(variations["classic"], hand, &Team{}); len(got) != 0 {
			t.Errorf("classic offered a sequence: %v", got)
		}
	})
}

// A sequence grows at both ends, and as far as the hand reaches — holding the
// eight and nine against a run ending at seven is one lay-off, not two turns.
func TestRunExtensions(t *testing.T) {
	m := &Meld{Kind: meldRun, Suit: "H", Cards: []string{"5H", "6H", "7H"}}

	got := runExtensions([]string{"8H", "9H", "4H", "8D", "JOKER1"}, m, 4)
	want := map[string]bool{"8H": true, "9H": true, "4H": true}
	if len(got) != len(want) {
		t.Fatalf("got %v, want the three hearts that continue the run", got)
	}
	for _, c := range got {
		if !want[c] {
			t.Errorf("%s does not extend %v", c, m.Cards)
		}
	}

	// Room is the samba's seven, so a run three cards from one never offers four.
	if got := runExtensions([]string{"8H", "9H", "TH", "JH"}, m, 2); len(got) != 2 {
		t.Errorf("offered %v with room for two", got)
	}
}
