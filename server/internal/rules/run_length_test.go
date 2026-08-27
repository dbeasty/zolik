package rules

import "testing"

// A run reaches as far as the suit itself does: the low ace, the twelve ranks
// above it, and the high ace. Fourteen slots, and the fourteen-card run is a
// legal meld — not an edge the engine merely tolerates.
func TestValidateRun_LongestRunIsAceToAce(t *testing.T) {
	full := []string{"AS", "2S", "3S", "4S", "5S", "6S", "7S", "8S", "9S", "TS", "JS", "QS", "KS", "AS"}
	mv, err := validateRun(full, 3)
	if err != nil {
		t.Fatalf("a run from the low ace to the high ace should be valid: %v", err)
	}
	if len(mv.ResolvedRun) != MaxRunLength {
		t.Fatalf("expected %d slots, got %d", MaxRunLength, len(mv.ResolvedRun))
	}
	if mv.ResolvedRun[0] != 1 || mv.ResolvedRun[len(mv.ResolvedRun)-1] != 14 {
		t.Fatalf("expected the window to span rank 1 to rank 14, got %v", mv.ResolvedRun)
	}
}

func TestValidateRun_PastTheAceIsRefusedForItsLength(t *testing.T) {
	// Fifteen cards reaching for a run. There is no fifteenth slot to put one
	// in, and the player needs to be told that rather than what they used to
	// be told: the window search came back empty and the generic complaint
	// blamed wild cards, which here would be two jokers they are holding
	// legitimately and could not have placed any better.
	long := []string{"2S", "3S", "4S", "5S", "6S", "7S", "8S", "9S", "TS", "JS", "QS", "KS", "AS", "JOKER1", "JOKER2"}
	_, err := ValidateMeld(long, ProfileZolikClassic)
	if err == nil {
		t.Fatal("a fifteen-card run should be refused — the suit only has fourteen slots")
	}
	re, ok := err.(RulesError)
	if !ok || re.Code != ErrRunTooLong {
		t.Fatalf("expected RUN_TOO_LONG, got %v", err)
	}
}

// The ceiling is a rule about runs, not a cap on lay-offs: a run that has not
// reached fourteen keeps growing through the lay-off path.
func TestValidateLayOff_RunStillGrowsBelowTheCeiling(t *testing.T) {
	p := "p1"
	st := classicState(p)
	st.Hands[p] = []string{"6S", "2H"}
	st.Melds[p] = [][]string{{"2S", "3S", "4S", "5S"}}
	st.MeldMeta[p] = []MeldInfo{{MeldID: "meld_1", Type: MeldRun, OwnerID: p}}
	st.NextMeldSeq = 1
	st.RoundReqMet[p] = true

	next, err := ValidateLayOff(st, p, "meld_1", []string{"6S"}, "end")
	if err != nil {
		t.Fatalf("a four-card run is nowhere near the ceiling and should still grow: %v", err)
	}
	if got := len(next.Melds[p][0]); got != 5 {
		t.Fatalf("expected the run to reach 5 cards, got %d", got)
	}
}
