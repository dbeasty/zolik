package canasta

import (
	"testing"

	"zolik/server/internal/module"
)

func sambaRules() ruleset { return variations["samba"] }

// The deck is the thing every other Samba number is derived from, so it is
// worth pinning on its own: 162 cards, six jokers, six red threes.
func TestSambaDeck(t *testing.T) {
	deck := buildDeck(sambaRules())
	if len(deck) != 162 {
		t.Errorf("deck is %d cards, want 162", len(deck))
	}

	jokers, reds := 0, 0
	for _, c := range deck {
		if rankOf(c) == rankJoker {
			jokers++
		}
		if isRedThree(c) {
			reds++
		}
	}
	if jokers != 6 {
		t.Errorf("deck has %d jokers, want 6", jokers)
	}
	if reds != 6 {
		t.Errorf("deck has %d red threes, want 6", reds)
	}
	if got := sambaRules().redThrees(); got != reds {
		t.Errorf("ruleset says %d red threes, the deck holds %d", got, reds)
	}
}

// Samba's numbers, each stated once here so a change to any of them has to come
// past a test that says what it was.
func TestSambaScoring(t *testing.T) {
	r := sambaRules()

	t.Run("a seven-card sequence is a samba, worth 1500", func(t *testing.T) {
		team := &Team{Melds: []Meld{{
			Kind: meldRun, Suit: "H",
			Cards: []string{"4H", "5H", "6H", "7H", "8H", "9H", "TH"},
		}}}
		if got := canastaScore(r, team); got != 1500 {
			t.Errorf("samba scored %d, want 1500", got)
		}
		if !team.Melds[0].isCanasta() {
			t.Error("a seven-card sequence should count toward going out")
		}
		if !team.Melds[0].closed(r) {
			t.Error("a samba is complete at seven and takes no eighth card")
		}
	})

	t.Run("a group canasta is 500 or 300 and stays open", func(t *testing.T) {
		natural := &Team{Melds: []Meld{{
			Kind: meldSet, Rank: "K",
			Cards: []string{"KH", "KD", "KS", "KC", "KH", "KD", "KS"},
		}}}
		if got := canastaScore(r, natural); got != 500 {
			t.Errorf("natural canasta scored %d, want 500", got)
		}
		if natural.Melds[0].closed(r) {
			t.Error("a Samba group canasta keeps taking cards")
		}

		mixed := &Team{Melds: []Meld{{
			Kind: meldSet, Rank: "K",
			Cards: []string{"KH", "KD", "KS", "KC", "KH", "KD", "JOKER1"},
		}}}
		if got := canastaScore(r, mixed); got != 300 {
			t.Errorf("mixed canasta scored %d, want 300", got)
		}
	})

	t.Run("the initial meld floor gains a fourth band", func(t *testing.T) {
		cases := []struct{ score, want int }{
			{-100, 15}, {0, 50}, {1499, 50}, {1500, 90}, {2999, 90},
			{3000, 120}, {6999, 120}, {7000, 150}, {12000, 150},
		}
		for _, tc := range cases {
			if got := r.meldFloor(tc.score); got != tc.want {
				t.Errorf("meldFloor(%d) = %d, want %d", tc.score, got, tc.want)
			}
		}
	})

	t.Run("red threes need the go-out canastas, and cost a flat 100 short of them", func(t *testing.T) {
		six := []string{"3H", "3D", "3H", "3D", "3H", "3D"}

		// Two canastas is what Samba asks for before red threes count up, and
		// one is not enough — the difference from Canasta, where one is.
		short := &Team{RedThrees: six, Melds: []Meld{sevenOf("K")}}
		if got := redThreeScore(r, short); got != -600 {
			t.Errorf("one canasta and six red threes scored %d, want -600", got)
		}

		enough := &Team{RedThrees: six, Melds: []Meld{sevenOf("K"), sevenOf("Q")}}
		if got := redThreeScore(r, enough); got != 1000 {
			t.Errorf("all six red threes scored %d, want 1000", got)
		}

		some := &Team{RedThrees: six[:3], Melds: []Meld{sevenOf("K"), sevenOf("Q")}}
		if got := redThreeScore(r, some); got != 300 {
			t.Errorf("three red threes scored %d, want 300", got)
		}
	})

	t.Run("going out is 200, and there is no concealed bonus", func(t *testing.T) {
		if r.GoingOutBonus != 200 {
			t.Errorf("going out pays %d, want 200", r.GoingOutBonus)
		}
		if r.ConcealedBonus != 0 {
			t.Errorf("Samba has no concealed bonus, but one is declared as %d", r.ConcealedBonus)
		}
	})
}

// Canasta's own numbers, restated beside Samba's, because the whole point of a
// ruleset is that the two do not have to agree.
func TestCanastaKeepsItsOwnNumbers(t *testing.T) {
	r := classicRules()

	four := []string{"3H", "3D", "3H", "3D"}
	// One canasta is enough in Canasta, and the penalty short of it negates the
	// all-four bonus rather than charging a flat hundred each.
	if got := redThreeScore(r, &Team{RedThrees: four}); got != -800 {
		t.Errorf("four red threes and no canasta scored %d, want -800", got)
	}
	if got := redThreeScore(r, &Team{RedThrees: four, Melds: []Meld{sevenOf("K")}}); got != 800 {
		t.Errorf("four red threes and a canasta scored %d, want 800", got)
	}
	if r.meldFloor(12000) != 120 {
		t.Errorf("Canasta's top floor is %d, want 120 — the 150 band is Samba's", r.meldFloor(12000))
	}
	if r.ConcealedBonus != 200 {
		t.Errorf("Canasta's concealed bonus is %d, want 200", r.ConcealedBonus)
	}
}

// Samba's wild limits are a ratio as well as a cap, which is what makes a
// two-wild group need six cards rather than four.
func TestSambaWildLimits(t *testing.T) {
	r := sambaRules()
	cases := []struct {
		name  string
		cards []string
		want  string
	}{
		{"three naturals", []string{"KH", "KD", "KS"}, ""},
		{"one wild against two naturals", []string{"KH", "KD", "JOKER1"}, ""},
		{"two wilds against four naturals", []string{"KH", "KD", "KS", "KC", "JOKER1", "2H"}, ""},
		{"a third wild is too many", []string{"KH", "KD", "KS", "KC", "JOKER1", "2H", "2D"}, ErrTooManyWilds},
		{"two wilds against two naturals breaks the ratio", []string{"KH", "KD", "JOKER1", "2H"}, ErrNotEnoughNaturals},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := module.CodeOf(validateGroup(r, tc.cards)); got != tc.want {
				t.Errorf("validateGroup(%v) = %q, want %q", tc.cards, got, tc.want)
			}
		})
	}

	// Canasta's own limit is three wilds and two naturals, with no ratio.
	if err := validateGroup(classicRules(), []string{"KH", "KD", "JOKER1", "2H"}); err != nil {
		t.Errorf("classic refused two wilds against two naturals: %v", err)
	}
}

// Samba lets a side keep more than one group of a rank; Canasta does not.
func TestSecondGroupOfARank(t *testing.T) {
	samba, classic := sambaRules(), classicRules()
	team := &Team{Melds: []Meld{sevenOf("K")}}

	if team.rankIsFull(samba, "K") {
		t.Error("Samba should allow a second group of kings")
	}
	if !team.rankIsFull(classic, "K") {
		t.Error("Canasta allows only one group of a rank")
	}

	// And the ids stay apart, which is what a client diffs on.
	first := team.newMeldID(meldSet, "Q")
	team.Melds = append(team.Melds, Meld{ID: first, Kind: meldSet, Rank: "Q"})
	if second := team.newMeldID(meldSet, "Q"); second == first {
		t.Errorf("two groups of queens both got the id %q", first)
	}
}

// sambaTable is twoHanded, dealt under Samba's rules.
func sambaTable(mutate func(s *GameState)) module.State {
	return twoHanded(func(s *GameState) {
		r := sambaRules()
		s.Variation = "samba"
		s.Rules = &r
		s.HandSize = r.HandSize
		s.TargetScore = r.TargetScore
		s.CanastasToGoOut = r.CanastasToGoOut
		if mutate != nil {
			mutate(s)
		}
	})
}

// A Samba turn draws two, and a red three among them is laid down and replaced
// rather than counted as one of the two.
func TestSambaDrawsTwo(t *testing.T) {
	t.Run("two cards reach the hand", func(t *testing.T) {
		raw := sambaTable(func(s *GameState) {
			s.Hands["p1"] = []string{"KH", "KD"}
			s.DrawPile = []string{"5C", "6C", "7C"}
		})
		next, code := apply(t, raw, "p1", module.Action{Verb: VerbDraw})
		if code != "" {
			t.Fatalf("draw refused: %s", code)
		}
		s := mustDecode(t, next)
		if got := len(s.Hands["p1"]); got != 4 {
			t.Errorf("hand is %d cards after drawing, want 4", got)
		}
		if got := len(s.DrawPile); got != 1 {
			t.Errorf("stock is %d cards, want 1", got)
		}
	})

	t.Run("a red three is replaced, not counted", func(t *testing.T) {
		raw := sambaTable(func(s *GameState) {
			s.Hands["p1"] = []string{"KH", "KD"}
			// Drawn from the end: 7C, then the red three, then 6C.
			s.DrawPile = []string{"5C", "6C", "3H", "7C"}
		})
		next, code := apply(t, raw, "p1", module.Action{Verb: VerbDraw})
		if code != "" {
			t.Fatalf("draw refused: %s", code)
		}
		s := mustDecode(t, next)
		if got := len(s.Hands["p1"]); got != 4 {
			t.Errorf("hand is %d cards, want 4 — the red three should have been replaced", got)
		}
		if got := len(s.Teams[0].RedThrees); got != 1 {
			t.Errorf("side has %d red threes down, want 1", got)
		}
	})

	t.Run("the stock running out mid-draw ends the deal", func(t *testing.T) {
		raw := sambaTable(func(s *GameState) {
			s.Hands["p1"] = []string{"KH", "KD"}
			s.DrawPile = []string{"7C"}
		})
		next, code := apply(t, raw, "p1", module.Action{Verb: VerbDraw})
		if code != "" {
			t.Fatalf("draw refused: %s", code)
		}
		if s := mustDecode(t, next); s.LastDeal == nil {
			t.Error("the deal should have ended when the stock ran out mid-draw")
		}
	})
}

// Samba's pile is frozen against everyone, all deal, so the two cheap captures
// Canasta allows are gone even for a side that has opened.
func TestSambaPileIsAlwaysFrozen(t *testing.T) {
	withOpenTable := func(s *GameState) {
		s.Phase = phaseDraw
		s.DiscardPile = []string{"4D", "KC"}
		s.Teams[0].HasMelded = true
		s.Teams[0].Melds = []Meld{{
			ID: "t0-K", Kind: meldSet, Rank: "K", Cards: []string{"KH", "KD", "KS"},
		}}
	}

	t.Run("a natural and a wild will not do it", func(t *testing.T) {
		raw := sambaTable(func(s *GameState) {
			withOpenTable(s)
			s.Hands["p1"] = []string{"KH", "JOKER1", "5S"}
		})
		if _, code := apply(t, raw, "p1", module.Action{
			Verb: VerbTakePile, Cards: []string{"KH", "JOKER1"},
		}); code != ErrPileFrozen {
			t.Errorf("capture with a wild was %q, want %s", code, ErrPileFrozen)
		}
	})

	t.Run("nor laying the top card onto a group", func(t *testing.T) {
		raw := sambaTable(func(s *GameState) {
			withOpenTable(s)
			s.Hands["p1"] = []string{"5S", "6S", "7S"}
		})
		if _, code := apply(t, raw, "p1", module.Action{
			Verb: VerbTakePile, Target: "t0-K",
		}); code != ErrPileFrozen {
			t.Errorf("capture onto a meld was %q, want %s", code, ErrPileFrozen)
		}
	})

	t.Run("two naturals from hand still do", func(t *testing.T) {
		raw := sambaTable(func(s *GameState) {
			withOpenTable(s)
			s.Hands["p1"] = []string{"KH", "KD", "5S"}
		})
		next, code := apply(t, raw, "p1", module.Action{
			Verb: VerbTakePile, Cards: []string{"KH", "KD"},
		})
		if code != "" {
			t.Fatalf("capture with two naturals refused: %s", code)
		}
		if s := mustDecode(t, next); len(s.DiscardPile) != 0 {
			t.Errorf("the pile still has %d cards after being taken", len(s.DiscardPile))
		}
	})
}

// The other way in: one card onto a sequence, and the pile stays standing.
func TestSambaTakeTop(t *testing.T) {
	table := func(mutate func(s *GameState)) module.State {
		return sambaTable(func(s *GameState) {
			s.Phase = phaseDraw
			s.DiscardPile = []string{"KC", "8H"}
			s.Teams[0].HasMelded = true
			s.Teams[0].MeldSeq = 1
			s.Teams[0].Melds = []Meld{{
				ID: "t0-seq1", Kind: meldRun, Suit: "H",
				Cards: []string{"5H", "6H", "7H"},
			}}
			s.Hands["p1"] = []string{"QS", "QD", "4C"}
			if mutate != nil {
				mutate(s)
			}
		})
	}

	t.Run("takes one card and leaves the rest", func(t *testing.T) {
		next, code := apply(t, table(nil), "p1", module.Action{
			Verb: VerbTakeTop, Target: "t0-seq1",
		})
		if code != "" {
			t.Fatalf("take_top refused: %s", code)
		}
		s := mustDecode(t, next)
		if len(s.DiscardPile) != 1 || s.DiscardPile[0] != "KC" {
			t.Errorf("pile is %v, want just the king of clubs left standing", s.DiscardPile)
		}
		if got := s.Teams[0].Melds[0].Cards; len(got) != 4 || got[3] != "8H" {
			t.Errorf("run is %v, want the eight of hearts on the end", got)
		}
		if len(s.Hands["p1"]) != 3 {
			t.Errorf("hand is %d cards — take_top puts nothing in it", len(s.Hands["p1"]))
		}
		if s.Phase != phaseMeld {
			t.Errorf("phase is %q, want %q — it replaces the draw", s.Phase, phaseMeld)
		}
	})

	t.Run("a card that does not continue the run is refused", func(t *testing.T) {
		raw := table(func(s *GameState) { s.DiscardPile = []string{"KC", "TH"} })
		if _, code := apply(t, raw, "p1", module.Action{
			Verb: VerbTakeTop, Target: "t0-seq1",
		}); code != ErrRunNotConsecutive {
			t.Errorf("taking the ten onto 5-6-7 was %q, want %s", code, ErrRunNotConsecutive)
		}
	})

	t.Run("nor one in another suit", func(t *testing.T) {
		raw := table(func(s *GameState) { s.DiscardPile = []string{"KC", "8S"} })
		if _, code := apply(t, raw, "p1", module.Action{
			Verb: VerbTakeTop, Target: "t0-seq1",
		}); code != ErrSequenceNeedsOneSuit {
			t.Errorf("taking the eight of spades was %q, want %s", code, ErrSequenceNeedsOneSuit)
		}
	})

	// The move that takes without giving: a player holding one card who took the
	// top would then have to discard it, which is going out, which their side
	// cannot do. Refused here rather than discovered at the discard.
	t.Run("refused when it would strand the player", func(t *testing.T) {
		raw := table(func(s *GameState) { s.Hands["p1"] = []string{"QS"} })
		if _, code := apply(t, raw, "p1", module.Action{
			Verb: VerbTakeTop, Target: "t0-seq1",
		}); code != ErrMustKeepACard {
			t.Errorf("take_top on a one-card hand was %q, want %s", code, ErrMustKeepACard)
		}
	})

	t.Run("classic has never heard of it", func(t *testing.T) {
		raw := twoHanded(func(s *GameState) {
			s.Phase = phaseDraw
			s.DiscardPile = []string{"8H"}
			s.Teams[0].Melds = []Meld{{ID: "t0-K", Rank: "K", Cards: []string{"KH", "KD", "KS"}}}
		})
		if _, code := apply(t, raw, "p1", module.Action{
			Verb: VerbTakeTop, Target: "t0-K",
		}); code != ErrUnknownAction {
			t.Errorf("classic answered take_top with %q, want %s", code, ErrUnknownAction)
		}
	})
}

// Six seats is three partnerships sitting opposite; five is five sides.
func TestSambaSeating(t *testing.T) {
	m := New()
	cases := []struct {
		seats int
		teams int
		pairs [][]string
	}{
		{2, 2, [][]string{{"p1"}, {"p2"}}},
		{3, 3, [][]string{{"p1"}, {"p2"}, {"p3"}}},
		{4, 2, [][]string{{"p1", "p3"}, {"p2", "p4"}}},
		{5, 5, nil},
		{6, 3, [][]string{{"p1", "p4"}, {"p2", "p5"}, {"p3", "p6"}}},
	}
	for _, tc := range cases {
		raw, err := m.NewMatch(module.MatchConfig{Variation: "samba"}, refs(seatNames(tc.seats)...), 7)
		if err != nil {
			t.Fatalf("%d seats: %v", tc.seats, err)
		}
		s := mustDecode(t, raw)
		if len(s.Teams) != tc.teams {
			t.Errorf("%d seats made %d sides, want %d", tc.seats, len(s.Teams), tc.teams)
		}
		for i, want := range tc.pairs {
			got := s.Teams[i].Players
			if len(got) != len(want) {
				t.Errorf("%d seats: side %d is %v, want %v", tc.seats, i, got, want)
				continue
			}
			for j := range want {
				if got[j] != want[j] {
					t.Errorf("%d seats: side %d is %v, want %v", tc.seats, i, got, want)
					break
				}
			}
		}
	}

	if _, err := m.NewMatch(module.MatchConfig{Variation: "samba"}, refs(seatNames(6)...), 7); err != nil {
		t.Errorf("six seats refused: %v", err)
	}
	seven := append(seatNames(6), "p7")
	if _, err := m.NewMatch(module.MatchConfig{Variation: "samba"}, refs(seven...), 7); err == nil {
		t.Error("a seven-seat samba should be refused")
	}
	if _, err := m.NewMatch(module.MatchConfig{Variation: "classic"}, refs(seatNames(5)...), 7); err == nil {
		t.Error("a five-seat classic should be refused — classic seats four")
	}
}

// Six seats deal thirteen; everything below deals fifteen.
func TestSambaHandSizeAtSixSeats(t *testing.T) {
	m := New()
	for _, tc := range []struct{ seats, want int }{{2, 15}, {5, 15}, {6, 13}} {
		raw, err := m.NewMatch(module.MatchConfig{Variation: "samba"}, refs(seatNames(tc.seats)...), 3)
		if err != nil {
			t.Fatalf("%d seats: %v", tc.seats, err)
		}
		if got := mustDecode(t, raw).HandSize; got != tc.want {
			t.Errorf("%d seats dealt %d cards, want %d", tc.seats, got, tc.want)
		}
	}

	// Still an option a table can override, not a constant.
	raw, err := m.NewMatch(module.MatchConfig{
		Variation: "samba", Options: module.Options{OptHandSize: 15},
	}, refs(seatNames(6)...), 3)
	if err != nil {
		t.Fatalf("six seats with handSize 15: %v", err)
	}
	if got := mustDecode(t, raw).HandSize; got != 15 {
		t.Errorf("an explicit handSize of 15 dealt %d", got)
	}
}

func sevenOf(rank string) Meld {
	m := Meld{Kind: meldSet, Rank: rank}
	suits := []string{"H", "D", "S", "C", "H", "D", "S"}
	for _, s := range suits {
		m.Cards = append(m.Cards, rank+s)
	}
	return m
}
