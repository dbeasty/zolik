package rules

import "testing"

// A run that has grown past ten cards holds a low natural (2, 3) and a high
// one (J) at the same time, which is exactly what the ace-bridge guard used
// to look for. From game 6a8ac136b1f69ebbeb326e37: a player laid a joker onto
// 2D-3D-*-5D-6D-7D-*-9D as the ten of diamonds, then could not lay the jack
// that continues it — the run was refused as an ace bridge and, worse, the
// message shown complained that "all cards in a set must share the same rank".
func TestValidateMeld_LongRunSpanningLowAndHighNaturals(t *testing.T) {
	cfg := ProfileZolikClassic
	base := []string{"2D", "3D", "JOKER2", "5D", "6D", "7D", "JOKER2", "9D", "JOKER1"}

	for _, next := range []string{"JD", "QD", "KD"} {
		base = append(base, next)
		mv, err := ValidateMeld(base, cfg)
		if err != nil {
			t.Fatalf("laying %s onto %v: %v", next, base[:len(base)-1], err)
		}
		if mv.Type != MeldRun {
			t.Fatalf("laying %s: got meld type %q, want run", next, mv.Type)
		}
		if got, want := len(mv.ResolvedRun), len(base); got != want {
			t.Fatalf("laying %s: resolved %d slots for %d cards", next, got, want)
		}
	}
}

// The same growth through the real lay-off path, so the fix holds where the
// player actually hits it: dropping the jack on the end of the run.
func TestValidateLayOff_ContinuesRunPastTheJack(t *testing.T) {
	const player, ai = "P", "A"
	state := GameState{
		Status:      StatusActive,
		Rules:       ProfileZolikClassic,
		GameNumber:  1,
		Round:       8,
		Phase:       PhaseMeld,
		CurrentTurn: player,
		TurnOrder:   []string{player, ai},
		Hands: map[string][]string{
			player: {"QD", "JD"},
			ai:     {"3C"},
		},
		Melds: map[string][][]string{
			ai:     {{"2D", "3D", "JOKER2", "5D", "6D", "7D", "JOKER2", "9D", "JOKER1"}},
			player: {{"QC", "KC", "AC"}, {"2D", "2H", "2S"}},
		},
		MeldMeta: map[string][]MeldInfo{
			ai:     {{MeldID: "meld_1", Type: MeldRun, OwnerID: ai, WildCount: 3}},
			player: {{MeldID: "meld_6", Type: MeldRun, OwnerID: player}, {MeldID: "meld_7", Type: MeldSet, OwnerID: player}},
		},
		RoundReqMet: map[string]bool{player: true, ai: true},
		DrawPile:    []string{"5C", "6C", "7C"},
		DiscardPile: []string{"5H"},
	}

	// Snapshot the run before the lay-off: ValidateLayOff writes the grown
	// meld back into the same slice, so reading it afterwards would compare
	// the run against itself.
	before := append([]string(nil), state.Melds[ai][0]...)

	next, err := ValidateLayOff(state, player, "meld_1", []string{"JD"}, "end")
	if err != nil {
		t.Fatalf("laying the jack on the end of the run: %v", err)
	}
	got := next.Melds[ai][0]
	want := []string{"2D", "3D", "JOKER2", "5D", "6D", "7D", "JOKER2", "9D", "JOKER1", "JD"}
	if len(got) != len(want) {
		t.Fatalf("run is %v, want %d cards", got, len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("run is %v, want %v", got, want)
		}
	}

	// And the client's drop hints have to agree, or the jack never becomes
	// draggable onto that end in the first place.
	sides := droppableEnds(before, want, ProfileZolikClassic)
	if !containsString(sides, "end") {
		t.Fatalf("droppable ends for the jack = %v, want it to include \"end\"", sides)
	}
}

// The guard still has to catch what it was written for: a genuine wrap from
// the king around through the ace to the low cards.
func TestValidateRun_StillRejectsTheRealAceBridge(t *testing.T) {
	if _, err := validateRun([]string{"KH", "AH", "2H", "3H"}, 4); err == nil {
		t.Fatal("expected K-A-2-3 to be rejected as an ace bridge")
	}
	if _, err := validateRun([]string{"2S", "3S", "QS", "KS"}, 4); err == nil {
		t.Fatal("expected 2-3 plus Q-K to be rejected: they cannot share one four-card window")
	}
}

// A failed run should be explained as a run, not as a set the player never
// tried to build.
func TestValidateMeld_ExplainsRunFailuresAsRuns(t *testing.T) {
	_, err := ValidateMeld([]string{"KH", "AH", "2H", "3H"}, ProfileZolikClassic)
	re, ok := err.(RulesError)
	if !ok {
		t.Fatalf("got %v, want a RulesError", err)
	}
	if re.Code != ErrAceBridge {
		t.Fatalf("got code %q (%s), want %q", re.Code, re.Message, ErrAceBridge)
	}

	// A real set attempt still gets the set explanation.
	_, err = ValidateMeld([]string{"5C", "5D", "6H"}, ProfileZolikClassic)
	re, ok = err.(RulesError)
	if !ok {
		t.Fatalf("got %v, want a RulesError", err)
	}
	if re.Message != "all cards in a set must share the same rank" {
		t.Fatalf("got %q, want the set-rank explanation", re.Message)
	}
}
