package prsi

import (
	"testing"

	"zolik/server/internal/module"
)

// Each special-card sentence the rules screen shows is pinned to what the
// engine does. The ace rule used to say the next turn "is skipped", while the
// engine (and the move log) let it be answered with an ace; stacking sevens,
// a svršek on anything and the turned-up card's effect were not written at all.
func TestSpecialCardRulesAreWhatTheEnginePlays(t *testing.T) {
	sections, err := New().Rules(module.MatchConfig{})
	if err != nil {
		t.Fatal(err)
	}
	stated := map[string]bool{}
	for _, sec := range sections {
		for _, it := range sec.Items {
			stated[it.LabelKey] = true
		}
	}
	for _, key := range []string{"prsi.rules.firstCard", "prsi.rules.sevens.stack", "prsi.rules.aces", "prsi.rules.queens.anything"} {
		if !stated[key] {
			t.Errorf("rules do not state %s", key)
		}
	}

	m := New()
	play := func(raw module.State, who, card string, params map[string]string) module.State {
		t.Helper()
		next, _, err := m.Apply(raw, who, module.Action{Verb: VerbPlay, Cards: []string{card}, Params: params})
		if err != nil {
			t.Fatalf("%s plays %s: %v", who, card, err)
		}
		return next
	}

	// prsi.rules.aces: an ace is answered with an ace, which passes it on.
	raw := withState(t, func(s *GameState) {
		s.DiscardPile = []string{"9S"}
		s.Hands = map[string][]string{"p1": {"AS", "KS"}, "p2": {"AD", "7C"}}
	})
	raw = play(raw, "p1", "AS", nil)
	raw = play(raw, "p2", "AD", nil)
	if s := stateOf(t, raw); s.Current != "p1" || !s.SkipPending {
		t.Fatalf("an ace answering an ace passes the missed turn back: %s on, skip %v", s.Current, s.SkipPending)
	}

	// prsi.rules.sevens.stack: a seven on a seven owes four.
	raw = withState(t, func(s *GameState) {
		s.DiscardPile = []string{"9C"}
		s.Hands = map[string][]string{"p1": {"7C", "KS"}, "p2": {"7D", "8H"}}
		s.DrawPile = []string{"8S", "9H", "TH", "JH", "KH"}
	})
	raw = play(raw, "p1", "7C", nil)
	raw = play(raw, "p2", "7D", nil)
	if s := stateOf(t, raw); s.PendingDraw != 4 {
		t.Fatalf("two sevens owe 4, got %d", s.PendingDraw)
	}

	// prsi.rules.queens.anything: a svršek goes on a card of another suit and
	// rank — but not on a seven waiting to be answered.
	raw = withState(t, func(s *GameState) {
		s.DiscardPile = []string{"9S"}
		s.Hands = map[string][]string{"p1": {"QH", "KD"}, "p2": {"8C", "8D"}}
	})
	play(raw, "p1", "QH", map[string]string{"suit": "C"})
	raw = withState(t, func(s *GameState) {
		s.DiscardPile = []string{"7S"}
		s.PendingDraw = 2
		s.Hands = map[string][]string{"p1": {"QH", "KD"}, "p2": {"8C", "8D"}}
	})
	if _, _, err := m.Apply(raw, "p1", module.Action{Verb: VerbPlay, Cards: []string{"QH"}, Params: map[string]string{"suit": "C"}}); module.CodeOf(err) != ErrMustAnswerDraw {
		t.Fatalf("a svršek on an unanswered seven: %v, want %s", err, ErrMustAnswerDraw)
	}

	// prsi.rules.firstCard: a turned-up seven or ace acts on the first
	// player, and a svršek is never turned up while another card can be.
	for seed := int64(1); seed <= 400; seed++ {
		s := stateOf(t, newGame(t, seed, "p1", "p2"))
		switch rankOf(s.top()) {
		case rankDrawTwo:
			if s.PendingDraw != 2 {
				t.Fatalf("seed %d: a turned-up seven owes the first player 2, got %d", seed, s.PendingDraw)
			}
		case rankSkip:
			if !s.SkipPending {
				t.Fatalf("seed %d: a turned-up ace does not act on the first player", seed)
			}
		case rankWild:
			t.Fatalf("seed %d: a svršek was turned up", seed)
		}
	}
}
