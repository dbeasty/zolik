package canasta

import (
	"slices"
	"testing"

	"zolik/server/internal/module"
)

func canastaXRules() ruleset { return variations[variationCanastaX] }

// canastaXTable is a two-handed CanastaX table with p1 to play, already drawn,
// and both sides opened unless mutate says otherwise.
func canastaXTable(mutate func(s *GameState)) module.State {
	return twoHanded(func(s *GameState) {
		r := canastaXRules()
		s.Variation = variationCanastaX
		s.Rules = &r
		s.HandSize = r.HandSize
		s.TargetScore = r.TargetScore
		s.CanastasToGoOut = r.CanastasToGoOut
		s.Phase = phaseMeld
		s.Teams[0].HasMelded = true
		s.Teams[1].HasMelded = true
		if mutate != nil {
			mutate(s)
		}
	})
}

func lowAt(i int) *int { return &i }

func TestArrangeRun(t *testing.T) {
	r := canastaXRules()
	cases := []struct {
		name    string
		cards   []string
		wantLow int
		out     []string
		low     string
		err     string
	}{
		{"clean", []string{"7H", "5H", "6H"}, noLow, []string{"5H", "6H", "7H"}, "5", ""},
		{"a wild fills the gap", []string{"5H", "JOKER1", "7H", "8H"}, noLow, []string{"5H", "6H", "JOKER1", "8H"}[:0], "", ""},
		{"a spare wild goes on top", []string{"5H", "6H", "2C"}, noLow, []string{"5H", "6H", "2C"}, "5", ""},
		{"unless asked to start lower", []string{"5H", "6H", "2C"}, 0, []string{"2C", "5H", "6H"}, "4", ""},
		{"at the ace it goes below", []string{"KH", "AH", "2C"}, noLow, []string{"2C", "KH", "AH"}, "Q", ""},
		{"one wild needs two naturals", []string{"5H", "2C", "2D"}, noLow, nil, "", ErrNotEnoughNaturals},
		{"two gaps, one wild", []string{"5H", "7H", "9H", "2C"}, noLow, nil, "", ErrRunNotConsecutive},
		{"one suit", []string{"5H", "6S", "2C"}, noLow, nil, "", ErrSequenceNeedsOneSuit},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, low, err := arrangeRun(r, nil, 0, tc.cards, tc.wantLow)
			if tc.err != "" {
				if module.CodeOf(err) != tc.err {
					t.Fatalf("err = %v, want %s", err, tc.err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected %v", err)
			}
			if tc.name == "a wild fills the gap" {
				if !slices.Equal(out, []string{"5H", "JOKER1", "7H", "8H"}) || runRanks[low] != "5" {
					t.Fatalf("got %v from %s", out, runRanks[low])
				}
				return
			}
			if !slices.Equal(out, tc.out) || runRanks[low] != tc.low {
				t.Fatalf("got %v from %s, want %v from %s", out, runRanks[low], tc.out, tc.low)
			}
		})
	}

	t.Run("no wilds where the table has not switched them on", func(t *testing.T) {
		_, _, err := arrangeRun(sambaRules(), nil, 0, []string{"5H", "6H", "2C"}, noLow)
		if module.CodeOf(err) != ErrSequenceNoWilds {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestDirtySequences(t *testing.T) {
	t.Run("laid with a wild in the gap, the wild keeps its place", func(t *testing.T) {
		raw := canastaXTable(func(s *GameState) {
			s.Hands["p1"] = []string{"5H", "7H", "JOKER1", "KS", "QS"}
		})
		raw, code := apply(t, raw, "p1", module.Action{Verb: VerbLayMeld, Cards: []string{"5H", "7H", "JOKER1"}})
		if code != "" {
			t.Fatal(code)
		}
		m := mustDecode(t, raw).Teams[0].Melds[0]
		if m.kind() != meldRun || !slices.Equal(m.Cards, []string{"5H", "JOKER1", "7H"}) || m.standsFor(1) != "6H" {
			t.Fatalf("meld %+v", m)
		}
	})

	t.Run("a seven-card sequence with a wild is a dirty samba, worth 700", func(t *testing.T) {
		team := &Team{Melds: []Meld{{Kind: meldRun, Suit: "H", Low: lowAt(0),
			Cards: []string{"4H", "5H", "JOKER1", "7H", "8H", "9H", "TH"}}}}
		if got := canastaScore(canastaXRules(), team); got != 700 {
			t.Fatalf("scored %d", got)
		}
		if canastaKind(team.Melds[0]) != "dirtySamba" {
			t.Fatal(canastaKind(team.Melds[0]))
		}
	})

	t.Run("a wild laid off goes on top", func(t *testing.T) {
		raw := canastaXTable(func(s *GameState) {
			s.Teams[0].Melds = []Meld{{ID: "t0-seq1", TeamID: 0, Kind: meldRun, Suit: "H", Cards: []string{"5H", "6H", "7H", "8H"}}}
			s.Hands["p1"] = []string{"2C", "KS", "QS"}
		})
		raw, code := apply(t, raw, "p1", module.Action{Verb: VerbLayOff, Cards: []string{"2C"}, Target: "t0-seq1"})
		if code != "" {
			t.Fatal(code)
		}
		m := mustDecode(t, raw).Teams[0].Melds[0]
		if !slices.Equal(m.Cards, []string{"5H", "6H", "7H", "8H", "2C"}) || m.standsFor(4) != "9H" {
			t.Fatalf("meld %+v", m)
		}
		// And it comes back off exactly.
		raw, code = apply(t, raw, "p1", module.Action{Verb: VerbUndoLayOff})
		if code != "" {
			t.Fatal(code)
		}
		if m := mustDecode(t, raw).Teams[0].Melds[0]; m.Low != nil || len(m.Cards) != 4 {
			t.Fatalf("after undo %+v", m)
		}
	})
}

func TestWildMeld(t *testing.T) {
	t.Run("only once the side has opened", func(t *testing.T) {
		raw := canastaXTable(func(s *GameState) {
			s.Teams[0].HasMelded = false
			s.Hands["p1"] = []string{"2C", "2D", "2H", "KS", "QS"}
		})
		if _, code := apply(t, raw, "p1", module.Action{Verb: VerbLayMeld, Cards: []string{"2C", "2D", "2H"}}); code != ErrMustMeldFirst {
			t.Fatalf("code %q", code)
		}
	})

	t.Run("laid, one per side, jokers by the ratio", func(t *testing.T) {
		raw := canastaXTable(func(s *GameState) {
			s.Hands["p1"] = []string{"2C", "2D", "2H", "JOKER1", "2S", "KS", "QS", "9D"}
		})
		raw, code := apply(t, raw, "p1", module.Action{Verb: VerbLayMeld, Cards: []string{"2C", "2D", "2H"}})
		if code != "" {
			t.Fatal(code)
		}
		s := mustDecode(t, raw)
		m := s.Teams[0].Melds[0]
		if m.Kind != meldWild || m.naturals() != 3 || m.wilds() != 0 {
			t.Fatalf("meld %+v", m)
		}
		raw, code = apply(t, raw, "p1", module.Action{Verb: VerbLayOff, Cards: []string{"JOKER1"}, Target: m.ID})
		if code != "" {
			t.Fatalf("joker onto three 2s: %s", code)
		}
		if _, code := apply(t, raw, "p1", module.Action{Verb: VerbLayOff, Cards: []string{"KS"}, Target: m.ID}); code != ErrWrongRank {
			t.Fatalf("king onto the 2s: %q", code)
		}
	})

	t.Run("seven 2s pay 2250, with jokers 1500, and both count to go out", func(t *testing.T) {
		r := canastaXRules()
		clean := Meld{Kind: meldWild, Rank: rankTwo, Cards: []string{"2C", "2D", "2H", "2S", "2C", "2D", "2H"}}
		dirty := Meld{Kind: meldWild, Rank: rankTwo, Cards: []string{"2C", "2D", "2H", "2S", "2C", "JOKER1", "2H"}}
		if got := canastaScore(r, &Team{Melds: []Meld{clean}}); got != 2250 {
			t.Errorf("clean %d", got)
		}
		if got := canastaScore(r, &Team{Melds: []Meld{dirty}}); got != 1500 {
			t.Errorf("with a joker %d", got)
		}
		if (&Team{Melds: []Meld{clean, dirty}}).canastas() != 2 {
			t.Error("both are canastas")
		}
	})
}

func TestMoveCards(t *testing.T) {
	table := func(mutate func(s *GameState)) module.State {
		return canastaXTable(func(s *GameState) {
			s.Teams[0].Melds = []Meld{
				{ID: "t0-K", TeamID: 0, Kind: meldSet, Rank: "K", Cards: []string{"KH", "KD", "KS", "KC", "2C"}},
				{ID: "t0-seq1", TeamID: 0, Kind: meldRun, Suit: "H", Cards: []string{"5H", "6H", "7H", "8H"}},
			}
			s.Hands["p1"] = []string{"9S", "QS", "JD"}
			if mutate != nil {
				mutate(s)
			}
		})
	}

	t.Run("a wild moves from a group onto a sequence", func(t *testing.T) {
		raw, code := apply(t, table(nil), "p1", module.Action{
			Verb: VerbMoveCards, Cards: []string{"2C"}, Target: "t0-seq1", Params: map[string]string{"from": "t0-K"},
		})
		if code != "" {
			t.Fatal(code)
		}
		s := mustDecode(t, raw)
		if k := s.Teams[0].meldByID("t0-K"); !k.isNatural() || len(k.Cards) != 4 {
			t.Fatalf("group %+v", k)
		}
		if run := s.Teams[0].meldByID("t0-seq1"); run.standsFor(4) != "9H" || run.Cards[4] != "2C" {
			t.Fatalf("run %+v", run)
		}
		// And back.
		raw, code = apply(t, raw, "p1", module.Action{Verb: VerbUndoReshape})
		if code != "" {
			t.Fatal(code)
		}
		if k := mustDecode(t, raw).Teams[0].meldByID("t0-K"); len(k.Cards) != 5 {
			t.Fatalf("after undo %+v", k)
		}
	})

	t.Run("never out of the middle of a sequence", func(t *testing.T) {
		raw := table(func(s *GameState) {
			s.Teams[0].Melds = append(s.Teams[0].Melds, Meld{ID: "t0-6", TeamID: 0, Kind: meldSet, Rank: "6", Cards: []string{"6S", "6D", "6C"}})
		})
		_, code := apply(t, raw, "p1", module.Action{
			Verb: VerbMoveCards, Cards: []string{"6H"}, Target: "t0-6", Params: map[string]string{"from": "t0-seq1"},
		})
		if code != ErrRunGap {
			t.Fatalf("code %q", code)
		}
	})

	t.Run("a meld moved out entirely is gone, and an undo brings it back in place", func(t *testing.T) {
		raw := table(func(s *GameState) {
			s.Teams[0].Melds = append(s.Teams[0].Melds[:1], Meld{ID: "t0-K-2", TeamID: 0, Kind: meldSet, Rank: "K", Cards: []string{"KH", "KD", "KS"}}, s.Teams[0].Melds[1])
		})
		raw, code := apply(t, raw, "p1", module.Action{
			Verb: VerbMoveCards, Cards: []string{"KH", "KD", "KS"}, Target: "t0-K", Params: map[string]string{"from": "t0-K-2"},
		})
		if code != "" {
			t.Fatal(code)
		}
		if s := mustDecode(t, raw); s.Teams[0].meldByID("t0-K-2") != nil || len(s.Teams[0].meldByID("t0-K").Cards) != 8 {
			t.Fatalf("melds %+v", s.Teams[0].Melds)
		}
		raw, code = apply(t, raw, "p1", module.Action{Verb: VerbUndoReshape})
		if code != "" {
			t.Fatal(code)
		}
		if ids := meldIDs(mustDecode(t, raw).Teams[0].Melds); !slices.Equal(ids, []string{"t0-K", "t0-K-2", "t0-seq1"}) {
			t.Fatalf("order after undo %v", ids)
		}
	})

	t.Run("a canasta may be broken, unless the side needs it to finish the turn", func(t *testing.T) {
		canasta := Meld{ID: "t0-Q", TeamID: 0, Kind: meldSet, Rank: "Q", Cards: []string{"QH", "QD", "QS", "QC", "QH", "QD", "2D"}}
		raw := table(func(s *GameState) {
			s.CanastasToGoOut = 1
			s.Rules.CanastasToGoOut = 1
			s.Teams[0].Melds = append(s.Teams[0].Melds, canasta)
		})
		if _, code := apply(t, raw, "p1", module.Action{
			Verb: VerbMoveCards, Cards: []string{"2D"}, Target: "t0-seq1", Params: map[string]string{"from": "t0-Q"},
		}); code != "" {
			t.Fatalf("with cards to spare: %q", code)
		}
		raw = table(func(s *GameState) {
			s.CanastasToGoOut = 1
			s.Rules.CanastasToGoOut = 1
			s.Teams[0].Melds = append(s.Teams[0].Melds, canasta)
			s.Hands["p1"] = []string{"9S"}
		})
		if _, code := apply(t, raw, "p1", module.Action{
			Verb: VerbMoveCards, Cards: []string{"QH"}, Target: "t0-K", Params: map[string]string{"from": "t0-Q"},
		}); code == "" {
			t.Fatal("broke the only canasta while holding the last card")
		}
	})

	t.Run("not before opening, and not onto the other side", func(t *testing.T) {
		raw := table(func(s *GameState) { s.Teams[0].HasMelded = false })
		if _, code := apply(t, raw, "p1", module.Action{
			Verb: VerbMoveCards, Cards: []string{"2C"}, Target: "t0-seq1", Params: map[string]string{"from": "t0-K"},
		}); code != ErrMustMeldFirst {
			t.Fatalf("code %q", code)
		}
		raw = table(func(s *GameState) {
			s.Teams[1].Melds = []Meld{{ID: "t1-9", TeamID: 1, Kind: meldSet, Rank: "9", Cards: []string{"9H", "9D", "9C"}}}
		})
		if _, code := apply(t, raw, "p1", module.Action{
			Verb: VerbMoveCards, Cards: []string{"2C"}, Target: "t1-9", Params: map[string]string{"from": "t0-K"},
		}); code != ErrNotYourMeld {
			t.Fatalf("code %q", code)
		}
	})

	t.Run("offered per pair of melds", func(t *testing.T) {
		offers, err := New().LegalActions(table(nil), "p1")
		if err != nil {
			t.Fatal(err)
		}
		o := offerByID(offers, "move:t0-K:t0-seq1")
		if o == nil || !o.Enabled || !slices.Contains(o.Source.Cards, "2C") {
			t.Fatalf("offer %+v", o)
		}
	})
}

func TestPoach(t *testing.T) {
	table := func(mutate func(s *GameState)) module.State {
		return canastaXTable(func(s *GameState) {
			s.Teams[1].Melds = []Meld{
				{ID: "t1-K", TeamID: 1, Kind: meldSet, Rank: "K", Cards: []string{"KH", "KD", "2C", "KS", "KC", "KD", "KH"}},
				{ID: "t1-seq1", TeamID: 1, Kind: meldRun, Suit: "S", Low: lowAt(1), Cards: []string{"5S", "JOKER1", "7S", "8S"}},
				{ID: "t1-wild", TeamID: 1, Kind: meldWild, Rank: rankTwo, Cards: []string{"2D", "2H", "JOKER2", "2S"}},
			}
			s.Hands["p1"] = []string{"KS", "6S", "2H", "9D", "QD"}
			if mutate != nil {
				mutate(s)
			}
		})
	}

	t.Run("a king buys the 2 out of their mixed canasta, which turns natural", func(t *testing.T) {
		raw, code := apply(t, table(nil), "p1", module.Action{Verb: VerbPoach, Cards: []string{"KS"}, Target: "t1-K"})
		if code != "" {
			t.Fatal(code)
		}
		s := mustDecode(t, raw)
		if !slices.Contains(s.Hands["p1"], "2C") || slices.Contains(s.Hands["p1"], "KS") {
			t.Fatalf("hand %v", s.Hands["p1"])
		}
		if k := s.Teams[1].meldByID("t1-K"); canastaKind(*k) != "natural" {
			t.Fatalf("their canasta is %s", canastaKind(*k))
		}
	})

	t.Run("a sequence's wild only for the card it stands for", func(t *testing.T) {
		raw, code := apply(t, table(nil), "p1", module.Action{Verb: VerbPoach, Cards: []string{"6S"}, Target: "t1-seq1"})
		if code != "" {
			t.Fatal(code)
		}
		if run := mustDecode(t, raw).Teams[1].meldByID("t1-seq1"); !run.isNatural() || run.Low != nil || run.runLow() != 1 {
			t.Fatalf("run %+v", run)
		}
		if _, code := apply(t, table(func(s *GameState) { s.Hands["p1"] = []string{"9S", "QD", "9D"} }), "p1",
			module.Action{Verb: VerbPoach, Cards: []string{"9S"}, Target: "t1-seq1"}); code != ErrNothingToPoach {
			t.Fatalf("9S for the 6S's joker: %q", code)
		}
	})

	t.Run("a 2 buys the joker out of a meld of 2s", func(t *testing.T) {
		raw, code := apply(t, table(nil), "p1", module.Action{Verb: VerbPoach, Cards: []string{"2H"}, Target: "t1-wild"})
		if code != "" {
			t.Fatal(code)
		}
		s := mustDecode(t, raw)
		if !slices.Contains(s.Hands["p1"], "JOKER2") || s.Teams[1].meldByID("t1-wild").wilds() != 0 {
			t.Fatalf("hand %v", s.Hands["p1"])
		}
		// And undone, the joker goes back.
		raw, code = apply(t, raw, "p1", module.Action{Verb: VerbUndoReshape})
		if code != "" {
			t.Fatal(code)
		}
		if s := mustDecode(t, raw); slices.Contains(s.Hands["p1"], "JOKER2") || s.Teams[1].meldByID("t1-wild").wilds() != 1 {
			t.Fatalf("after undo %v", s.Hands["p1"])
		}
	})

	t.Run("offered on the opponents' melds", func(t *testing.T) {
		offers, err := New().LegalActions(table(nil), "p1")
		if err != nil {
			t.Fatal(err)
		}
		for _, id := range []string{"poach:t1-K", "poach:t1-seq1", "poach:t1-wild"} {
			if o := offerByID(offers, id); o == nil || !o.Enabled {
				t.Errorf("%s: %+v", id, o)
			}
		}
	})
}

func TestUndoOrderWithReshapes(t *testing.T) {
	raw := canastaXTable(func(s *GameState) {
		s.Teams[0].Melds = []Meld{
			{ID: "t0-K", TeamID: 0, Kind: meldSet, Rank: "K", Cards: []string{"KH", "KD", "KS", "KC", "2C"}},
			{ID: "t0-seq1", TeamID: 0, Kind: meldRun, Suit: "H", Cards: []string{"5H", "6H", "7H", "8H"}},
		}
		s.Hands["p1"] = []string{"KS", "QS", "JD", "9D"}
	})
	raw, code := apply(t, raw, "p1", module.Action{Verb: VerbLayOff, Cards: []string{"KS"}, Target: "t0-K"})
	if code != "" {
		t.Fatal(code)
	}
	raw, code = apply(t, raw, "p1", module.Action{
		Verb: VerbMoveCards, Cards: []string{"2C"}, Target: "t0-seq1", Params: map[string]string{"from": "t0-K"},
	})
	if code != "" {
		t.Fatal(code)
	}
	if _, code := apply(t, raw, "p1", module.Action{Verb: VerbUndoLayOff}); code != ErrUndoLatestFirst && code != ErrNothingToUndo {
		t.Fatalf("lay-off undone from under a move: %q", code)
	}
	raw, code = apply(t, raw, "p1", module.Action{Verb: VerbUndoReshape})
	if code != "" {
		t.Fatal(code)
	}
	if _, code = apply(t, raw, "p1", module.Action{Verb: VerbUndoLayOff}); code != "" {
		t.Fatalf("lay-off after the move came back: %q", code)
	}
}

func TestTopOnlyCapture(t *testing.T) {
	raw := canastaXTable(func(s *GameState) {
		s.Phase = phaseDraw
		s.DiscardPile = []string{"4C", "9D", "TS", "QH"}
		s.Hands["p1"] = []string{"QD", "QS", "5C", "7D"}
	})
	offers, err := New().LegalActions(raw, "p1")
	if err != nil {
		t.Fatal(err)
	}
	o := offerByID(offers, "take_pile:naturals:top")
	if o == nil || !o.Enabled || o.Verb != VerbTakePileTop {
		t.Fatalf("offer %+v", o)
	}
	raw, code := apply(t, raw, "p1", module.Action{Verb: VerbTakePileTop, Cards: []string{"QD", "QS"}})
	if code != "" {
		t.Fatal(code)
	}
	s := mustDecode(t, raw)
	if !slices.Equal(s.DiscardPile, []string{"4C", "9D", "TS"}) || len(s.Hands["p1"]) != 2 {
		t.Fatalf("pile %v hand %v", s.DiscardPile, s.Hands["p1"])
	}
	raw, code = apply(t, raw, "p1", module.Action{Verb: VerbUndoTakePile})
	if code != "" {
		t.Fatal(code)
	}
	if s := mustDecode(t, raw); len(s.DiscardPile) != 4 || len(s.Hands["p1"]) != 4 {
		t.Fatalf("after undo pile %v hand %v", s.DiscardPile, s.Hands["p1"])
	}
}

// The house rules are options: on in CanastaX, off everywhere else, and a
// table may switch any of them on.
func TestHouseRulesAreOptions(t *testing.T) {
	if r := resolveRules(module.MatchConfig{Variation: "samba"}, 2); r.Poach || r.Rearrange || r.WildMeld || r.DirtySequences || r.TopOnlyCapture {
		t.Fatalf("samba %+v", r)
	}
	r := resolveRules(module.MatchConfig{Variation: "classic", Options: module.Options{OptWildMeld: module.OptOn, OptDirtySequences: module.OptOn}}, 2)
	if !r.WildMeld || r.WildCanastaBonus != 2250 {
		t.Fatalf("classic with the 2s meld %+v", r)
	}
	if r.DirtySequences {
		t.Fatal("dirty sequences need a variation with sequences")
	}
	x := resolveRules(module.MatchConfig{Variation: variationCanastaX}, 4)
	if !x.Poach || !x.Rearrange || !x.WildMeld || !x.DirtySequences || !x.TopOnlyCapture {
		t.Fatalf("canastax %+v", x)
	}
}

func meldIDs(ms []Meld) []string {
	var out []string
	for _, m := range ms {
		out = append(out, m.ID)
	}
	return out
}

// CanastaX is still offer-driven: a driver that reads nothing but offers plays
// it to the end at every seat count, and the house moves are among what it
// finds to do. Moves are left to the random sweep (dead_end_sweep_test.go): a
// driver that always prefers one would carry a card back and forth forever.
func TestCanastaXIsOfferDriven(t *testing.T) {
	m := New()
	verbs := map[string]int{}
	for seats := 2; seats <= 6; seats += 2 {
		players := refs(seatNames(seats)...)
		for seed := int64(1); seed <= 4; seed++ {
			cfg := module.MatchConfig{Variation: variationCanastaX, Options: module.Options{OptTargetScore: 1000}}
			state, err := m.NewMatch(cfg, players, seed)
			if err != nil {
				t.Fatalf("NewMatch: %v", err)
			}
			_, res, err := module.PlayWithOffers(m, state, players, module.DriverOptions{
				MaxActions: 8000, Prefer: append([]string{VerbPoach, VerbTakePileTop}, driverPrefer...),
			})
			if err != nil {
				t.Fatalf("%d seats seed %d: %v", seats, seed, err)
			}
			if !res.Finished {
				t.Fatalf("%d seats seed %d: did not finish in %d actions (verbs=%v)", seats, seed, res.Actions, res.Verbs)
			}
			for v, n := range res.Verbs {
				verbs[v] += n
			}
		}
	}
	for _, v := range []string{VerbPoach, VerbTakePileTop} {
		if verbs[v] == 0 {
			t.Errorf("no %s in any match (verbs=%v)", v, verbs)
		}
	}
}

// A wild that could go at either end of a dirty sequence is offered at both,
// and lands where it is let go.
func TestWildAtEitherEnd(t *testing.T) {
	raw := canastaXTable(func(s *GameState) {
		s.Teams[0].Melds = []Meld{{ID: "t0-seq1", TeamID: 0, Kind: meldRun, Suit: "H", Cards: []string{"6H", "7H", "8H", "9H"}}}
		s.Hands["p1"] = []string{"2C", "KS", "QS"}
	})
	offers, err := New().LegalActions(raw, "p1")
	if err != nil {
		t.Fatal(err)
	}
	o := offerByID(offers, "lay_off:t0-seq1")
	if o == nil || len(o.Source.Placements) != 1 || !slices.Equal(o.Source.Placements[0].Positions, []string{"front", "end"}) {
		t.Fatalf("offer %+v", o)
	}
	raw, code := apply(t, raw, "p1", module.Action{Verb: VerbLayOff, Cards: []string{"2C"}, Target: "t0-seq1",
		Params: map[string]string{module.PositionParam: "front"}})
	if code != "" {
		t.Fatal(code)
	}
	if m := mustDecode(t, raw).Teams[0].Melds[0]; m.Cards[0] != "2C" || m.standsFor(0) != "5H" {
		t.Fatalf("meld %+v", m)
	}

	// At the ace there is only one end, and no choice to offer.
	raw = canastaXTable(func(s *GameState) {
		s.Teams[0].Melds = []Meld{{ID: "t0-seq1", TeamID: 0, Kind: meldRun, Suit: "H", Cards: []string{"JH", "QH", "KH", "AH"}}}
		s.Hands["p1"] = []string{"2C", "KS", "QS"}
	})
	offers, _ = New().LegalActions(raw, "p1")
	if o := offerByID(offers, "lay_off:t0-seq1"); o == nil || len(o.Source.Placements) != 0 {
		t.Fatalf("offer at the ace %+v", o)
	}
}

// A bot moves a card between its own melds only to finish a canasta.
func TestBotFinishesACanastaByMoving(t *testing.T) {
	raw := canastaXTable(func(s *GameState) {
		s.Teams[0].Melds = []Meld{
			{ID: "t0-K", TeamID: 0, Kind: meldSet, Rank: "K", Cards: []string{"KH", "KD", "KS", "KC", "KH", "KD"}},
			{ID: "t0-9", TeamID: 0, Kind: meldSet, Rank: "9", Cards: []string{"9C", "9D", "9S", "9H", "2C"}},
		}
		s.Hands["p1"] = []string{"5S", "7D", "TC"}
	})
	offers, err := New().LegalActions(raw, "p1")
	if err != nil {
		t.Fatal(err)
	}
	a, ok := bot{}.Act(raw, module.BotSeat{PlayerID: "p1", Skill: module.SkillMedium}, offers)
	if !ok || a.Verb != VerbMoveCards || a.Target != "t0-K" || a.Cards[0] != "2C" {
		t.Fatalf("bot chose %+v", a)
	}
}
