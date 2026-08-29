package zolikmod

import (
	"testing"

	"zolik/server/internal/module"
	"zolik/server/internal/rules"
)

// The closing gesture's presentational half: when the deal ended on a discard
// and the profile plays the gesture, the pile's top card — and only the top
// card — is dressed face down. Its value still travels (the deal is already
// scored; see module.CardView.FaceDown), so the assertions below check the
// flag, not an absence.

func discardZoneOf(t *testing.T, vm module.ViewModel) module.Zone {
	t.Helper()
	for _, z := range vm.Zones {
		if z.ID == discardZoneID {
			return z
		}
	}
	t.Fatalf("no discard zone in view: %#v", vm.Zones)
	return module.Zone{}
}

func closedState(cfg rules.RulesConfig, wentOutByDiscard bool) module.State {
	gs := rules.GameState{
		Status:           rules.StatusCompleted,
		Rules:            cfg,
		GameNumber:       1,
		TurnOrder:        []string{"p1", "p2"},
		Hands:            map[string][]string{"p1": {}, "p2": {"4C"}},
		DiscardPile:      []string{"2D", "9S"},
		WentOutByDiscard: wentOutByDiscard,
	}
	raw, err := encode(&matchState{Rules: gs, Players: refs("p1", "p2")})
	if err != nil {
		panic(err)
	}
	return raw
}

func TestView_ClosingDiscardLandsFaceDown(t *testing.T) {
	vm, err := New().View(closedState(rules.ProfileZolikClassic, true), "p2")
	if err != nil {
		t.Fatalf("View: %v", err)
	}
	pile := discardZoneOf(t, vm)
	top := pile.Cards[len(pile.Cards)-1]
	if !top.FaceDown {
		t.Fatalf("expected the closing card face down, got %#v", top)
	}
	if top.Card != "9S" {
		t.Fatalf("the value still travels — the deal is scored; got %#v", top)
	}
	for _, c := range pile.Cards[:len(pile.Cards)-1] {
		if c.FaceDown {
			t.Fatalf("only the closing card lies face down, got %#v", pile.Cards)
		}
	}
}

func TestView_ContinentalShowsTheClosingDiscardFaceUp(t *testing.T) {
	vm, err := New().View(closedState(rules.ProfileContinental, true), "p2")
	if err != nil {
		t.Fatalf("View: %v", err)
	}
	pile := discardZoneOf(t, vm)
	if top := pile.Cards[len(pile.Cards)-1]; top.FaceDown {
		t.Fatalf("Continental plays every discard face up, got %#v", top)
	}
}

func TestView_DealEndedWithoutADiscardStaysFaceUp(t *testing.T) {
	vm, err := New().View(closedState(rules.ProfileZolikClassic, false), "p2")
	if err != nil {
		t.Fatalf("View: %v", err)
	}
	pile := discardZoneOf(t, vm)
	if top := pile.Cards[len(pile.Cards)-1]; top.FaceDown {
		t.Fatalf("a deal closed by a lay-off has no closing discard, got %#v", top)
	}
}
