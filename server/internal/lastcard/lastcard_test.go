package lastcard

import (
	"encoding/json"
	"testing"

	"zolik/server/internal/module"
)

func players(ids ...string) []module.PlayerRef {
	out := make([]module.PlayerRef, 0, len(ids))
	for _, id := range ids {
		out = append(out, module.PlayerRef{ID: id, Name: id})
	}
	return out
}

func newGame(t *testing.T, seed int64, ids ...string) module.State {
	t.Helper()
	st, err := New().NewMatch(module.MatchConfig{}, players(ids...), seed)
	if err != nil {
		t.Fatalf("NewMatch: %v", err)
	}
	return st
}

// stateOf decodes for assertions. Tests may look inside; the runtime may not.
func stateOf(t *testing.T, raw module.State) *GameState {
	t.Helper()
	s, err := decode(raw)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	return s
}

// withState builds a three-player table, p1 to move, play going p1→p2→p3.
func withState(t *testing.T, mut func(*GameState)) module.State {
	t.Helper()
	s := &GameState{
		Status:      "active",
		Players:     []string{"p1", "p2", "p3"},
		TurnOrder:   []string{"p1", "p2", "p3"},
		Current:     "p1",
		Direction:   1,
		DrawPile:    []string{"C-1", "C-2", "C-3", "C-4", "C-5", "C-6"},
		DiscardPile: []string{"T-5"},
		Hands: map[string][]string{
			"p1": {"T-9", "V-2", "A-7"},
			"p2": {"C-8", "V-3", "A-1"},
			"p3": {"T-1", "T-2", "V-9"},
		},
		Seed: 1,
	}
	mut(s)
	raw, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return raw
}

func apply(t *testing.T, raw module.State, pid string, a module.Action) module.State {
	t.Helper()
	out, _, err := New().Apply(raw, pid, a)
	if err != nil {
		t.Fatalf("%s %s %v: %v", pid, a.Verb, a.Cards, err)
	}
	return out
}

func refused(t *testing.T, raw module.State, pid string, a module.Action, code string) {
	t.Helper()
	_, _, err := New().Apply(raw, pid, a)
	if got := module.CodeOf(err); got != code {
		t.Fatalf("%s %s %v: got %q, want %q", pid, a.Verb, a.Cards, got, code)
	}
}

func playCard(card string) module.Action { return playAction(card) }

func playWild(card, colour string) module.Action {
	return module.Action{Verb: VerbPlay, Cards: []string{card}, Params: map[string]string{"colour": colour}}
}

func TestDeckIs108CardsInTheRightProportions(t *testing.T) {
	deck := buildDeck()
	if len(deck) != 108 {
		t.Fatalf("deck has %d cards, want 108", len(deck))
	}
	n := map[string]int{}
	for _, c := range deck {
		n[c]++
	}
	for _, c := range colours {
		if n[c+"-0"] != 1 {
			t.Errorf("%s-0 appears %d times, want 1", c, n[c+"-0"])
		}
		for _, f := range []string{"1", "9", faceSkip, faceReverse, faceDrawTwo} {
			if n[c+"-"+f] != 2 {
				t.Errorf("%s-%s appears %d times, want 2", c, f, n[c+"-"+f])
			}
		}
	}
	if n[cardWild] != 4 || n[cardWildDrawFour] != 4 {
		t.Errorf("wilds %d, wild draw fours %d, want 4 and 4", n[cardWild], n[cardWildDrawFour])
	}
}

func TestNewMatchDealsSevenAndNeverOpensOnAWild(t *testing.T) {
	for seed := int64(1); seed <= 200; seed++ {
		s := stateOf(t, newGame(t, seed, "p1", "p2", "p3", "p4"))
		total := len(s.DrawPile) + len(s.DiscardPile)
		for _, h := range s.Hands {
			total += len(h)
		}
		if total != 108 {
			t.Fatalf("seed %d: %d cards accounted for, want 108", seed, total)
		}
		if isWild(s.top()) {
			t.Fatalf("seed %d: opened on a wild", seed)
		}
		hands := 0
		for _, h := range s.Hands {
			if len(h) == defaultHandSize {
				hands++
			}
		}
		// Every hand is seven, except one that took an opening Draw Two.
		if hands < 3 {
			t.Fatalf("seed %d: only %d hands of %d", seed, hands, defaultHandSize)
		}
	}
}

func TestOpeningActionCardsTakeEffect(t *testing.T) {
	seen := map[string]bool{}
	for seed := int64(1); seed <= 400; seed++ {
		raw := newGame(t, seed, "p1", "p2", "p3")
		s := stateOf(t, raw)
		starter := s.TurnOrder[module.StartingSeat(seed, 3)]
		switch faceOf(s.top()) {
		case faceSkip:
			seen["skip"] = true
			if s.Current == starter {
				t.Errorf("seed %d: an opening Skip did not skip %s", seed, starter)
			}
		case faceDrawTwo:
			seen["draw"] = true
			if len(s.Hands[starter]) != defaultHandSize+2 || s.Current == starter {
				t.Errorf("seed %d: opening Draw Two left %s with %d cards, to move=%v",
					seed, starter, len(s.Hands[starter]), s.Current == starter)
			}
		case faceReverse:
			seen["reverse"] = true
			if s.Direction != -1 {
				t.Errorf("seed %d: an opening Reverse did not turn play round", seed)
			}
		}
	}
	for _, k := range []string{"skip", "draw", "reverse"} {
		if !seen[k] {
			t.Errorf("no seed opened on a %s — the test is vacuous for it", k)
		}
	}
}

func TestMatchingByColourNumberOrSymbol(t *testing.T) {
	raw := withState(t, func(s *GameState) {
		s.DiscardPile = []string{"T-5"}
		s.Hands["p1"] = []string{"T-9", "V-5", "A-7", "C-S"}
	})
	apply(t, raw, "p1", playCard("T-9")) // colour
	apply(t, raw, "p1", playCard("V-5")) // number
	refused(t, raw, "p1", playCard("A-7"), ErrCardDoesNotMatch)

	raw = withState(t, func(s *GameState) {
		s.DiscardPile = []string{"T-S"}
		s.Hands["p1"] = []string{"C-S", "A-7"}
	})
	apply(t, raw, "p1", playCard("C-S")) // symbol
}

func TestAWildOnTopIsAnsweredByTheNamedColourOnly(t *testing.T) {
	raw := withState(t, func(s *GameState) {
		s.DiscardPile = []string{"T-5", cardWild}
		s.DeclaredColour = "V"
		s.Hands["p1"] = []string{"T-5", "V-2", cardWild}
	})
	refused(t, raw, "p1", playCard("T-5"), ErrCardDoesNotMatch)
	apply(t, raw, "p1", playCard("V-2"))
	apply(t, raw, "p1", playWild(cardWild, "A")) // a wild still goes on anything
}

func TestAWildMustNameAColour(t *testing.T) {
	raw := withState(t, func(s *GameState) { s.Hands["p1"] = []string{cardWild, "A-7"} })
	refused(t, raw, "p1", module.Action{Verb: VerbPlay, Cards: []string{cardWild}}, ErrColourRequired)
	refused(t, raw, "p1", playWild(cardWild, "H"), ErrUnknownColour)
	s := stateOf(t, apply(t, raw, "p1", playWild(cardWild, "A")))
	if s.DeclaredColour != "A" || s.colourInPlay() != "A" {
		t.Errorf("declared %q, in play %q, want A", s.DeclaredColour, s.colourInPlay())
	}
}

func TestWildDrawFourOnlyWithoutTheColourInPlay(t *testing.T) {
	raw := withState(t, func(s *GameState) {
		s.DiscardPile = []string{"T-5"}
		s.Hands["p1"] = []string{cardWildDrawFour, "T-9", "A-7"}
	})
	refused(t, raw, "p1", playWild(cardWildDrawFour, "A"), ErrDrawFourHeld)

	// Holding a 5 of another colour is fine: the rule is about colour only.
	raw = withState(t, func(s *GameState) {
		s.DiscardPile = []string{"T-5"}
		s.Hands["p1"] = []string{cardWildDrawFour, "V-5", "A-7"}
	})
	s := stateOf(t, apply(t, raw, "p1", playWild(cardWildDrawFour, "A")))
	if len(s.Hands["p2"]) != 7 {
		t.Errorf("p2 holds %d, want 3+4", len(s.Hands["p2"]))
	}
	if s.Current != "p3" {
		t.Errorf("to move %s, want p3 (p2 drew and lost the turn)", s.Current)
	}
}

func TestSkipDrawTwoAndReverse(t *testing.T) {
	// Skip.
	raw := withState(t, func(s *GameState) { s.Hands["p1"] = []string{"T-S", "A-7"} })
	if s := stateOf(t, apply(t, raw, "p1", playCard("T-S"))); s.Current != "p3" {
		t.Errorf("after a Skip, %s to move, want p3", s.Current)
	}
	// Draw Two.
	raw = withState(t, func(s *GameState) { s.Hands["p1"] = []string{"T-D", "A-7"} })
	s := stateOf(t, apply(t, raw, "p1", playCard("T-D")))
	if s.Current != "p3" || len(s.Hands["p2"]) != 5 {
		t.Errorf("after a Draw Two: %s to move, p2 holds %d; want p3 and 5", s.Current, len(s.Hands["p2"]))
	}
	// Reverse, three players: back to p3, and on round from there.
	raw = withState(t, func(s *GameState) { s.Hands["p1"] = []string{"T-R", "A-7"} })
	s = stateOf(t, apply(t, raw, "p1", playCard("T-R")))
	if s.Current != "p3" || s.Direction != -1 {
		t.Errorf("after a Reverse: %s to move, direction %d; want p3 and -1", s.Current, s.Direction)
	}
	if got := s.nextPlayer("p3"); got != "p2" {
		t.Errorf("after a Reverse p3 is followed by %s, want p2", got)
	}
}

func TestReverseBetweenTwoPlayersIsASkip(t *testing.T) {
	raw := withState(t, func(s *GameState) {
		s.Players = []string{"p1", "p2"}
		s.TurnOrder = []string{"p1", "p2"}
		delete(s.Hands, "p3")
		s.Hands["p1"] = []string{"T-R", "A-7"}
	})
	if s := stateOf(t, apply(t, raw, "p1", playCard("T-R"))); s.Current != "p1" {
		t.Errorf("after a two-player Reverse %s is to move, want p1 again", s.Current)
	}
}

func TestDrawingAPlayableCardLeavesItToPlayOrKeep(t *testing.T) {
	raw := withState(t, func(s *GameState) {
		s.Hands["p1"] = []string{"A-7", "V-2"}
		s.DrawPile = []string{"T-1"} // drawn from the end
	})
	raw = apply(t, raw, "p1", module.Action{Verb: VerbDraw})
	s := stateOf(t, raw)
	if s.Current != "p1" || s.DrawnCard != "T-1" {
		t.Fatalf("after drawing a playable card: %s to move, drawn %q", s.Current, s.DrawnCard)
	}
	refused(t, raw, "p1", module.Action{Verb: VerbDraw}, ErrAlreadyDrew)
	refused(t, raw, "p1", playCard("V-2"), ErrOnlyDrawnCard)

	kept := stateOf(t, apply(t, raw, "p1", module.Action{Verb: VerbPass}))
	if kept.Current != "p2" || kept.DrawnCard != "" || len(kept.Hands["p1"]) != 3 {
		t.Errorf("keeping it: %s to move, drawn %q, %d cards", kept.Current, kept.DrawnCard, len(kept.Hands["p1"]))
	}
	played := stateOf(t, apply(t, raw, "p1", playCard("T-1")))
	if played.top() != "T-1" || played.Current != "p2" {
		t.Errorf("playing it: top %s, %s to move", played.top(), played.Current)
	}
}

func TestDrawingAnUnplayableCardEndsTheTurn(t *testing.T) {
	raw := withState(t, func(s *GameState) {
		s.Hands["p1"] = []string{"A-7"}
		s.DrawPile = []string{"C-1"}
	})
	s := stateOf(t, apply(t, raw, "p1", module.Action{Verb: VerbDraw}))
	if s.Current != "p2" || s.DrawnCard != "" {
		t.Errorf("%s to move, drawn %q; want p2 and nothing held open", s.Current, s.DrawnCard)
	}
	refused(t, raw, "p1", module.Action{Verb: VerbPass}, ErrNothingToKeep)
}

func TestTheLastCardWinsAndStillBites(t *testing.T) {
	raw := withState(t, func(s *GameState) { s.Hands["p1"] = []string{"T-D"} })
	s := stateOf(t, apply(t, raw, "p1", playCard("T-D")))
	if s.Status != "completed" || s.WinnerID != "p1" {
		t.Fatalf("status %s winner %s", s.Status, s.WinnerID)
	}
	if len(s.Hands["p2"]) != 5 {
		t.Errorf("p2 holds %d, want 5 — the Draw Two still applies", len(s.Hands["p2"]))
	}
	if done, w, _ := New().Finished(apply(t, raw, "p1", playCard("T-D"))); !done || w[0] != "p1" {
		t.Errorf("Finished = %v %v", done, w)
	}
}

func TestEmptyPilesReshuffleTheDiscards(t *testing.T) {
	raw := withState(t, func(s *GameState) {
		s.DrawPile = nil
		s.DiscardPile = []string{"C-1", "C-2", "T-5"}
		s.Hands["p1"] = []string{"A-7"}
	})
	s := stateOf(t, apply(t, raw, "p1", module.Action{Verb: VerbDraw}))
	if s.top() != "T-5" || len(s.DiscardPile) != 1 || s.Reshuffles != 1 {
		t.Errorf("after reshuffle: top %s, pile %d, reshuffles %d", s.top(), len(s.DiscardPile), s.Reshuffles)
	}
}

// With nothing anywhere to draw and nobody able to play, the deal must still
// end — a table stranded with no legal exit is the one outcome not allowed.
func TestAStrandedTableEndsOnTheFewestCards(t *testing.T) {
	raw := withState(t, func(s *GameState) {
		s.DrawPile = nil
		s.DiscardPile = []string{"T-5"}
		s.Hands = map[string][]string{"p1": {"A-7", "A-8"}, "p2": {"C-8"}, "p3": {"V-1", "V-2", "V-3"}}
	})
	for _, p := range []string{"p1", "p2", "p3"} {
		raw = apply(t, raw, p, module.Action{Verb: VerbDraw})
	}
	s := stateOf(t, raw)
	if s.Status != "completed" || s.WinnerID != "p2" {
		t.Errorf("status %s, winner %s; want completed and p2", s.Status, s.WinnerID)
	}
}

func TestOnlyTheDrawerSeesTheDrawnCardPrompt(t *testing.T) {
	raw := withState(t, func(s *GameState) {
		s.Hands["p1"] = []string{"A-7", "T-1"}
		s.DrawnCard = "T-1"
	})
	for _, viewer := range []string{"p1", "p2"} {
		vm, err := New().View(raw, viewer)
		if err != nil {
			t.Fatal(err)
		}
		if got := len(vm.Prompts) > 0; got != (viewer == "p1") {
			t.Errorf("%s sees the drawn-card prompt: %v", viewer, got)
		}
	}
}
