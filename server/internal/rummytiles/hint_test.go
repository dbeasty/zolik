package rummytiles

import "testing"

// The first-lay requirement is a hint — listed in its own sheet, sent to both
// seats while unmet — and the turn prompt is separate, sent to the mover only.
func TestView_FirstLayIsAHintAndTheTurnIsNot(t *testing.T) {
	raw := newMatch(t, 21, "p1", "p2")
	s := stateOf(t, raw)
	m := New()

	for _, viewer := range s.Players {
		vm, err := m.View(raw, viewer)
		if err != nil {
			t.Fatalf("View: %v", err)
		}
		var hints, turns int
		for _, p := range vm.Prompts {
			switch p.LabelKey {
			case "rummytiles.prompt.initialMeld":
				if !p.Hint || p.Params["n"] != 30 {
					t.Errorf("%s: initial meld prompt = %+v, want a hint worth 30", viewer, p)
				}
				hints++
			case "rummytiles.prompt.yourTurn":
				if p.Hint {
					t.Errorf("%s: the turn prompt must not be a hint", viewer)
				}
				turns++
			}
		}
		if hints != 1 {
			t.Errorf("%s: %d first-lay hints, want 1", viewer, hints)
		}
		want := 0
		if viewer == s.Current {
			want = 1
		}
		if turns != want {
			t.Errorf("%s: %d turn prompts, want %d", viewer, turns, want)
		}
	}

	// Once the first lay is made the requirement is no longer sent.
	laid := withState(t, func(g *GameState) { g.InitialMeld["p1"] = true })
	vm, err := m.View(laid, "p1")
	if err != nil {
		t.Fatalf("View: %v", err)
	}
	for _, p := range vm.Prompts {
		if p.Hint {
			t.Errorf("a met requirement was still sent: %+v", p)
		}
	}
}
