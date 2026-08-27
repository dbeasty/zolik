package ai

import (
	"testing"

	"zolik/server/internal/rules"
)

// manyMeldsTable builds a table with melds owned by several players, all of
// which the given card could legally extend. Which one the agent picks used to
// depend on Go's randomised map iteration order.
func manyMeldsTable() (map[string][][]string, map[string][]rules.MeldInfo) {
	melds := map[string][][]string{
		"pAlice":   {{"5C", "6C", "7C"}},
		"pBob":     {{"5D", "6D", "7D"}},
		"pCarol":   {{"5H", "6H", "7H"}},
		"pDan":     {{"5S", "6S", "7S"}},
		"pErin":    {{"9C", "TC", "JC"}},
		"pFrances": {{"9D", "TD", "JD"}},
	}
	meta := map[string][]rules.MeldInfo{
		"pAlice":   {{MeldID: "m-alice", Type: rules.MeldRun, OwnerID: "pAlice"}},
		"pBob":     {{MeldID: "m-bob", Type: rules.MeldRun, OwnerID: "pBob"}},
		"pCarol":   {{MeldID: "m-carol", Type: rules.MeldRun, OwnerID: "pCarol"}},
		"pDan":     {{MeldID: "m-dan", Type: rules.MeldRun, OwnerID: "pDan"}},
		"pErin":    {{MeldID: "m-erin", Type: rules.MeldRun, OwnerID: "pErin"}},
		"pFrances": {{MeldID: "m-frances", Type: rules.MeldRun, OwnerID: "pFrances"}},
	}
	return melds, meta
}

// TestFindLayOff_IsDeterministicAcrossRuns is the regression guard for
// map-order-dependent play: with several equally-valid targets on the table,
// the agent must pick the same one every time, or the same position produces
// different moves on different runs and the behaviour can't be reasoned about
// or reproduced from a bug report.
func TestFindLayOff_IsDeterministicAcrossRuns(t *testing.T) {
	melds, meta := manyMeldsTable()
	cfg := rules.ProfileZolikClassic
	// 8C extends pAlice's run, 8D extends pBob's, 8H pCarol's, 8S pDan's —
	// four owners, four legal lay-offs, one of which has to be chosen.
	hand := []string{"8C", "8D", "8H", "8S", "2C"}

	wantMeld, wantCard, ok := findLayOff(meta, melds, hand, cfg, 1)
	if !ok {
		t.Fatalf("expected a lay-off to be available")
	}
	for i := 0; i < 200; i++ {
		gotMeld, gotCard, ok := findLayOff(meta, melds, hand, cfg, 1)
		if !ok {
			t.Fatalf("iteration %d: expected a lay-off to be available", i)
		}
		if gotMeld != wantMeld || gotCard != wantCard {
			t.Fatalf("iteration %d: lay-off choice is not deterministic — first run picked %s onto %s, this run picked %s onto %s",
				i, wantCard, wantMeld, gotCard, gotMeld)
		}
	}
}

// TestFindLayOff_PrefersTheLowestOwnerIDPinsTheOrdering documents which
// deterministic order was chosen, so a future refactor that quietly changes it
// shows up as a failing test rather than as a silent behaviour change.
func TestFindLayOff_PrefersTheLowestOwnerIDPinsTheOrdering(t *testing.T) {
	melds, meta := manyMeldsTable()
	meldID, card, ok := findLayOff(meta, melds, []string{"8S", "8C", "2C"}, rules.ProfileZolikClassic, 1)
	if !ok {
		t.Fatalf("expected a lay-off to be available")
	}
	// Owners are visited in sorted order, so pAlice's run is reached before
	// pDan's even though 8S comes first in hand.
	if meldID != "m-alice" || card != "8C" {
		t.Fatalf("expected the first owner in sorted order (pAlice) to be extended with 8C, got %s onto %s", card, meldID)
	}
}

// TestChooseAction_IsDeterministic covers the whole decision function rather
// than one helper: the same visible state and hand must always yield the same
// action. HeuristicAgent deliberately carries no randomness (it previously
// held a wall-clock-seeded RNG that nothing read).
func TestChooseAction_IsDeterministic(t *testing.T) {
	melds, meta := manyMeldsTable()
	visible := VisibleState{
		GameNumber:  1,
		Round:       1,
		Phase:       string(rules.PhaseMeld),
		CurrentTurn: "ai1",
		Melds:       melds,
		MeldMeta:    meta,
		RoundReqMet: map[string]bool{"ai1": true},
		Rules:       rules.ProfileZolikClassic,
	}
	hand := []string{"8C", "8D", "KH", "QS", "4D"}

	for _, difficulty := range []string{"easy", "medium", "hard"} {
		first := NewHeuristicAgent(difficulty).ChooseAction(visible, hand)
		for i := 0; i < 100; i++ {
			got := NewHeuristicAgent(difficulty).ChooseAction(visible, hand)
			if got.Type != first.Type || got.Card != first.Card || got.MeldID != first.MeldID || got.DrawFrom != first.DrawFrom {
				t.Fatalf("%s: ChooseAction is not deterministic — first %+v, iteration %d %+v", difficulty, first, i, got)
			}
		}
	}
}

// TestExtendsAnyLiveMeld_IsDeterministic pins the discard-safety check, which
// also walks the melds map. Its answer is order-independent in principle, but
// the fixed order keeps it that way if the loop ever gains an early return.
func TestExtendsAnyLiveMeld_IsDeterministic(t *testing.T) {
	melds, _ := manyMeldsTable()
	cfg := rules.ProfileZolikClassic
	for i := 0; i < 200; i++ {
		if !extendsAnyLiveMeld("8C", melds, cfg) {
			t.Fatalf("iteration %d: 8C extends pAlice's 5C-6C-7C and must be reported as dangerous", i)
		}
		if extendsAnyLiveMeld("2S", melds, cfg) {
			t.Fatalf("iteration %d: 2S fits nothing on the table and must be reported as safe", i)
		}
	}
}
