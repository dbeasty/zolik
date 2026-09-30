package zolikmod

import (
	"testing"

	"zolik/server/internal/module"
	"zolik/server/internal/rules"
)

// The open-pile option, where Žolíky is not like the other three games: its
// pile has always been published whole, so the option's default is on and its
// persisted form is the restriction (RulesConfig.DiscardPileTopOnly) rather
// than the openness — a match dealt before the option existed comes back with
// the pile it has always had.
//
// The second half is the one that matters at the table: under free pickup the
// draw offers every card in the pile by name, so folding it away would hide
// cards the same turn is inviting the player to take. The fold is refused
// there, and this says so.

func pileState(cfg rules.RulesConfig) module.State {
	gs := rules.GameState{
		Status:      rules.StatusActive,
		Rules:       cfg,
		GameNumber:  1,
		TurnOrder:   []string{"p1", "p2"},
		CurrentTurn: "p1",
		Hands:       map[string][]string{"p1": {"4C"}, "p2": {"5C"}},
		DiscardPile: []string{"2D", "7H", "9S"},
	}
	raw, err := encode(&matchState{Rules: gs, Players: refs("p1", "p2")})
	if err != nil {
		panic(err)
	}
	return raw
}

func pileCards(t *testing.T, cfg rules.RulesConfig) []string {
	t.Helper()
	vm, err := New().View(pileState(cfg), "p2")
	if err != nil {
		t.Fatalf("View: %v", err)
	}
	var out []string
	for _, c := range discardZoneOf(t, vm).Cards {
		out = append(out, c.Card)
	}
	return out
}

func TestView_PileIsOpenUnlessTheTableFoldedIt(t *testing.T) {
	// Continental draws the top card only, so the fold is honoured there.
	open := rules.ProfileContinental
	if got := pileCards(t, open); len(got) != 3 {
		t.Fatalf("the default is the open pile; got %v", got)
	}

	folded := rules.ProfileContinental
	folded.DiscardPileTopOnly = true
	if got := pileCards(t, folded); len(got) != 1 || got[0] != "9S" {
		t.Fatalf("a folded pile shows its top card alone; got %v", got)
	}
}

func TestView_FreePickupKeepsThePileOpen(t *testing.T) {
	cfg := rules.ProfileZolikClassic
	cfg.DiscardPileTopOnly = true
	if cfg.DiscardPickupMode != rules.DiscardPickupAnyFromPile {
		t.Fatalf("this test is about free pickup; the profile draws %q", cfg.DiscardPickupMode)
	}
	if got := pileCards(t, cfg); len(got) != 3 {
		t.Fatalf("free pickup cannot hide what it offers; got %v", got)
	}
}

// The option is resolved into the persisted ruleset, and its default is the
// behaviour the game already had — so a table that says nothing keeps the open
// pile, and a stored config from before the option is read the same way.
func TestResolveConfig_OpenPileIsTheDefault(t *testing.T) {
	if resolveConfig(module.MatchConfig{Variation: "continental"}).DiscardPileTopOnly {
		t.Error("a table that chose nothing should keep the open pile")
	}
	off := module.MatchConfig{
		Variation: "continental",
		Options:   module.Options{module.OptOpenDiscardPile: module.OptOff},
	}
	if !resolveConfig(off).DiscardPileTopOnly {
		t.Error("a table that folded the pile should have it folded")
	}
	on := module.MatchConfig{
		Variation: "continental",
		Options:   module.Options{module.OptOpenDiscardPile: module.OptOn},
	}
	if resolveConfig(on).DiscardPileTopOnly {
		t.Error("a table that opened the pile should have it open")
	}
}
