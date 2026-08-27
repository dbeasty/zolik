package canasta

import (
	"strings"
	"testing"

	"zolik/server/internal/module"
)

// filler is a draw pile with nothing interesting in it, so a test that is not
// about drawing never accidentally ends a deal by running the stock out (see
// advanceTurn) or turns up a red three it did not ask for.
func filler(n int) []string {
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, "8S")
	}
	return out
}

// twoHanded builds a head-to-head state directly, so a rule can be tested at
// the exact moment it applies instead of being played into existence.
func twoHanded(mutate func(s *GameState)) module.State {
	s := &GameState{
		Status:          "active",
		Players:         []string{"p1", "p2"},
		TurnOrder:       []string{"p1", "p2"},
		Current:         "p1",
		Phase:           phaseDraw,
		Teams:           []Team{{ID: 0, Players: []string{"p1"}}, {ID: 1, Players: []string{"p2"}}},
		TeamOf:          map[string]int{"p1": 0, "p2": 1},
		Hands:           map[string][]string{"p1": {}, "p2": {"8H", "9H", "TH"}},
		DrawPile:        filler(30),
		HandSize:        11,
		TargetScore:     5000,
		CanastasToGoOut: 1,
	}
	if mutate != nil {
		mutate(s)
	}
	// Derived exactly the way advanceTurn derives it, so a hand-built state
	// cannot disagree with one that was played into existence — otherwise a
	// test setting up a table with melds already on it would look like a
	// concealed go-out to the scorer.
	if s.Current != "" {
		s.MeldsAtTurnStart = len(s.team(s.Current).Melds) > 0
	}
	raw, err := encode(s)
	if err != nil {
		panic(err)
	}
	return raw
}

// fourHanded is the same for a partnership table: p1+p3 against p2+p4.
func fourHanded(mutate func(s *GameState)) module.State {
	s := &GameState{
		Status:    "active",
		Players:   []string{"p1", "p2", "p3", "p4"},
		TurnOrder: []string{"p1", "p2", "p3", "p4"},
		Current:   "p1",
		Phase:     phaseMeld,
		Teams: []Team{
			{ID: 0, Players: []string{"p1", "p3"}},
			{ID: 1, Players: []string{"p2", "p4"}},
		},
		TeamOf:          map[string]int{"p1": 0, "p3": 0, "p2": 1, "p4": 1},
		Hands:           map[string][]string{"p1": {}, "p2": {}, "p3": {}, "p4": {}},
		DrawPile:        filler(30),
		HandSize:        11,
		TargetScore:     5000,
		CanastasToGoOut: 1,
	}
	if mutate != nil {
		mutate(s)
	}
	// Derived exactly the way advanceTurn derives it, so a hand-built state
	// cannot disagree with one that was played into existence — otherwise a
	// test setting up a table with melds already on it would look like a
	// concealed go-out to the scorer.
	if s.Current != "" {
		s.MeldsAtTurnStart = len(s.team(s.Current).Melds) > 0
	}
	raw, err := encode(s)
	if err != nil {
		panic(err)
	}
	return raw
}

// apply is the shorthand every table below uses: run one action and report the
// engine's own error code.
func apply(t *testing.T, raw module.State, player string, a module.Action) (module.State, string) {
	t.Helper()
	next, _, err := New().Apply(raw, player, a)
	if err != nil {
		return raw, module.CodeOf(err)
	}
	return next, ""
}

func mustDecode(t *testing.T, raw module.State) *GameState {
	t.Helper()
	s, err := decode(raw)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	return s
}

// --- taking the discard pile ------------------------------------------------

// TestTakingThePile is the table for the rule the whole game turns on. Every
// row is a real situation at a real table; the frozen ones are what make the
// pile worth freezing.
func TestTakingThePile(t *testing.T) {
	cases := []struct {
		name   string
		setup  func(s *GameState)
		action module.Action
		want   string // "" means accepted
	}{
		{
			name: "two naturals take an unfrozen pile",
			setup: func(s *GameState) {
				s.Teams[0].HasMelded = true
				s.DiscardPile = []string{"4C", "AS"}
				s.Hands["p1"] = []string{"AH", "AD", "8C", "9C"}
			},
			action: module.Action{Verb: VerbTakePile, Cards: []string{"AH", "AD"}},
		},
		{
			name: "a natural and a wild take an unfrozen pile",
			setup: func(s *GameState) {
				s.Teams[0].HasMelded = true
				s.DiscardPile = []string{"4C", "AS"}
				s.Hands["p1"] = []string{"AH", "2C", "8C", "9C"}
			},
			action: module.Action{Verb: VerbTakePile, Cards: []string{"AH", "2C"}},
		},
		{
			name: "a wild cannot take a frozen pile",
			setup: func(s *GameState) {
				s.Teams[0].HasMelded = true
				s.Frozen = true
				s.DiscardPile = []string{"4C", "AS"}
				s.Hands["p1"] = []string{"AH", "2C", "8C", "9C"}
			},
			action: module.Action{Verb: VerbTakePile, Cards: []string{"AH", "2C"}},
			want:   ErrPileFrozen,
		},
		{
			name: "two naturals still take a frozen pile",
			setup: func(s *GameState) {
				s.Teams[0].HasMelded = true
				s.Frozen = true
				s.DiscardPile = []string{"4C", "AS"}
				s.Hands["p1"] = []string{"AH", "AD", "8C", "9C"}
			},
			action: module.Action{Verb: VerbTakePile, Cards: []string{"AH", "AD"}},
		},
		{
			name: "a partnership that has not opened is frozen out even when the pile is not",
			setup: func(s *GameState) {
				s.DiscardPile = []string{"4C", "AS"}
				s.Hands["p1"] = []string{"AH", "2C", "8C", "9C"}
			},
			action: module.Action{Verb: VerbTakePile, Cards: []string{"AH", "2C"}},
			want:   ErrPileFrozen,
		},
		{
			name: "a black three on top blocks the pile",
			setup: func(s *GameState) {
				s.Teams[0].HasMelded = true
				s.DiscardPile = []string{"AS", "3C"}
				s.Hands["p1"] = []string{"3S", "3H", "8C"}
			},
			action: module.Action{Verb: VerbTakePile, Cards: []string{"3S", "3H"}},
			want:   ErrPileBlocked,
		},
		{
			name: "a wild on top cannot be melded, so the pile cannot be taken",
			setup: func(s *GameState) {
				s.Teams[0].HasMelded = true
				s.DiscardPile = []string{"AS", "2H"}
				s.Hands["p1"] = []string{"AH", "AD", "8C"}
			},
			action: module.Action{Verb: VerbTakePile, Cards: []string{"AH", "AD"}},
			want:   ErrTopCardUnusable,
		},
		{
			name: "the top card can be laid off onto the partnership's own meld",
			setup: func(s *GameState) {
				s.Teams[0].HasMelded = true
				s.Teams[0].Melds = []Meld{{ID: meldID(0, "A"), TeamID: 0, Rank: "A", Cards: []string{"AC", "AH", "AD"}}}
				s.DiscardPile = []string{"4C", "AS"}
				s.Hands["p1"] = []string{"8C", "9C"}
			},
			action: module.Action{Verb: VerbTakePile, Target: meldID(0, "A")},
		},
		{
			name: "but not off an opponent's meld",
			setup: func(s *GameState) {
				s.Teams[0].HasMelded = true
				s.Teams[1].Melds = []Meld{{ID: meldID(1, "A"), TeamID: 1, Rank: "A", Cards: []string{"AC", "AH", "AD"}}}
				s.DiscardPile = []string{"4C", "AS"}
				s.Hands["p1"] = []string{"8C", "9C"}
			},
			action: module.Action{Verb: VerbTakePile, Target: meldID(1, "A")},
			want:   ErrNotYourMeld,
		},
		{
			name: "an empty pile cannot be taken",
			setup: func(s *GameState) {
				s.Teams[0].HasMelded = true
				s.DiscardPile = nil
				s.Hands["p1"] = []string{"AH", "AD"}
			},
			action: module.Action{Verb: VerbTakePile, Cards: []string{"AH", "AD"}},
			want:   ErrPileEmpty,
		},
		{
			name: "the pile cannot be taken after drawing",
			setup: func(s *GameState) {
				s.Phase = phaseMeld
				s.Teams[0].HasMelded = true
				s.DiscardPile = []string{"4C", "AS"}
				s.Hands["p1"] = []string{"AH", "AD", "8C"}
			},
			action: module.Action{Verb: VerbTakePile, Cards: []string{"AH", "AD"}},
			want:   ErrWrongPhase,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw := twoHanded(tc.setup)
			_, got := apply(t, raw, "p1", tc.action)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// TestTakingThePileMovesEveryCard checks the payoff, not just the permission:
// the top card ends up melded and everything buried under it ends up in hand.
func TestTakingThePileMovesEveryCard(t *testing.T) {
	raw := twoHanded(func(s *GameState) {
		s.Teams[0].HasMelded = true
		s.Frozen = true
		s.DiscardPile = []string{"4C", "5D", "9S", "AS"}
		s.Hands["p1"] = []string{"AH", "AD", "8C"}
	})
	next, code := apply(t, raw, "p1", module.Action{Verb: VerbTakePile, Cards: []string{"AH", "AD"}})
	if code != "" {
		t.Fatalf("refused: %s", code)
	}
	s := mustDecode(t, next)

	if len(s.DiscardPile) != 0 {
		t.Errorf("pile should be empty, holds %v", s.DiscardPile)
	}
	if s.Frozen {
		t.Error("taking the pile should thaw it")
	}
	m := s.Teams[0].meld("A")
	if m == nil || len(m.Cards) != 3 {
		t.Fatalf("expected a three-card ace meld, got %+v", m)
	}
	// 8C was already there; 4C, 5D and 9S came off the pile; the two aces left.
	want := map[string]bool{"8C": true, "4C": true, "5D": true, "9S": true}
	if len(s.Hands["p1"]) != len(want) {
		t.Fatalf("hand is %v, want the four cards %v", s.Hands["p1"], want)
	}
	for _, c := range s.Hands["p1"] {
		if !want[c] {
			t.Errorf("unexpected card %q in hand", c)
		}
	}
	if s.Phase != phaseMeld {
		t.Errorf("phase is %q, want %q", s.Phase, phaseMeld)
	}
}

// --- meld shape -------------------------------------------------------------

func TestMeldShape(t *testing.T) {
	cases := []struct {
		name  string
		cards []string
		want  string
	}{
		{name: "three naturals", cards: []string{"KH", "KD", "KS"}},
		{name: "two naturals and a wild", cards: []string{"KH", "KD", "2S"}},
		{name: "two naturals and three wilds", cards: []string{"KH", "KD", "2S", "2C", "JOKER1"}},
		{name: "seven cards is a canasta", cards: []string{"KH", "KD", "KS", "KC", "KH", "2S", "JOKER1"}},
		{name: "too few", cards: []string{"KH", "KD"}, want: ErrMeldTooSmall},
		{name: "too many", cards: []string{"KH", "KD", "KS", "KC", "KH", "KD", "KS", "KC"}, want: ErrMeldTooLarge},
		{name: "mixed ranks", cards: []string{"KH", "QD", "KS"}, want: ErrMeldMixedRanks},
		{name: "four wilds is too many", cards: []string{"KH", "KD", "2S", "2C", "2D", "JOKER1"}, want: ErrTooManyWilds},
		{name: "one natural is not enough", cards: []string{"KH", "2S", "JOKER1"}, want: ErrNotEnoughNaturals},
		{name: "all wilds have no rank", cards: []string{"2S", "2C", "JOKER1"}, want: ErrMeldMixedRanks},
		{name: "threes are not an ordinary meld", cards: []string{"3C", "3S", "3C"}, want: ErrCannotMeldThree},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateMeld(tc.cards)
			got := ""
			if err != nil {
				got = module.CodeOf(err)
			}
			if got != tc.want {
				t.Errorf("validateMeld(%v) = %q, want %q", tc.cards, got, tc.want)
			}
		})
	}
}

// --- the initial meld -------------------------------------------------------

// TestInitialMeldMinimum pins the four floors, which are the mechanism that
// stops a partnership in front from running away with the match.
func TestInitialMeldMinimum(t *testing.T) {
	cases := []struct {
		score int
		want  int
	}{
		{score: -200, want: 15}, {score: -1, want: 15},
		{score: 0, want: 50}, {score: 1499, want: 50},
		{score: 1500, want: 90}, {score: 2999, want: 90},
		{score: 3000, want: 120}, {score: 9000, want: 120},
	}
	for _, tc := range cases {
		if got := initialMeldMinimum(tc.score); got != tc.want {
			t.Errorf("initialMeldMinimum(%d) = %d, want %d", tc.score, got, tc.want)
		}
	}
}

func TestOpeningTheTable(t *testing.T) {
	cases := []struct {
		name  string
		setup func(s *GameState)
		cards []string
		want  string
	}{
		{
			name: "three aces clear a fifty floor",
			setup: func(s *GameState) {
				s.Hands["p1"] = []string{"AH", "AD", "AS", "8C", "9C"}
			},
			cards: []string{"AH", "AD", "AS"},
		},
		{
			name: "three fours do not, with nothing else in hand to reach it",
			setup: func(s *GameState) {
				s.Hands["p1"] = []string{"4H", "4D", "4S", "8C", "9C"}
			},
			cards: []string{"4H", "4D", "4S"},
			want:  ErrInitialMeldNotMet,
		},
		{
			name: "three fours are fine when a second meld can still reach fifty",
			setup: func(s *GameState) {
				s.Hands["p1"] = []string{"4H", "4D", "4S", "KH", "KD", "KS", "KC", "8C"}
			},
			cards: []string{"4H", "4D", "4S"},
		},
		{
			name: "a partnership in the hole only needs fifteen",
			setup: func(s *GameState) {
				s.Teams[0].Score = -100
				s.Hands["p1"] = []string{"4H", "4D", "4S", "8C", "9C"}
			},
			cards: []string{"4H", "4D", "4S"},
		},
		{
			name: "a partnership past three thousand needs a hundred and twenty",
			setup: func(s *GameState) {
				s.Teams[0].Score = 3200
				s.Hands["p1"] = []string{"AH", "AD", "AS", "8C", "9C"}
			},
			cards: []string{"AH", "AD", "AS"},
			want:  ErrInitialMeldNotMet,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw := twoHanded(func(s *GameState) {
				s.Phase = phaseMeld
				tc.setup(s)
			})
			_, got := apply(t, raw, "p1", module.Action{Verb: VerbLayMeld, Cards: tc.cards})
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// TestOpeningAcrossTwoMelds is the reason the minimum is a property of a turn
// rather than of a meld — and the reason a lay that puts it out of reach has to
// be refused before the cards leave the hand, since nothing can pick them up
// again.
func TestOpeningAcrossTwoMelds(t *testing.T) {
	raw := twoHanded(func(s *GameState) {
		s.Phase = phaseMeld
		// 15 in fours, then 40 in kings: neither opens alone, together they do.
		s.Hands["p1"] = []string{"4H", "4D", "4S", "KH", "KD", "KS", "KC", "8C", "9C"}
	})

	next, code := apply(t, raw, "p1", module.Action{Verb: VerbLayMeld, Cards: []string{"4H", "4D", "4S"}})
	if code != "" {
		t.Fatalf("first meld refused: %s", code)
	}
	s := mustDecode(t, next)
	if s.Teams[0].HasMelded {
		t.Error("fifteen points should not have opened the table")
	}

	// Until the floor is cleared the turn cannot end, and the offer list says
	// so rather than leaving the player to guess.
	if _, code := apply(t, next, "p1", module.Action{Verb: VerbDiscard, Cards: []string{"8C"}}); code != ErrInitialMeldNotMet {
		t.Errorf("discard before opening = %q, want %q", code, ErrInitialMeldNotMet)
	}
	offers, err := New().LegalActions(next, "p1")
	if err != nil {
		t.Fatalf("LegalActions: %v", err)
	}
	if o := module.FindOffer(offers, OfferDiscard); o == nil || o.Enabled || o.WhyNot != ErrInitialMeldNotMet {
		t.Errorf("the discard offer should be off with a reason, got %+v", o)
	}

	next2, code := apply(t, next, "p1", module.Action{Verb: VerbLayMeld, Cards: []string{"KH", "KD", "KS", "KC"}})
	if code != "" {
		t.Fatalf("second meld refused: %s", code)
	}
	s = mustDecode(t, next2)
	if !s.Teams[0].HasMelded {
		t.Error("fifteen plus forty should have opened the table")
	}
	if _, code := apply(t, next2, "p1", module.Action{Verb: VerbDiscard, Cards: []string{"8C"}}); code != "" {
		t.Errorf("discard after opening refused: %s", code)
	}
}

// --- lay-offs and partnerships ----------------------------------------------

// TestPartnersShareMelds is the single fact that made this a module rather than
// a RulesConfig profile: a meld belongs to the partnership, not the player who
// laid it.
func TestPartnersShareMelds(t *testing.T) {
	raw := fourHanded(func(s *GameState) {
		s.Current = "p3"
		s.Teams[0].HasMelded = true
		s.Teams[0].Melds = []Meld{{ID: meldID(0, "K"), TeamID: 0, Rank: "K", Cards: []string{"KH", "KD", "KS"}}}
		s.Hands["p3"] = []string{"KC", "8C", "9C"}
		s.Hands["p2"] = []string{"KC", "8C", "9C"}
	})

	// p3 is p1's partner and may extend the meld p1 laid.
	next, code := apply(t, raw, "p3", module.Action{
		Verb: VerbLayOff, Cards: []string{"KC"}, Target: meldID(0, "K"),
	})
	if code != "" {
		t.Fatalf("a partner should be able to lay off, got %s", code)
	}
	if m := mustDecode(t, next).Teams[0].meld("K"); len(m.Cards) != 4 {
		t.Errorf("meld has %d cards, want 4", len(m.Cards))
	}

	// An opponent may not, even holding the same card.
	opp := fourHanded(func(s *GameState) {
		s.Current = "p2"
		s.Teams[0].HasMelded = true
		s.Teams[1].HasMelded = true
		s.Teams[0].Melds = []Meld{{ID: meldID(0, "K"), TeamID: 0, Rank: "K", Cards: []string{"KH", "KD", "KS"}}}
		s.Hands["p2"] = []string{"KC", "8C", "9C"}
	})
	if _, code := apply(t, opp, "p2", module.Action{
		Verb: VerbLayOff, Cards: []string{"KC"}, Target: meldID(0, "K"),
	}); code != ErrNotYourMeld {
		t.Errorf("an opponent laying off = %q, want %q", code, ErrNotYourMeld)
	}
}

func TestLayOffRules(t *testing.T) {
	cases := []struct {
		name   string
		setup  func(s *GameState)
		action module.Action
		want   string
	}{
		{
			name: "a canasta is closed",
			setup: func(s *GameState) {
				s.Teams[0].HasMelded = true
				s.Teams[0].Melds = []Meld{{ID: meldID(0, "K"), TeamID: 0, Rank: "K",
					Cards: []string{"KH", "KD", "KS", "KC", "KH", "KD", "KS"}}}
				s.Hands["p1"] = []string{"KC", "8C", "9C"}
			},
			action: module.Action{Verb: VerbLayOff, Cards: []string{"KC"}, Target: meldID(0, "K")},
			want:   ErrMeldClosed,
		},
		{
			name: "a fourth wild will not fit",
			setup: func(s *GameState) {
				s.Teams[0].HasMelded = true
				s.Teams[0].Melds = []Meld{{ID: meldID(0, "K"), TeamID: 0, Rank: "K",
					Cards: []string{"KH", "KD", "2S", "2C", "JOKER1"}}}
				s.Hands["p1"] = []string{"2D", "8C", "9C"}
			},
			action: module.Action{Verb: VerbLayOff, Cards: []string{"2D"}, Target: meldID(0, "K")},
			want:   ErrTooManyWilds,
		},
		{
			name: "the wrong rank will not fit either",
			setup: func(s *GameState) {
				s.Teams[0].HasMelded = true
				s.Teams[0].Melds = []Meld{{ID: meldID(0, "K"), TeamID: 0, Rank: "K", Cards: []string{"KH", "KD", "KS"}}}
				s.Hands["p1"] = []string{"QC", "8C", "9C"}
			},
			action: module.Action{Verb: VerbLayOff, Cards: []string{"QC"}, Target: meldID(0, "K")},
			want:   ErrWrongRank,
		},
		{
			name: "you cannot lay off before your partnership has any melds",
			setup: func(s *GameState) {
				s.Hands["p1"] = []string{"KC", "8C", "9C"}
			},
			action: module.Action{Verb: VerbLayOff, Cards: []string{"KC"}, Target: meldID(0, "K")},
			want:   ErrMustMeldFirst,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw := twoHanded(func(s *GameState) {
				s.Phase = phaseMeld
				tc.setup(s)
			})
			_, got := apply(t, raw, "p1", tc.action)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// --- threes -----------------------------------------------------------------

// TestRedThreesLayThemselves covers the card that is not really in the game: it
// never sits in a hand and never reaches the pile.
func TestRedThreesLayThemselves(t *testing.T) {
	t.Run("drawing one lays it and takes a replacement", func(t *testing.T) {
		raw := twoHanded(func(s *GameState) {
			s.Hands["p1"] = []string{"8C", "9C"}
			// Drawn from the end: a red three, then an ordinary replacement.
			s.DrawPile = []string{"7C", "QS", "3H"}
		})
		next, code := apply(t, raw, "p1", module.Action{Verb: VerbDraw})
		if code != "" {
			t.Fatalf("draw refused: %s", code)
		}
		s := mustDecode(t, next)
		if len(s.Teams[0].RedThrees) != 1 || s.Teams[0].RedThrees[0] != "3H" {
			t.Errorf("red threes are %v, want [3H]", s.Teams[0].RedThrees)
		}
		for _, c := range s.Hands["p1"] {
			if isRedThree(c) {
				t.Error("a red three stayed in the hand")
			}
		}
		if len(s.Hands["p1"]) != 3 {
			t.Errorf("hand has %d cards, want 3 — the replacement should have arrived", len(s.Hands["p1"]))
		}
	})

	t.Run("one can never be discarded", func(t *testing.T) {
		raw := twoHanded(func(s *GameState) {
			s.Phase = phaseMeld
			s.Teams[0].HasMelded = true
			s.Hands["p1"] = []string{"3H", "8C", "9C"}
		})
		if _, code := apply(t, raw, "p1", module.Action{Verb: VerbDiscard, Cards: []string{"3H"}}); code != ErrCannotDiscardThree {
			t.Errorf("got %q, want %q", code, ErrCannotDiscardThree)
		}
	})

	t.Run("they count against a partnership with no canasta", func(t *testing.T) {
		bare := &Team{RedThrees: []string{"3H", "3D"}}
		if got := redThreeScore(bare); got != -200 {
			t.Errorf("two red threes and no canasta = %d, want -200", got)
		}
		withCanasta := &Team{
			RedThrees: []string{"3H", "3D"},
			Melds:     []Meld{{Rank: "K", Cards: []string{"KH", "KD", "KS", "KC", "KH", "KD", "KS"}}},
		}
		if got := redThreeScore(withCanasta); got != 200 {
			t.Errorf("two red threes with a canasta = %d, want 200", got)
		}
		all := &Team{
			RedThrees: []string{"3H", "3D", "3H", "3D"},
			Melds:     withCanasta.Melds,
		}
		if got := redThreeScore(all); got != allRedThreesBonus {
			t.Errorf("all four red threes = %d, want %d", got, allRedThreesBonus)
		}
	})
}

// TestBlackThreesOnlyGoDownOnTheWayOut covers the other three: a blocker, not a
// scoring card.
func TestBlackThreesOnlyGoDownOnTheWayOut(t *testing.T) {
	t.Run("not while the hand still holds cards", func(t *testing.T) {
		raw := twoHanded(func(s *GameState) {
			s.Phase = phaseMeld
			s.Teams[0].HasMelded = true
			s.Teams[0].Melds = []Meld{{ID: meldID(0, "K"), TeamID: 0, Rank: "K",
				Cards: []string{"KH", "KD", "KS", "KC", "KH", "KD", "KS"}}}
			s.Hands["p1"] = []string{"3C", "3S", "3C", "8C", "9C"}
		})
		if _, code := apply(t, raw, "p1", module.Action{
			Verb: VerbLayMeld, Cards: []string{"3C", "3S", "3C"},
		}); code != ErrCannotMeldThree {
			t.Errorf("got %q, want %q", code, ErrCannotMeldThree)
		}
	})

	t.Run("but yes as the move that empties it", func(t *testing.T) {
		raw := twoHanded(func(s *GameState) {
			s.Phase = phaseMeld
			s.Teams[0].HasMelded = true
			s.Teams[0].Melds = []Meld{{ID: meldID(0, "K"), TeamID: 0, Rank: "K",
				Cards: []string{"KH", "KD", "KS", "KC", "KH", "KD", "KS"}}}
			s.Hands["p1"] = []string{"3C", "3S", "3C"}
		})
		next, code := apply(t, raw, "p1", module.Action{
			Verb: VerbLayMeld, Cards: []string{"3C", "3S", "3C"},
		})
		if code != "" {
			t.Fatalf("going out on black threes refused: %s", code)
		}
		if s := mustDecode(t, next); s.LastDeal == nil || s.LastDeal.WentOut != "p1" {
			t.Errorf("the deal should have ended with p1 out, got %+v", s.LastDeal)
		}
	})
}

// --- going out --------------------------------------------------------------

func TestGoingOutNeedsACanasta(t *testing.T) {
	withCanasta := func(s *GameState) {
		s.Teams[0].Melds = append(s.Teams[0].Melds, Meld{
			ID: meldID(0, "K"), TeamID: 0, Rank: "K",
			Cards: []string{"KH", "KD", "KS", "KC", "KH", "KD", "KS"},
		})
	}

	t.Run("the last card cannot be discarded without one", func(t *testing.T) {
		raw := twoHanded(func(s *GameState) {
			s.Phase = phaseMeld
			s.Teams[0].HasMelded = true
			s.Teams[0].Melds = []Meld{{ID: meldID(0, "Q"), TeamID: 0, Rank: "Q", Cards: []string{"QH", "QD", "QS"}}}
			s.Hands["p1"] = []string{"8C"}
		})
		if _, code := apply(t, raw, "p1", module.Action{Verb: VerbDiscard, Cards: []string{"8C"}}); code != ErrCannotGoOutYet {
			t.Errorf("got %q, want %q", code, ErrCannotGoOutYet)
		}
	})

	t.Run("and can with one", func(t *testing.T) {
		raw := twoHanded(func(s *GameState) {
			s.Phase = phaseMeld
			s.Teams[0].HasMelded = true
			withCanasta(s)
			s.Hands["p1"] = []string{"8C"}
		})
		next, code := apply(t, raw, "p1", module.Action{Verb: VerbDiscard, Cards: []string{"8C"}})
		if code != "" {
			t.Fatalf("going out refused: %s", code)
		}
		s := mustDecode(t, next)
		if s.LastDeal == nil || s.LastDeal.WentOut != "p1" {
			t.Fatalf("deal should have ended with p1 out, got %+v", s.LastDeal)
		}
		if s.LastDeal.Teams[0].GoingOut != goingOutBonus {
			t.Errorf("going-out bonus was %d, want %d", s.LastDeal.Teams[0].GoingOut, goingOutBonus)
		}
	})

	t.Run("modern american wants two", func(t *testing.T) {
		raw := twoHanded(func(s *GameState) {
			s.Phase = phaseMeld
			s.CanastasToGoOut = 2
			s.Teams[0].HasMelded = true
			withCanasta(s)
			s.Hands["p1"] = []string{"8C"}
		})
		if _, code := apply(t, raw, "p1", module.Action{Verb: VerbDiscard, Cards: []string{"8C"}}); code != ErrCannotGoOutYet {
			t.Errorf("got %q, want %q", code, ErrCannotGoOutYet)
		}
	})
}

// TestMeldingCannotStrandAPlayer is the rule that keeps every turn finishable:
// a turn ends with a discard, and shedding your last card is going out, so a
// partnership that cannot go out must be left holding two.
func TestMeldingCannotStrandAPlayer(t *testing.T) {
	raw := twoHanded(func(s *GameState) {
		s.Phase = phaseMeld
		s.Teams[0].HasMelded = true
		s.Teams[0].Melds = []Meld{{ID: meldID(0, "Q"), TeamID: 0, Rank: "Q", Cards: []string{"QH", "QD", "QS"}}}
		s.Hands["p1"] = []string{"KH", "KD", "KS", "8C"}
	})
	// Melding the three kings would leave one card, which could only be
	// discarded by going out — and this partnership has no canasta.
	if _, code := apply(t, raw, "p1", module.Action{
		Verb: VerbLayMeld, Cards: []string{"KH", "KD", "KS"},
	}); code != ErrMustKeepACard {
		t.Errorf("got %q, want %q", code, ErrMustKeepACard)
	}
}

// TestConcealedGoOut is worth 200 rather than 100, and only for a partnership
// that had nothing on the table when the turn began.
func TestConcealedGoOut(t *testing.T) {
	raw := twoHanded(func(s *GameState) {
		s.Phase = phaseMeld
		s.MeldsAtTurnStart = false
		s.Hands["p1"] = []string{"KH", "KD", "KS", "KC", "KH", "KD", "KS"}
	})
	next, code := apply(t, raw, "p1", module.Action{
		Verb: VerbLayMeld, Cards: []string{"KH", "KD", "KS", "KC", "KH", "KD", "KS"},
	})
	if code != "" {
		t.Fatalf("concealed go-out refused: %s", code)
	}
	s := mustDecode(t, next)
	if s.LastDeal == nil || !s.LastDeal.Concealed {
		t.Fatalf("expected a concealed go-out, got %+v", s.LastDeal)
	}
	if s.LastDeal.Teams[0].GoingOut != concealedBonus {
		t.Errorf("bonus was %d, want %d", s.LastDeal.Teams[0].GoingOut, concealedBonus)
	}
}

// --- the discard pile --------------------------------------------------------

func TestDiscardingAWildFreezesThePile(t *testing.T) {
	raw := twoHanded(func(s *GameState) {
		s.Phase = phaseMeld
		s.Teams[0].HasMelded = true
		s.DiscardPile = []string{"8H"}
		s.Hands["p1"] = []string{"2C", "9C", "TC"}
	})
	next, code := apply(t, raw, "p1", module.Action{Verb: VerbDiscard, Cards: []string{"2C"}})
	if code != "" {
		t.Fatalf("discard refused: %s", code)
	}
	if s := mustDecode(t, next); !s.Frozen {
		t.Error("discarding a wild should freeze the pile")
	}
}

// TestStockExhaustionEndsTheDeal covers the deal nobody wins: the stock runs
// out and the player to move has no legal capture, so there is no move to make.
func TestStockExhaustionEndsTheDeal(t *testing.T) {
	raw := twoHanded(func(s *GameState) {
		s.Phase = phaseMeld
		s.Teams[0].HasMelded = true
		s.DrawPile = nil
		s.DiscardPile = []string{"8H"}
		// p2 holds nothing that could take a pile topped by a nine.
		s.Hands["p1"] = []string{"9C", "TC", "JC"}
		s.Hands["p2"] = []string{"4S", "5S", "6S"}
	})
	next, code := apply(t, raw, "p1", module.Action{Verb: VerbDiscard, Cards: []string{"9C"}})
	if code != "" {
		t.Fatalf("discard refused: %s", code)
	}
	s := mustDecode(t, next)
	if s.LastDeal == nil {
		t.Fatal("the deal should have ended")
	}
	if !s.LastDeal.Exhausted {
		t.Error("the deal should be marked exhausted")
	}
	if s.LastDeal.WentOut != "" {
		t.Errorf("nobody went out, but %q is recorded", s.LastDeal.WentOut)
	}
	for _, tr := range s.LastDeal.Teams {
		if tr.GoingOut != 0 {
			t.Errorf("team %d got a going-out bonus for a deal nobody won", tr.TeamID)
		}
	}
}

// --- turn order and status ---------------------------------------------------

func TestTurnAndStatusGuards(t *testing.T) {
	raw := twoHanded(nil)

	if _, code := apply(t, raw, "p2", module.Action{Verb: VerbDraw}); code != ErrNotYourTurn {
		t.Errorf("off-turn draw = %q, want %q", code, ErrNotYourTurn)
	}
	if _, code := apply(t, raw, "p1", module.Action{Verb: "eat_the_deck"}); code != ErrUnknownAction {
		t.Errorf("nonsense verb = %q, want %q", code, ErrUnknownAction)
	}
	done := twoHanded(func(s *GameState) { s.Status = "completed" })
	if _, code := apply(t, done, "p1", module.Action{Verb: VerbDraw}); code != ErrGameNotActive {
		t.Errorf("move in a finished match = %q, want %q", code, ErrGameNotActive)
	}
}

// TestApplyDoesNotMutateItsInput is free behind an opaque State — Apply decodes
// a fresh value every call — but pinning it stops anyone reintroducing the
// aliasing hazard the rummy engine needed a regression test for.
func TestApplyDoesNotMutateItsInput(t *testing.T) {
	raw := twoHanded(func(s *GameState) {
		s.Phase = phaseMeld
		s.Teams[0].HasMelded = true
		s.Hands["p1"] = []string{"KH", "KD", "KS", "8C", "9C"}
	})
	before := string(raw)

	if _, code := apply(t, raw, "p1", module.Action{Verb: VerbLayMeld, Cards: []string{"KH", "KD", "KS"}}); code != "" {
		t.Fatalf("meld refused: %s", code)
	}
	if _, err := New().LegalActions(raw, "p1"); err != nil {
		t.Fatalf("LegalActions: %v", err)
	}
	if _, err := New().View(raw, "p1"); err != nil {
		t.Fatalf("View: %v", err)
	}
	if string(raw) != before {
		t.Error("the caller's state changed underneath it")
	}
}

// --- dealing ------------------------------------------------------------------

func TestDealingHandlesRedThreesAndTheUpcard(t *testing.T) {
	m := New()
	for seed := int64(1); seed <= 40; seed++ {
		raw, err := m.NewMatch(module.MatchConfig{}, refs("p1", "p2", "p3", "p4"), seed)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		s := mustDecode(t, raw)

		for p, hand := range s.Hands {
			for _, c := range hand {
				if isRedThree(c) {
					t.Errorf("seed %d: %s was dealt a red three that stayed in hand", seed, p)
				}
			}
			if len(hand) != s.HandSize {
				t.Errorf("seed %d: %s holds %d cards, want %d", seed, p, len(hand), s.HandSize)
			}
		}

		// Every card is accounted for exactly once: two decks, no card lost to
		// the red-three replacement loop and none duplicated by it.
		total := len(s.DrawPile) + len(s.DiscardPile)
		for _, h := range s.Hands {
			total += len(h)
		}
		for i := range s.Teams {
			total += len(s.Teams[i].RedThrees)
		}
		if total != 108 {
			t.Errorf("seed %d: %d cards in play, want 108", seed, total)
		}

		// A wild or a red three turned up starts the deal frozen.
		if top := s.top(); isWild(top) || isRedThree(top) {
			if !s.Frozen {
				t.Errorf("seed %d: upcard %q should have frozen the pile", seed, top)
			}
		}
	}
}

// TestMatchRunsMultipleDeals checks the rollover: a deal that ends short of the
// target deals again rather than declaring a winner.
func TestMatchRunsMultipleDeals(t *testing.T) {
	raw := twoHanded(func(s *GameState) {
		s.Phase = phaseMeld
		s.TargetScore = 100000 // unreachable, so the deal must roll over
		s.Teams[0].HasMelded = true
		s.Teams[0].Melds = []Meld{{ID: meldID(0, "K"), TeamID: 0, Rank: "K",
			Cards: []string{"KH", "KD", "KS", "KC", "KH", "KD", "KS"}}}
		s.Hands["p1"] = []string{"8C"}
	})
	next, code := apply(t, raw, "p1", module.Action{Verb: VerbDiscard, Cards: []string{"8C"}})
	if code != "" {
		t.Fatalf("going out refused: %s", code)
	}
	s := mustDecode(t, next)
	if s.Status != "active" {
		t.Errorf("status is %q, want the match still running", s.Status)
	}
	if s.DealNumber != 1 {
		t.Errorf("deal number is %d, want 1", s.DealNumber)
	}
	if len(s.Hands["p1"]) != s.HandSize {
		t.Errorf("the next deal dealt %d cards, want %d", len(s.Hands["p1"]), s.HandSize)
	}
	if len(s.Teams[0].Melds) != 0 || s.Teams[0].HasMelded {
		t.Error("the new deal should start with a clean table")
	}
	if s.Teams[0].Score <= 0 {
		t.Errorf("the winning side scored %d for the deal", s.Teams[0].Score)
	}
	// The dealer moves, so the first-player advantage does not sit on one seat.
	if s.Current != "p1" {
		t.Errorf("second deal starts with %q; the deal should have rotated", s.Current)
	}
}

func TestFinishedReportsTheWinningPartnership(t *testing.T) {
	raw := fourHanded(func(s *GameState) {
		s.TargetScore = 100
		s.Teams[0].HasMelded = true
		s.Teams[0].Melds = []Meld{{ID: meldID(0, "K"), TeamID: 0, Rank: "K",
			Cards: []string{"KH", "KD", "KS", "KC", "KH", "KD", "KS"}}}
		s.Hands["p1"] = []string{"8C"}
	})
	next, code := apply(t, raw, "p1", module.Action{Verb: VerbDiscard, Cards: []string{"8C"}})
	if code != "" {
		t.Fatalf("going out refused: %s", code)
	}
	done, winners, err := New().Finished(next)
	if err != nil {
		t.Fatalf("Finished: %v", err)
	}
	if !done {
		t.Fatal("the match should be over")
	}
	// The whole partnership, not one of its two players. This used to return a
	// single id with a comment saying that was the wrong shape; Hold'em's split
	// pots made fixing it unavoidable, and the fix landed here first.
	if len(winners) != 2 {
		t.Fatalf("winners are %v, want both members of the winning partnership", winners)
	}
	got := map[string]bool{winners[0]: true, winners[1]: true}
	if !got["p1"] || !got["p3"] {
		t.Errorf("winners are %v, want p1 and p3", winners)
	}
	vm, err := New().View(next, "p1")
	if err != nil {
		t.Fatalf("View: %v", err)
	}
	var sawStandings bool
	for _, f := range vm.Status {
		if strings.HasPrefix(f.LabelKey, "status.teamScore") {
			sawStandings = true
		}
	}
	if !sawStandings {
		t.Error("the view should carry per-team standings, since Finished can only name one player")
	}
}

// TestNewMatch_DealerVariesBySeed guards the fix for a lobby's host always
// dealing first: before it, Dealer was always seat 0 (so seat 1 always
// opened). Now it is picked from the match seed via module.StartingSeat, and
// dealNew leads from Dealer+1 — so the seat that actually opens is exactly
// the seed's chosen seat.
func TestNewMatch_DealerVariesBySeed(t *testing.T) {
	players := []module.PlayerRef{{ID: "p1"}, {ID: "p2"}}
	seen := map[string]bool{}
	for seed := int64(0); seed < 40; seed++ {
		raw, err := New().NewMatch(module.MatchConfig{}, players, seed)
		if err != nil {
			t.Fatalf("seed %d: NewMatch: %v", seed, err)
		}
		s, err := decode(raw)
		if err != nil {
			t.Fatalf("seed %d: decode: %v", seed, err)
		}
		want := module.StartingSeat(seed, len(players))
		if s.Current != s.TurnOrder[want] {
			t.Fatalf("seed %d: opened on %q, want seat %d (%q)", seed, s.Current, want, s.TurnOrder[want])
		}
		seen[s.Current] = true
	}
	if len(seen) < 2 {
		t.Fatalf("40 seeds only ever opened on %v — the opening seat is not varying", seen)
	}
}
