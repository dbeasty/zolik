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

	t.Run("three jokers start it, and 2s join them past any ratio", func(t *testing.T) {
		raw := canastaXTable(func(s *GameState) {
			s.Hands["p1"] = []string{"JOKER1", "JOKER1", "JOKER2", "2C", "2D", "KS", "QS", "9D"}
		})
		raw, code := apply(t, raw, "p1", module.Action{Verb: VerbLayMeld, Cards: []string{"JOKER1", "JOKER1", "JOKER2"}})
		if code != "" {
			t.Fatalf("three jokers: %s", code)
		}
		s := mustDecode(t, raw)
		m := s.Teams[0].Melds[0]
		if m.Kind != meldWild || m.naturals() != 0 || m.wilds() != 3 {
			t.Fatalf("meld %+v", m)
		}
		if _, code := apply(t, raw, "p1", module.Action{Verb: VerbLayOff, Cards: []string{"2C", "2D"}, Target: m.ID}); code != "" {
			t.Fatalf("2s onto three jokers: %s", code)
		}
	})

	t.Run("a hand of three jokers is offered the meld", func(t *testing.T) {
		raw := canastaXTable(func(s *GameState) {
			s.Hands["p1"] = []string{"JOKER1", "JOKER1", "JOKER2", "KS", "QS", "9D"}
		})
		offers, err := New().LegalActions(raw, "p1")
		if err != nil {
			t.Fatal(err)
		}
		o := offerByID(offers, OfferLayMeld+":wild")
		if o == nil || !o.Enabled || len(o.Source.Submit) != 3 {
			t.Fatalf("no meld of wilds:\n%s", module.DescribeOffers(offers))
		}
	})

	// Three 2s used to fit the kings' offer as well as the meld of wilds, so
	// the Meld control could not settle on one and stayed grey. A group's
	// wilds now say they need a natural of the rank beside them.
	t.Run("a group's wilds ask for a natural", func(t *testing.T) {
		raw := canastaXTable(func(s *GameState) {
			s.Hands["p1"] = []string{"2D", "2H", "2S", "KC", "KD", "JOKER1", "9D"}
		})
		offers, err := New().LegalActions(raw, "p1")
		if err != nil {
			t.Fatal(err)
		}
		o := offerByID(offers, OfferLayMeld+":K")
		if o == nil {
			t.Fatalf("no kings:\n%s", module.DescribeOffers(offers))
		}
		for _, p := range o.Source.Placements {
			if !isWild(p.Card) {
				if len(p.Requires) != 0 {
					t.Errorf("natural %s needs company %v", p.Card, p.Requires)
				}
				continue
			}
			ways := append([][]string{p.Requires}, p.Alternatives...)
			if len(ways) != 2 || ways[0][0] != "KC" || ways[1][0] != "KD" {
				t.Errorf("wild %s: ways %v, want one king or the other", p.Card, ways)
			}
		}
		if len(o.Source.Placements) != len(o.Source.Cards) {
			t.Errorf("placements %d for %d cards", len(o.Source.Placements), len(o.Source.Cards))
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

	t.Run("two melds combine though no one card could move alone", func(t *testing.T) {
		raw := table(func(s *GameState) {
			s.Teams[0].Melds = append(s.Teams[0].Melds,
				Meld{ID: "t0-K-2", TeamID: 0, Kind: meldSet, Rank: "K", Cards: []string{"KH", "KD", "KS"}},
				Meld{ID: "t0-seq2", TeamID: 0, Kind: meldRun, Suit: "H", Cards: []string{"9H", "TH", "JH"}},
			)
		})
		offers, err := New().LegalActions(raw, "p1")
		if err != nil {
			t.Fatal(err)
		}
		requires := func(o *module.ActionOffer, card string) []string {
			for _, p := range o.Source.Placements {
				if p.Card == card {
					return p.Requires
				}
			}
			t.Fatalf("%s not placed by %+v", card, o.Source)
			return nil
		}
		kings := offerByID(offers, "move:t0-K-2:t0-K")
		if kings == nil || len(kings.Source.Cards) != 0 {
			t.Fatalf("kings offer %+v", kings)
		}
		if got := requires(kings, "KH"); !slices.Equal(got, []string{"KD", "KS"}) {
			t.Fatalf("KH requires %v", got)
		}
		run := offerByID(offers, "move:t0-seq2:t0-seq1")
		if run == nil {
			t.Fatal("no offer to join the runs")
		}
		if got := requires(run, "JH"); len(got) != 2 {
			t.Fatalf("JH requires %v", got)
		}
		// And what was offered goes through.
		raw, code := apply(t, raw, "p1", module.Action{
			OfferID: run.ID, Verb: VerbMoveCards, Cards: []string{"9H", "TH", "JH"}, Target: "t0-seq1",
		})
		if code != "" {
			t.Fatal(code)
		}
		if s := mustDecode(t, raw); len(s.Teams[0].meldByID("t0-seq1").Cards) != 7 || s.Teams[0].meldByID("t0-seq2") != nil {
			t.Fatalf("melds %+v", s.Teams[0].Melds)
		}
	})

	t.Run("a meld with duplicate cards moves whole, every copy listed", func(t *testing.T) {
		// The reported board: 4D 4D 4H and 4H 4H JOKER 4C, which together are
		// six natural fours and a joker. A client checks a selection against
		// the offer's cards as a multiset, so each copy must be listed.
		raw := table(func(s *GameState) {
			s.Teams[0].Melds = append(s.Teams[0].Melds,
				Meld{ID: "t0-4", TeamID: 0, Kind: meldSet, Rank: "4", Cards: []string{"4D", "4D", "4H"}},
				Meld{ID: "t0-4-2", TeamID: 0, Kind: meldSet, Rank: "4", Cards: []string{"4H", "4H", "JOKER1", "4C"}},
			)
		})
		offers, err := New().LegalActions(raw, "p1")
		if err != nil {
			t.Fatal(err)
		}
		listed := func(o *module.ActionOffer) []string {
			var out []string
			if len(o.Source.Placements) > 0 {
				for _, p := range o.Source.Placements {
					out = append(out, p.Card)
				}
			} else {
				out = append(out, o.Source.Cards...)
			}
			return sortedCards(out)
		}
		for _, tc := range []struct{ from, to string }{{"t0-4", "t0-4-2"}, {"t0-4-2", "t0-4"}} {
			o := offerByID(offers, "move:"+tc.from+":"+tc.to)
			if o == nil {
				t.Fatalf("no offer %s -> %s", tc.from, tc.to)
			}
			whole := sortedCards(mustDecode(t, raw).Teams[0].meldByID(tc.from).Cards)
			if got := listed(o); !slices.Equal(got, whole) {
				t.Fatalf("%s -> %s lists %v, the meld holds %v", tc.from, tc.to, got, whole)
			}
			after, code := apply(t, raw, "p1", module.Action{OfferID: o.ID, Verb: VerbMoveCards, Cards: whole, Target: tc.to})
			if code != "" {
				t.Fatalf("%s -> %s: %s", tc.from, tc.to, code)
			}
			if s := mustDecode(t, after); s.Teams[0].meldByID(tc.from) != nil || len(s.Teams[0].meldByID(tc.to).Cards) != 7 {
				t.Fatalf("melds %+v", s.Teams[0].Melds)
			}
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

// TestPoachOwnDirtyMeld: a side may buy the wild out of its own dirty meld,
// and the natural that would do it is offered both ways at once — joining
// the meld and buying its joker — so the client must ask which, rather than
// take whichever offer comes first (it laid the 8 off, every time).
func TestPoachOwnDirtyMeld(t *testing.T) {
	raw := canastaXTable(func(s *GameState) {
		s.Teams[0].Melds = []Meld{{ID: "t0-8", TeamID: 0, Kind: meldSet, Rank: "8", Cards: []string{"8H", "8D", "8S", "JOKER1"}}}
		s.Hands["p1"] = []string{"8C", "QD", "9D"}
	})
	offers, err := New().LegalActions(raw, "p1")
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"lay_off:t0-8", "poach:t0-8"} {
		o := offerByID(offers, id)
		if o == nil || !o.Enabled || o.Source == nil || !slices.Contains(o.Source.Cards, "8C") {
			t.Fatalf("%s: %+v", id, o)
		}
	}
	raw, code := apply(t, raw, "p1", module.Action{Verb: VerbPoach, Cards: []string{"8C"}, Target: "t0-8"})
	if code != "" {
		t.Fatal(code)
	}
	s := mustDecode(t, raw)
	if m := s.Teams[0].meldByID("t0-8"); m.wilds() != 0 || !slices.Contains(m.Cards, "8C") {
		t.Fatalf("meld %v", m.Cards)
	}
	if !slices.Contains(s.Hands["p1"], "JOKER1") || slices.Contains(s.Hands["p1"], "8C") {
		t.Fatalf("hand %v", s.Hands["p1"])
	}
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

// A natural that reaches a dirty sequence only across a gap is offered with a
// wild for the gap, and with the natural for it where the hand has both; a
// wild the 2:1 ratio refuses alone is offered with a natural beside it.
func TestLayOffAcrossAGap(t *testing.T) {
	placement := func(o *module.ActionOffer, card string) *module.Placement {
		for i, p := range o.Source.Placements {
			if p.Card == card {
				return &o.Source.Placements[i]
			}
		}
		return nil
	}
	run := func(low *int, cards ...string) Meld {
		return Meld{ID: "t0-seq1", TeamID: 0, Kind: meldRun, Suit: "H", Low: low, Cards: cards}
	}
	offerFor := func(t *testing.T, raw module.State) *module.ActionOffer {
		t.Helper()
		offers, err := New().LegalActions(raw, "p1")
		if err != nil {
			t.Fatal(err)
		}
		o := offerByID(offers, "lay_off:t0-seq1")
		if o == nil || !o.Enabled {
			t.Fatalf("offer %+v", o)
		}
		return o
	}

	t.Run("the nine with a joker for the eight", func(t *testing.T) {
		raw := canastaXTable(func(s *GameState) {
			s.Teams[0].Melds = []Meld{run(nil, "5H", "6H", "7H")}
			s.Hands["p1"] = []string{"9H", "JOKER1", "KS", "QS"}
		})
		p := placement(offerFor(t, raw), "9H")
		if p == nil || !slices.Equal(p.Requires, []string{"JOKER1"}) {
			t.Fatalf("9H placement %+v", p)
		}
		raw, code := apply(t, raw, "p1", module.Action{Verb: VerbLayOff, Cards: []string{"9H", "JOKER1"}, Target: "t0-seq1"})
		if code != "" {
			t.Fatal(code)
		}
		if m := mustDecode(t, raw).Teams[0].Melds[0]; !slices.Equal(m.Cards, []string{"5H", "6H", "7H", "JOKER1", "9H"}) {
			t.Fatalf("meld %+v", m)
		}
	})

	t.Run("the nine with the eight or a joker", func(t *testing.T) {
		raw := canastaXTable(func(s *GameState) {
			s.Teams[0].Melds = []Meld{run(nil, "5H", "6H", "7H")}
			s.Hands["p1"] = []string{"8H", "9H", "JOKER1", "KS", "QS"}
		})
		p := placement(offerFor(t, raw), "9H")
		if p == nil || !slices.Equal(p.Requires, []string{"8H"}) ||
			len(p.Alternatives) != 1 || !slices.Equal(p.Alternatives[0], []string{"JOKER1"}) {
			t.Fatalf("9H placement %+v", p)
		}
	})

	t.Run("a wild the ratio refuses alone goes on with a natural", func(t *testing.T) {
		raw := canastaXTable(func(s *GameState) {
			s.Teams[0].Melds = []Meld{run(lowAt(0), "4H", "5H", "2C", "7H")}
			s.Hands["p1"] = []string{"8H", "JOKER1", "KS", "QS"}
		})
		p := placement(offerFor(t, raw), "JOKER1")
		if p == nil || !slices.Equal(p.Requires, []string{"8H"}) {
			t.Fatalf("joker placement %+v", p)
		}
	})

	t.Run("a clean table's sequence is not bridged", func(t *testing.T) {
		raw := twoHanded(func(s *GameState) {
			r := variations["samba"]
			s.Variation = "samba"
			s.Rules = &r
			s.Phase = phaseMeld
			s.Teams[0].HasMelded = true
			s.Teams[0].Melds = []Meld{run(nil, "5H", "6H", "7H")}
			s.Hands["p1"] = []string{"8H", "9H", "JOKER1", "KS", "QS"}
		})
		if p := placement(offerFor(t, raw), "9H"); p == nil || len(p.Alternatives) != 0 {
			t.Fatalf("9H placement %+v", p)
		}
	})
}

// A dirty sequence may be laid with any wild in the hand standing in the gap:
// 9H-JOKER1-JH as well as 9H-2C-JH. Before opening, the offer spends the joker
// when the 2 would leave it short of the minimum; a wild goes only with the
// run's naturals, so wilds alone never look like this run.
func TestNewDirtyRunTakesAnyWild(t *testing.T) {
	for _, opened := range []bool{false, true} {
		raw := canastaXTable(func(s *GameState) {
			s.Teams[0].HasMelded = opened
			s.Teams[1].HasMelded = opened
			s.Hands["p1"] = []string{"9H", "2C", "JOKER1", "JH", "KS", "5D", "6C", "4C", "7S", "3D", "QC"}
		})
		offers, err := New().LegalActions(raw, "p1")
		if err != nil {
			t.Fatal(err)
		}
		var o *module.ActionOffer
		for i := range offers {
			if f := offers[i].Facts; offers[i].Verb == VerbLayMeld && len(f) > 0 && f[0].Value == "9H-JH" {
				o = &offers[i]
			}
		}
		if o == nil {
			t.Fatalf("opened=%v: no 9H-JH offer", opened)
		}
		if !opened && !slices.Equal(o.Source.Submit, []string{"9H", "JOKER1", "JH"}) {
			t.Fatalf("opening submit %v", o.Source.Submit)
		}
		byCard := map[string]module.Placement{}
		for _, p := range o.Source.Placements {
			byCard[p.Card] = p
		}
		for _, w := range []string{"2C", "JOKER1"} {
			if p, ok := byCard[w]; !ok || !slices.Equal(p.Requires, []string{"9H", "JH"}) {
				t.Fatalf("opened=%v: %s placement %+v in %+v", opened, w, p, o.Source.Placements)
			}
		}
		if _, code := apply(t, raw, "p1", module.Action{Verb: VerbLayMeld, Cards: []string{"9H", "JOKER1", "JH"}}); code != "" {
			t.Fatalf("opened=%v: 9H JOKER1 JH: %s", opened, code)
		}
		_, code := apply(t, raw, "p1", module.Action{Verb: VerbLayMeld, Cards: []string{"9H", "2C", "JH"}})
		if want := map[bool]string{false: ErrInitialMeldNotMet, true: ""}[opened]; code != want {
			t.Fatalf("opened=%v: 9H 2C JH: %q, want %q", opened, code, want)
		}
	}
}

// A hand that could lay a longer dirty sequence may still lay a shorter one:
// 9H-JOKER1-JH with the queen of hearts kept back.
func TestNewDirtyRunShorterThanTheHandAllows(t *testing.T) {
	raw := canastaXTable(func(s *GameState) {
		s.Hands["p1"] = []string{"9H", "JOKER1", "JH", "QH", "KS", "5D", "6C"}
	})
	offers, err := New().LegalActions(raw, "p1")
	if err != nil {
		t.Fatal(err)
	}
	var short, long *module.ActionOffer
	for i := range offers {
		if f := offers[i].Facts; offers[i].Verb == VerbLayMeld && len(f) > 0 {
			switch f[0].Value {
			case "9H-JH":
				short = &offers[i]
			case "9H-QH":
				long = &offers[i]
			}
		}
	}
	if short == nil || long == nil || short.ID == long.ID {
		t.Fatalf("short %+v long %+v", short, long)
	}
	if !slices.Equal(short.Source.Submit, []string{"9H", "JOKER1", "JH"}) {
		t.Fatalf("short submit %v", short.Source.Submit)
	}
}

// Match 6ae8f68a4c2fd2bc950e7dba: the engine took every one of these, and the
// offers named none of them, so the client refused the selection. A joker on
// the end of a gapless stretch, 5C-6C-7C-JOKER; a joker onto 8S-9S-2C with the
// six and seven, which the 2:1 ratio needs both of; and onto 9C-TC-JC-JOKER2,
// where nothing goes on alone, the seven with the joker for the eight.
func TestCanastaXLaysTheEngineTakes(t *testing.T) {
	raw := canastaXTable(func(s *GameState) {
		s.Teams[0].Melds = []Meld{
			{ID: "t0-seq9", TeamID: 0, Kind: meldRun, Suit: "C", Cards: []string{"9C", "TC", "JC", "JOKER2"}},
			{ID: "t0-seq10", TeamID: 0, Kind: meldRun, Suit: "S", Cards: []string{"8S", "9S", "2C"}},
		}
		s.Hands["p1"] = []string{"4C", "5C", "6C", "7C", "6S", "7S", "4H", "5H", "6H", "7H", "JOKER1", "2H", "KH", "9D"}
	})
	offers, err := New().LegalActions(raw, "p1")
	if err != nil {
		t.Fatal(err)
	}
	// covers is the client's own test (drops.ts fits): every card listed, and
	// every card that needs company holding one of its companies in full.
	covers := func(o *module.ActionOffer, cards []string) bool {
		listed := map[string]module.Placement{}
		for _, p := range o.Source.Placements {
			listed[p.Card] = p
		}
		for _, c := range o.Source.Cards {
			if _, ok := listed[c]; !ok {
				listed[c] = module.Placement{Card: c}
			}
		}
		for _, c := range cards {
			p, ok := listed[c]
			if !ok {
				return false
			}
			if len(p.Requires) == 0 {
				continue
			}
			held := false
			for _, way := range append([][]string{p.Requires}, p.Alternatives...) {
				held = held || !slices.ContainsFunc(way, func(r string) bool { return !slices.Contains(cards, r) })
			}
			if !held {
				return false
			}
		}
		return true
	}
	offered := func(a module.Action) bool {
		for i := range offers {
			o := &offers[i]
			if o.Verb != a.Verb || !o.Enabled || o.Source == nil {
				continue
			}
			if a.Verb == VerbLayOff && o.Target.MeldID != a.Target {
				continue
			}
			if covers(o, a.Cards) {
				return true
			}
		}
		return false
	}
	for _, a := range []module.Action{
		{Verb: VerbLayMeld, Cards: []string{"5C", "6C", "7C", "JOKER1"}},
		{Verb: VerbLayMeld, Cards: []string{"4C", "5C", "6C", "7C", "JOKER1"}},
		{Verb: VerbLayMeld, Cards: []string{"5H", "6H", "7H", "JOKER1"}},
		{Verb: VerbLayMeld, Cards: []string{"4H", "5H", "6H", "7H", "JOKER1"}},
		{Verb: VerbLayOff, Cards: []string{"6S", "7S", "JOKER1"}, Target: "t0-seq10"},
		{Verb: VerbLayOff, Cards: []string{"7C", "JOKER1"}, Target: "t0-seq9"},
		{Verb: VerbLayOff, Cards: []string{"6C", "7C", "JOKER1"}, Target: "t0-seq9"},
		{Verb: VerbLayOff, Cards: []string{"7C", "2H"}, Target: "t0-seq9"},
	} {
		if _, code := apply(t, raw, "p1", a); code != "" {
			t.Fatalf("%s %v: engine refused %s", a.Verb, a.Cards, code)
		}
		if !offered(a) {
			t.Errorf("%s %v onto %q: the engine takes it, no offer covers it", a.Verb, a.Cards, a.Target)
		}
	}
	// And what the ratio refuses is still not offered: a lone joker onto a
	// run of two naturals, or with only the seven beside it.
	for _, a := range []module.Action{
		{Verb: VerbLayOff, Cards: []string{"JOKER1"}, Target: "t0-seq10"},
		{Verb: VerbLayOff, Cards: []string{"7S", "JOKER1"}, Target: "t0-seq10"},
		{Verb: VerbLayOff, Cards: []string{"JOKER1"}, Target: "t0-seq9"},
	} {
		if offered(a) {
			t.Errorf("%s %v onto %q: offered, though the engine refuses it", a.Verb, a.Cards, a.Target)
		}
	}
}
