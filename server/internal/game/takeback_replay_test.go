package game

import (
	"testing"

	"zolik/server/internal/models"
	"zolik/server/internal/rules"
)

// TestDealSnapshotToRulesState_ReplaysUnderTheGamesOwnRuleset closes the last
// hole in the "config split brain" that TestGameRules_UsesPersistedConfigNot-
// ProfileName guards elsewhere: a game's ruleset is frozen onto the document
// precisely so it can never be re-derived from a profile name, but the
// takeback replay path was doing exactly that.
//
// Rebuilding from the name plus the snapshot's two legacy scalar columns
// happens to agree today, because those two are the only knobs a lobby can
// currently set. Any other tunable — MinRunSize here, which has no legacy
// column at all — silently reverted to the profile default, so a takeback
// would replay the deal under different rules than the ones the deal was
// actually played under, and hand back a state the players never reached.
func TestDealSnapshotToRulesState_ReplaysUnderTheGamesOwnRuleset(t *testing.T) {
	cfg := rules.ProfileZolikClassic
	cfg.MinRunSize = 5          // house rule; zolik_classic ships 3
	cfg.InitialMeldMinimum = 50 // has a legacy column, so both paths agree
	cfg.DiscardDrawMinRound = 4 // ditto

	g := models.Game{RulesProfile: "zolik_classic", GameNumber: 2, Round: 1}
	setGameRules(&g, cfg)
	g.Hands = map[string][]string{"p1": {"5H"}}
	g.Melds = map[string][][]string{}
	g.RoundReqMet = map[string]bool{"p1": false}
	g.TurnOrder = []string{"p1"}
	g.CurrentTurn = "p1"
	g.Phase = string(rules.PhaseDraw)

	snap := captureDealSnapshot(g, 0)
	got := dealSnapshotToRulesState(*snap, GameRules(g))

	if got.Rules.MinRunSize != 5 {
		t.Fatalf("MinRunSize: want the game's own 5, got %d — replay fell back to the profile default", got.Rules.MinRunSize)
	}
	if got.Rules.InitialMeldMinimum != 50 {
		t.Fatalf("InitialMeldMinimum: want 50, got %d", got.Rules.InitialMeldMinimum)
	}
	if got.Rules.DiscardDrawMinRound != 4 {
		t.Fatalf("DiscardDrawMinRound: want 4, got %d", got.Rules.DiscardDrawMinRound)
	}
	if got.GameNumber != g.GameNumber {
		t.Fatalf("GameNumber: want %d, got %d", g.GameNumber, got.GameNumber)
	}
}

// A zero-value config must not reach the engine as MinRunSize 0 — every other
// entry point resolves it, and so must this one.
func TestDealSnapshotToRulesState_ResolvesAZeroConfig(t *testing.T) {
	snap := models.DealSnapshot{GameNumber: 1, Phase: string(rules.PhaseDraw)}

	got := dealSnapshotToRulesState(snap, rules.RulesConfig{})

	if got.Rules.MinRunSize == 0 {
		t.Fatal("a zero-value ruleset reached the engine unresolved")
	}
}
