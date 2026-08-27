package prsi

import (
	"encoding/json"
	"testing"

	"zolik/server/internal/module"
)

func newGame(t *testing.T, seed int64, playerIDs ...string) module.State {
	t.Helper()
	players := make([]module.PlayerRef, 0, len(playerIDs))
	for _, id := range playerIDs {
		players = append(players, module.PlayerRef{ID: id, Name: id})
	}
	st, err := New().NewMatch(module.MatchConfig{}, players, seed)
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

func withState(t *testing.T, mut func(*GameState)) module.State {
	t.Helper()
	s := &GameState{
		Status:      "active",
		Players:     []string{"p1", "p2"},
		TurnOrder:   []string{"p1", "p2"},
		Current:     "p1",
		DrawPile:    []string{"8H", "9H", "TH"},
		DiscardPile: []string{"9S"},
		Hands:       map[string][]string{"p1": {"9D", "KS"}, "p2": {"7C", "AD"}},
		Seed:        1,
	}
	mut(s)
	raw, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return raw
}

func TestNewMatch_DealsAndTurnsAStarter(t *testing.T) {
	s := stateOf(t, newGame(t, 42, "p1", "p2", "p3"))

	if len(s.Hands) != 3 {
		t.Fatalf("expected 3 hands, got %d", len(s.Hands))
	}
	for id, h := range s.Hands {
		if len(h) != defaultHandSize {
			t.Errorf("%s holds %d cards, want %d", id, len(h), defaultHandSize)
		}
	}
	if len(s.DiscardPile) != 1 {
		t.Errorf("expected exactly one card turned up, got %d", len(s.DiscardPile))
	}
	// Every card accounted for: 32 total, none duplicated or lost.
	seen := map[string]int{}
	for _, h := range s.Hands {
		for _, c := range h {
			seen[c]++
		}
	}
	for _, c := range append(append([]string{}, s.DrawPile...), s.DiscardPile...) {
		seen[c]++
	}
	if len(seen) != 32 {
		t.Errorf("deck has %d distinct cards, want 32", len(seen))
	}
	for c, n := range seen {
		if n != 1 {
			t.Errorf("card %s appears %d times", c, n)
		}
	}
}

func TestNewMatch_NeverStartsOnAWild(t *testing.T) {
	// A wild on the opening card would leave nobody having named a suit.
	// Checked across many seeds because it only bites on the ones where a
	// Queen happens to come up first.
	for seed := int64(0); seed < 200; seed++ {
		s := stateOf(t, newGame(t, seed, "p1", "p2"))
		if rankOf(s.top()) == rankWild {
			t.Fatalf("seed %d opened on a wild (%s)", seed, s.top())
		}
	}
}

func TestPlay_MatchesSuitOrRank(t *testing.T) {
	m := New()
	// Top is 9S. 9D matches by rank; KS would match by suit.
	raw := withState(t, func(s *GameState) {})
	if _, _, err := m.Apply(raw, "p1", module.Action{Verb: VerbPlay, Cards: []string{"9D"}}); err != nil {
		t.Errorf("9D matches 9S by rank: %v", err)
	}
	if _, _, err := m.Apply(raw, "p1", module.Action{Verb: VerbPlay, Cards: []string{"KS"}}); err != nil {
		t.Errorf("KS matches 9S by suit: %v", err)
	}
	// A card matching neither is refused.
	raw2 := withState(t, func(s *GameState) { s.Hands["p1"] = []string{"8H"} })
	_, _, err := m.Apply(raw2, "p1", module.Action{Verb: VerbPlay, Cards: []string{"8H"}})
	if module.CodeOf(err) != ErrCardDoesNotFit {
		t.Errorf("8H matches neither 9 nor S; got %v", err)
	}
}

func TestSeven_ObligesTheNextPlayerAndStacks(t *testing.T) {
	m := New()
	raw := withState(t, func(s *GameState) {
		s.DiscardPile = []string{"7S"}
		s.Hands["p1"] = []string{"7D", "KS"}
		s.PendingDraw = 2
	})

	// While a seven is unanswered, only another seven will do.
	if _, _, err := m.Apply(raw, "p1", module.Action{Verb: VerbPlay, Cards: []string{"KS"}}); module.CodeOf(err) != ErrMustAnswerDraw {
		t.Errorf("KS should be refused while a seven is pending, got %v", err)
	}

	// Answering with a seven passes the debt on, grown by two.
	next, _, err := m.Apply(raw, "p1", module.Action{Verb: VerbPlay, Cards: []string{"7D"}})
	if err != nil {
		t.Fatalf("answering with a seven: %v", err)
	}
	s := stateOf(t, next)
	if s.PendingDraw != 4 {
		t.Errorf("pendingDraw = %d, want 4 (two stacked sevens)", s.PendingDraw)
	}
	if s.Current != "p2" {
		t.Errorf("turn went to %q, want p2", s.Current)
	}
}

func TestSeven_TakingTheCardsClearsTheObligation(t *testing.T) {
	m := New()
	raw := withState(t, func(s *GameState) {
		s.DiscardPile = []string{"7S"}
		s.PendingDraw = 4
		s.DrawPile = []string{"8H", "9H", "TH", "JH", "QH"}
	})
	next, _, err := m.Apply(raw, "p1", module.Action{Verb: VerbDraw})
	if err != nil {
		t.Fatalf("taking the cards: %v", err)
	}
	s := stateOf(t, next)
	if got := len(s.Hands["p1"]); got != 2+4 {
		t.Errorf("p1 holds %d cards, want 6 (2 + the 4 owed)", got)
	}
	if s.PendingDraw != 0 {
		t.Errorf("pendingDraw = %d, want 0", s.PendingDraw)
	}
}

func TestAce_SkipsTheNextPlayerUnlessAnswered(t *testing.T) {
	m := New()
	raw := withState(t, func(s *GameState) {
		s.DiscardPile = []string{"AS"}
		s.SkipPending = true
		s.Hands["p1"] = []string{"AD", "KS"}
	})

	// Only another ace answers an ace.
	if _, _, err := m.Apply(raw, "p1", module.Action{Verb: VerbPlay, Cards: []string{"KS"}}); err == nil {
		t.Error("KS should not answer a pending skip")
	}

	// Or you take the skip.
	next, _, err := m.Apply(raw, "p1", module.Action{Verb: VerbPass})
	if err != nil {
		t.Fatalf("passing on a skip: %v", err)
	}
	s := stateOf(t, next)
	if s.SkipPending {
		t.Error("the skip should be spent")
	}
	if s.Current != "p2" {
		t.Errorf("turn went to %q, want p2", s.Current)
	}
}

func TestPass_OnlyLegalWhileASkipIsPending(t *testing.T) {
	// Otherwise passing would be a way to stall forever.
	m := New()
	raw := withState(t, func(s *GameState) {})
	if _, _, err := m.Apply(raw, "p1", module.Action{Verb: VerbPass}); err == nil {
		t.Error("passing with nothing pending should be refused")
	}
}

func TestWild_RequiresASuitAndSetsIt(t *testing.T) {
	m := New()
	raw := withState(t, func(s *GameState) {
		s.Hands["p1"] = []string{"QH", "KS"}
		s.DiscardPile = []string{"9S"}
	})

	// A wild without a named suit changes nothing.
	if _, _, err := m.Apply(raw, "p1", module.Action{Verb: VerbPlay, Cards: []string{"QH"}}); module.CodeOf(err) != ErrSuitRequired {
		t.Errorf("a wild needs a suit, got %v", err)
	}
	if _, _, err := m.Apply(raw, "p1", module.Action{
		Verb: VerbPlay, Cards: []string{"QH"}, Params: map[string]string{"suit": "Z"},
	}); module.CodeOf(err) != ErrUnknownSuit {
		t.Errorf("a nonsense suit should be refused, got %v", err)
	}

	next, _, err := m.Apply(raw, "p1", module.Action{
		Verb: VerbPlay, Cards: []string{"QH"}, Params: map[string]string{"suit": "D"},
	})
	if err != nil {
		t.Fatalf("playing a wild: %v", err)
	}
	s := stateOf(t, next)
	if s.DeclaredSuit != "D" {
		t.Errorf("declaredSuit = %q, want D", s.DeclaredSuit)
	}
	// The named suit is what the next card must match — and the Queen's own
	// rank must NOT be a way around it, or naming a suit would mean nothing.
	if !s.playable("8D") {
		t.Error("a diamond should answer a declared diamond")
	}
	if s.playable("8C") {
		t.Error("a club should not answer a declared diamond")
	}
}

func TestPlay_LastCardWinsBeforeAnyEffectApplies(t *testing.T) {
	// A seven that empties your hand wins; it does not first oblige an
	// opponent to draw and then win.
	m := New()
	raw := withState(t, func(s *GameState) {
		s.Hands["p1"] = []string{"7S"}
		s.DiscardPile = []string{"9S"}
	})
	next, _, err := m.Apply(raw, "p1", module.Action{Verb: VerbPlay, Cards: []string{"7S"}})
	if err != nil {
		t.Fatalf("playing the last card: %v", err)
	}
	s := stateOf(t, next)
	if s.Status != "completed" || s.WinnerID != "p1" {
		t.Fatalf("expected p1 to win, got status=%s winner=%s", s.Status, s.WinnerID)
	}
	if s.PendingDraw != 0 {
		t.Errorf("the winning seven should not leave an obligation, got %d", s.PendingDraw)
	}
}

func TestDraw_RecyclesTheDiscardPileKeepingItsTop(t *testing.T) {
	m := New()
	raw := withState(t, func(s *GameState) {
		s.DrawPile = nil
		s.DiscardPile = []string{"2H", "3H", "4H", "9S"}
	})
	next, _, err := m.Apply(raw, "p1", module.Action{Verb: VerbDraw})
	if err != nil {
		t.Fatalf("drawing from an empty pile: %v", err)
	}
	s := stateOf(t, next)
	if len(s.DiscardPile) != 1 || s.DiscardPile[0] != "9S" {
		t.Errorf("the face-up card should survive the recycle, got %v", s.DiscardPile)
	}
	if len(s.DrawPile) != 2 { // 3 recycled, 1 drawn
		t.Errorf("draw pile = %v, want 2 cards left", s.DrawPile)
	}
}

func TestApply_RefusesOutOfTurnAndUnknownVerbs(t *testing.T) {
	m := New()
	raw := withState(t, func(s *GameState) {})
	if _, _, err := m.Apply(raw, "p2", module.Action{Verb: VerbDraw}); module.CodeOf(err) != ErrNotYourTurn {
		t.Errorf("p2 is not on turn, got %v", err)
	}
	if _, _, err := m.Apply(raw, "p1", module.Action{Verb: "meld"}); module.CodeOf(err) != ErrUnknownAction {
		t.Errorf("prsi has no meld verb, got %v", err)
	}
}

func TestApply_DoesNotMutateTheCallersState(t *testing.T) {
	// State is a JSON blob decoded fresh on every call, so a refused or
	// speculative action physically cannot touch the caller's copy. This is
	// the aliasing hazard the rummy engine needed a regression test for,
	// absent by construction here — worth pinning so it stays that way.
	m := New()
	raw := withState(t, func(s *GameState) {})
	before := string(raw)
	for i := 0; i < 5; i++ {
		_, _, _ = m.Apply(raw, "p1", module.Action{Verb: VerbPlay, Cards: []string{"9D"}})
		_, _, _ = m.Apply(raw, "p1", module.Action{Verb: VerbDraw})
	}
	if string(raw) != before {
		t.Error("Apply mutated the state it was given")
	}
}

// TestNewMatch_OpeningPlayerVariesBySeed guards the fix for a lobby's host
// always leading: before it, Current was always TurnOrder[0]. Now it is
// picked from the match seed via module.StartingSeat.
func TestNewMatch_OpeningPlayerVariesBySeed(t *testing.T) {
	seen := map[string]bool{}
	for seed := int64(0); seed < 40; seed++ {
		s := stateOf(t, newGame(t, seed, "p1", "p2", "p3"))
		want := module.StartingSeat(seed, len(s.TurnOrder))
		if s.Current != s.TurnOrder[want] {
			t.Fatalf("seed %d: opened on %q, want seat %d (%q)", seed, s.Current, want, s.TurnOrder[want])
		}
		seen[s.Current] = true
	}
	if len(seen) < 2 {
		t.Fatalf("40 seeds only ever opened on %v — the opening seat is not varying", seen)
	}
}
