package zolikmod

import (
	"slices"
	"testing"

	"zolik/server/internal/module"
	"zolik/server/internal/rules"
)

// hintView renders gs for viewerID and returns the prompts marked Hint, the
// ones the client lists in its hints sheet.
func hintView(t *testing.T, gs rules.GameState, viewerID string) []module.Fact {
	t.Helper()
	players := []module.PlayerRef{{ID: "p1", Name: "A"}, {ID: "p2", Name: "B"}}
	st, err := encode(&matchState{Rules: gs, Players: players})
	if err != nil {
		t.Fatal(err)
	}
	vm, err := (&Module{}).View(st, viewerID)
	if err != nil {
		t.Fatal(err)
	}
	var out []module.Fact
	for _, p := range vm.Prompts {
		if p.Hint {
			out = append(out, p)
		}
	}
	return out
}

func hintKeys(fs []module.Fact) []string {
	var out []string
	for _, f := range fs {
		out = append(out, f.LabelKey)
	}
	return out
}

func hintState(cfg rules.RulesConfig) rules.GameState {
	return rules.GameState{
		Status:      rules.StatusActive,
		Rules:       cfg,
		GameNumber:  1,
		Round:       1,
		Phase:       rules.PhaseDraw,
		CurrentTurn: "p1",
		TurnOrder:   []string{"p1", "p2"},
		Hands:       map[string][]string{"p1": {"7C", "KS"}, "p2": {"2D", "2H"}},
		Melds:       map[string][][]string{},
		MeldMeta:    map[string][]rules.MeldInfo{},
		RoundReqMet: map[string]bool{},
		DrawPile:    []string{"3S", "4D"},
		DiscardPile: []string{"9C"},
		GameScores:  map[string][]int{},
		TotalScores: map[string]int{},
	}
}

// A Continental player who is not down is told what the deal still asks of
// them, the way Canasta tells a side its first-meld floor.
func TestHintsListWhatAContinentalPlayerStillOwes(t *testing.T) {
	cfg := resolveConfig(module.MatchConfig{Variation: "continental"})
	gs := hintState(cfg)
	contract := cfg.ContractFor(gs.GameNumber)

	got := hintView(t, gs, "p1")
	var want []string
	if contract.Sets > 0 {
		want = append(want, "zolik.hint.setsNeeded")
	}
	if contract.Runs > 0 {
		want = append(want, "zolik.hint.runsNeeded")
	}
	if contract.RequireCleanRun {
		want = append(want, "zolik.hint.cleanRunNeeded")
	}
	want = append(want, "zolik.hint.meldPoints", "zolik.hint.downInOneTurn",
		"zolik.hint.layOffWhenDown", "zolik.hint.discardLocked")
	if !slices.Equal(hintKeys(got), want) {
		t.Fatalf("hints = %v, want %v", hintKeys(got), want)
	}
	for _, f := range got {
		switch f.LabelKey {
		case "zolik.hint.meldPoints":
			if f.Params["n"] != 35 || f.Params["have"] != 0 {
				t.Errorf("meldPoints params = %v", f.Params)
			}
		case "zolik.hint.discardLocked":
			if f.Params["n"] != 3 {
				t.Errorf("discardLocked params = %v", f.Params)
			}
		}
	}

	// Once down, and past the lock, nothing is owed and nothing is sent.
	gs.RoundReqMet["p1"] = true
	gs.Round = 3
	if got := hintView(t, gs, "p1"); len(got) != 0 {
		t.Errorf("a down player past the discard lock still gets hints: %v", hintKeys(got))
	}
}

// What is owed is counted from the melds already laid: a player with one of
// two sets down is told one more, not the whole contract again.
func TestHintsCountWhatIsLeftNotTheWholeContract(t *testing.T) {
	cfg := resolveConfig(module.MatchConfig{Variation: "continental"})
	gs := hintState(cfg)
	if c := cfg.ContractFor(gs.GameNumber); c.Sets < 2 {
		t.Skipf("deal 1 asks for %d sets; this test needs two", c.Sets)
	}
	gs.Melds["p1"] = [][]string{{"9D", "9H", "9S"}}
	gs.MeldMeta["p1"] = []rules.MeldInfo{{MeldID: "m1", Type: rules.MeldSet, OwnerID: "p1"}}

	got := hintView(t, gs, "p1")
	for _, f := range got {
		if f.LabelKey == "zolik.hint.setsNeeded" {
			if want := cfg.ContractFor(1).Sets - 1; f.Params["n"] != want {
				t.Errorf("setsNeeded n = %v, want %d", f.Params["n"], want)
			}
		}
		if f.LabelKey == "zolik.hint.meldPoints" && f.Params["have"] != 27 {
			t.Errorf("meldPoints have = %v, want 27", f.Params["have"])
		}
	}
}

// Žolík Classic asks for one thing, a joker-free run: no counts, no floor, no
// lock.
func TestHintsOnAClassicTableNameTheCleanRun(t *testing.T) {
	gs := hintState(resolveConfig(module.MatchConfig{}))
	want := []string{"zolik.hint.cleanRunNeeded", "zolik.hint.downInOneTurn", "zolik.hint.layOffWhenDown"}
	if got := hintKeys(hintView(t, gs, "p1")); !slices.Equal(got, want) {
		t.Fatalf("hints = %v, want %v", got, want)
	}
}

// Nobody sits at an open view, so nothing is owed by it.
func TestHintsAreNotSentToASpectator(t *testing.T) {
	gs := hintState(resolveConfig(module.MatchConfig{Variation: "continental"}))
	if got := hintView(t, gs, ""); len(got) != 0 {
		t.Errorf("spectator hints = %v", hintKeys(got))
	}
}
