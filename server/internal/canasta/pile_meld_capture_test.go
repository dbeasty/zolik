package canasta

import (
	"testing"

	"zolik/server/internal/module"
)

// Claiming the pile with a meld already on the table.
//
// The cheapest capture in the game: the top card is matched by a meld the side
// laid on some earlier turn, so nothing leaves the hand and the whole pile is
// claimed anyway. Three things have to be true at once — the pile is reachable,
// the meld is still open, and the variation allows the move at all — and each
// of them is one subtest below, because each fails for its own reason and a
// player is owed the right one.
//
// `TestTakingThePile` in engine_test.go already pins the happy path for
// Classic. What is here is the rest of the clause: what "open" means, whose
// meld counts, and the variations that refuse.

// americanTable is a Modern American two-hander, built the way sambaTable
// builds a Samba one.
func americanTable(mutate func(s *GameState)) module.State {
	return twoHanded(func(s *GameState) {
		r := variations["modern_american"]
		s.Variation = "modern_american"
		s.Rules = &r
		s.HandSize = r.HandSize
		s.TargetScore = r.TargetScore
		s.CanastasToGoOut = r.CanastasToGoOut
		if mutate != nil {
			mutate(s)
		}
	})
}

// openTable is the shape every case here starts from: an ace on top of the
// pile, and three aces already melded under it.
func openTable(s *GameState) {
	s.Phase = phaseDraw
	s.Teams[0].HasMelded = true
	s.Teams[0].Melds = []Meld{{
		ID: meldID(0, "A"), TeamID: 0, Kind: meldSet, Rank: "A",
		Cards: []string{"AC", "AH", "AD"},
	}}
	s.DiscardPile = []string{"4C", "7D", "AS"}
	s.Hands["p1"] = []string{"8C", "9C", "TC"} // not an ace among them
}

// No pair needed: the meld does the matching, and the hand keeps everything it
// was holding plus the whole pile.
func TestMeldCaptureNeedsNothingFromHand(t *testing.T) {
	raw := twoHanded(openTable)

	next, code := apply(t, raw, "p1", module.Action{
		Verb: VerbTakePile, Target: meldID(0, "A"),
	})
	if code != "" {
		t.Fatalf("capture onto an open meld refused: %s", code)
	}

	s := mustDecode(t, next)
	if len(s.DiscardPile) != 0 {
		t.Errorf("the pile still holds %d cards after being claimed", len(s.DiscardPile))
	}
	// Three cards held, two buried cards gained, and the ace went to the meld
	// rather than to the hand.
	want := []string{"8C", "9C", "TC", "4C", "7D"}
	if got := len(s.Hands["p1"]); got != len(want) {
		t.Errorf("hand is %d cards, want %d: %v", got, len(want), s.Hands["p1"])
	}
	for _, c := range want {
		if !hasCards(s.Hands["p1"], []string{c}) {
			t.Errorf("%s is not in hand after the capture: %v", c, s.Hands["p1"])
		}
	}
	if m := s.Teams[0].meldByID(meldID(0, "A")); m == nil || len(m.Cards) != 4 {
		t.Errorf("the meld did not receive the top card: %+v", m)
	}
}

// The meld has to be one that can still take a card. A closed canasta cannot,
// and so cannot reach the pile either.
func TestMeldCaptureNeedsAnOpenMeld(t *testing.T) {
	cases := []struct {
		name  string
		cards []string
		want  string
	}{
		{
			name:  "an incomplete meld claims the pile",
			cards: []string{"AC", "AH", "AD", "AS", "2C", "2D"}, // six: one short
		},
		{
			name:  "a closed canasta cannot",
			cards: []string{"AC", "AH", "AD", "AS", "2C", "2D", "2H"}, // seven
			want:  ErrMeldClosed,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw := twoHanded(func(s *GameState) {
				openTable(s)
				s.Teams[0].Melds[0].Cards = tc.cards
			})
			if _, got := apply(t, raw, "p1", module.Action{
				Verb: VerbTakePile, Target: meldID(0, "A"),
			}); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// The pile still has to be reachable. A meld on the table is permission to
// claim it, not a way through a block or a freeze.
func TestMeldCaptureStillNeedsAClearPile(t *testing.T) {
	cases := []struct {
		name  string
		setup func(s *GameState)
		want  string
	}{
		{
			name: "a black three on top blocks it",
			setup: func(s *GameState) {
				s.DiscardPile = []string{"4C", "AS", "3C"}
			},
			want: ErrPileBlocked,
		},
		{
			name: "a wild on top cannot be melded, so nothing reaches the pile",
			setup: func(s *GameState) {
				s.DiscardPile = []string{"4C", "AS", "2H"}
			},
			want: ErrTopCardUnusable,
		},
		{
			name: "a wild buried in it freezes the pile against the meld too",
			setup: func(s *GameState) {
				s.Frozen = true
			},
			want: ErrPileFrozen,
		},
		{
			name: "a side that has not opened cannot use its own table",
			setup: func(s *GameState) {
				s.Teams[0].HasMelded = false
			},
			want: ErrPileFrozen,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw := twoHanded(func(s *GameState) {
				openTable(s)
				tc.setup(s)
			})
			if _, got := apply(t, raw, "p1", module.Action{
				Verb: VerbTakePile, Target: meldID(0, "A"),
			}); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// The meld belongs to the partnership, so a partner's meld claims the pile for
// either of them — and an opponent's claims nothing.
func TestMeldCaptureUsesThePartnershipsTable(t *testing.T) {
	t.Run("a partner's meld counts", func(t *testing.T) {
		raw := fourHanded(func(s *GameState) {
			s.Phase = phaseDraw
			s.Current = "p1"
			// Laid by p3, who sits on p1's side.
			s.Teams[0].HasMelded = true
			s.Teams[0].Melds = []Meld{{
				ID: meldID(0, "A"), TeamID: 0, Kind: meldSet, Rank: "A",
				Cards: []string{"AC", "AH", "AD"},
			}}
			s.DiscardPile = []string{"4C", "AS"}
			s.Hands["p1"] = []string{"8C", "9C"}
		})
		if _, code := apply(t, raw, "p1", module.Action{
			Verb: VerbTakePile, Target: meldID(0, "A"),
		}); code != "" {
			t.Errorf("capture onto a partner's meld refused: %s", code)
		}
	})

	t.Run("an opponent's does not", func(t *testing.T) {
		raw := twoHanded(func(s *GameState) {
			openTable(s)
			s.Teams[1].Melds = s.Teams[0].Melds
			s.Teams[1].Melds[0].ID, s.Teams[1].Melds[0].TeamID = meldID(1, "A"), 1
			s.Teams[0].Melds = nil
		})
		if _, got := apply(t, raw, "p1", module.Action{
			Verb: VerbTakePile, Target: meldID(1, "A"),
		}); got != ErrNotYourMeld {
			t.Errorf("got %q, want %q", got, ErrNotYourMeld)
		}
	})
}

// Which variations have the move at all. Modern American removed it: there the
// pile always costs two cards out of the hand, however the table looks.
func TestMeldCaptureByVariation(t *testing.T) {
	cases := []struct {
		name  string
		table func(mutate func(s *GameState)) module.State
		want  string
	}{
		{name: "classic allows it", table: twoHanded},
		{
			name:  "modern american forbids it outright",
			table: americanTable,
			want:  ErrMeldCaptureNotAllowed,
		},
		{
			// Samba refuses it too, but by the rule its players were told: the
			// pile is frozen the whole deal (docs/samba-plan.md §5). Samba's
			// consolation is `take_top` — see TestSambaTakeTop.
			name:  "samba refuses it as a frozen pile",
			table: sambaTable,
			want:  ErrPileFrozen,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw := tc.table(openTable)
			if _, got := apply(t, raw, "p1", module.Action{
				Verb: VerbTakePile, Target: meldID(0, "A"),
			}); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// Modern American's refusal is not merely enforced — the capture is never
// offered, so a client cannot put a button on screen that the engine will then
// refuse. The two answers come from one implementation (pileTakeOptions), and
// this is what pins them together for this clause.
func TestMeldCaptureIsNotOfferedWhereItIsForbidden(t *testing.T) {
	meldOffer := func(t *testing.T, raw module.State) *module.ActionOffer {
		t.Helper()
		offers, err := New().LegalActions(raw, "p1")
		if err != nil {
			t.Fatalf("LegalActions: %v", err)
		}
		for i, o := range offers {
			if o.ID == OfferTakePile+":meld:"+meldID(0, "A") {
				return &offers[i]
			}
		}
		return nil
	}

	t.Run("classic offers it", func(t *testing.T) {
		o := meldOffer(t, twoHanded(openTable))
		if o == nil {
			t.Fatal("no capture-onto-meld offer at a table where it is legal")
		}
		if !o.Enabled {
			t.Errorf("the offer is disabled: %s", o.WhyNot)
		}
		if o.LabelKey != "verb.takePileOntoMeld" {
			t.Errorf("label is %q, want verb.takePileOntoMeld", o.LabelKey)
		}
	})

	for _, tc := range []struct {
		name  string
		table func(mutate func(s *GameState)) module.State
	}{
		{"modern american", americanTable},
		{"samba", sambaTable},
	} {
		t.Run(tc.name+" offers nothing to press", func(t *testing.T) {
			if o := meldOffer(t, tc.table(openTable)); o != nil {
				t.Errorf("capture-onto-meld was offered: %+v", o)
			}
		})
	}
}

// The written rules say which it is, so a player can find out before the
// argument rather than during it.
func TestRulesStateThePileCapture(t *testing.T) {
	cases := []struct {
		variation string
		want      string
	}{
		{"classic", "canasta.rules.pileOntoMeld"},
		{"modern_american", "canasta.rules.pileNoMeldCapture"},
		{"samba", "canasta.rules.pileAlwaysFrozen"},
	}

	for _, tc := range cases {
		t.Run(tc.variation, func(t *testing.T) {
			sections, err := New().Rules(module.MatchConfig{Variation: tc.variation})
			if err != nil {
				t.Fatalf("Rules: %v", err)
			}
			ids := module.RuleIDsIn(sections)
			if !ids[tc.want] {
				t.Errorf("%s does not state %s; it states %v", tc.variation, tc.want, ids)
			}
			// And does not also state the opposite.
			for _, other := range []string{"canasta.rules.pileOntoMeld", "canasta.rules.pileNoMeldCapture"} {
				if other != tc.want && ids[other] {
					t.Errorf("%s states %s as well as %s", tc.variation, other, tc.want)
				}
			}
		})
	}
}
