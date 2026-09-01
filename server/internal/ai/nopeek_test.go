package ai

import (
	"reflect"
	"testing"

	"zolik/server/internal/module"
	"zolik/server/internal/rules"
)

// TestVisibleForCarriesNoHiddenHand is the guarantee that makes a strong bot
// fair rather than cheating.
//
// The module hands the agent a snapshot built from the engine's whole state —
// every hand on the table included — and the agent is trusted to look only at
// its own. "Trusted" is not a guarantee, so this is one: permute every hidden
// hand and assert the snapshot comes out identical. The agent cannot read what
// the snapshot does not carry.
//
// It is the counterpart to holdem's TestBotDoesNotPeek, and it matters more
// here, because this agent counts cards. A counter that could see a hand would
// not be a strong opponent, it would be a cheat with a good excuse.
func TestVisibleForCarriesNoHiddenHand(t *testing.T) {
	base := rules.GameState{
		GameNumber:  1,
		Phase:       rules.PhaseMeld,
		CurrentTurn: "me",
		TurnOrder:   []string{"me", "them", "other"},
		Hands: map[string][]string{
			"me":    {"KH", "KS", "3C"},
			"them":  {"AH", "AS", "AD", "2C"},
			"other": {"7H", "8H", "9H"},
		},
		DrawPile:    []string{"4C", "5C"},
		DiscardPile: []string{"QD"},
		Melds:       map[string][][]string{},
		MeldMeta:    map[string][]rules.MeldInfo{},
		RoundReqMet: map[string]bool{"me": true, "them": false, "other": false},
		TotalScores: map[string]int{},
		Rules:       rules.ProfileZolikClassic,
	}
	ledger := Ledger{Deal: 1, Discards: []SeenDiscard{{Player: "them", Card: "QD"}}}

	want := VisibleFor(base, ledger, "me")

	// Same hand sizes, entirely different cards. Anything that leaked would
	// have to change.
	swapped := base
	swapped.Hands = map[string][]string{
		"me":    {"KH", "KS", "3C"},
		"them":  {"JC", "JD", "JS", "TH"},
		"other": {"2S", "2D", "2H"},
	}
	got := VisibleFor(swapped, ledger, "me")

	if !reflect.DeepEqual(want, got) {
		t.Fatalf("the snapshot changed when hidden hands changed — something leaks\n want %+v\n got  %+v", want, got)
	}
	// And it must still carry the counts, or the agent is blind to the
	// endgame rather than merely honest about it.
	if got.HandCounts["them"] != 4 || got.HandCounts["other"] != 3 {
		t.Errorf("hand counts missing or wrong: %v", got.HandCounts)
	}
}

// TestAgentChoosesTheSameMoveWhateverOpponentsHold is the same guarantee one
// level up: not that the snapshot is clean, but that the decision is.
func TestAgentChoosesTheSameMoveWhateverOpponentsHold(t *testing.T) {
	for _, skill := range []string{"easy", "medium", "hard"} {
		hand := []string{"KH", "KS", "3C", "9D"}
		mk := func(theirs []string) rules.Action {
			st := rules.GameState{
				GameNumber:  1,
				Phase:       rules.PhaseMeld,
				CurrentTurn: "me",
				TurnOrder:   []string{"me", "them"},
				Hands:       map[string][]string{"me": hand, "them": theirs},
				DiscardPile: []string{"QD"},
				Melds:       map[string][][]string{},
				MeldMeta:    map[string][]rules.MeldInfo{},
				RoundReqMet: map[string]bool{"me": true, "them": true},
				TotalScores: map[string]int{},
				Rules:       rules.ProfileZolikClassic,
			}
			v := VisibleFor(st, Ledger{Deal: 1}, "me")
			return NewAgent(module.Skill(skill), 7).ChooseAction(v, append([]string(nil), hand...))
		}
		a := mk([]string{"AH", "AS", "AD"})
		b := mk([]string{"2C", "3D", "4S"})
		if !reflect.DeepEqual(a, b) {
			t.Errorf("%s changed its move when the opponent's hand changed: %+v vs %+v", skill, a, b)
		}
	}
}
