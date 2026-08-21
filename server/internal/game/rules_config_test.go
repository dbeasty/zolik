package game

import (
	"testing"

	"zolik/server/internal/models"
	"zolik/server/internal/rules"
)

// TestGameRules_UsesPersistedConfigNotProfileName is the regression guard for
// the config split brain: the ruleset a game runs under is frozen onto the
// document, so a house-rule override survives a reload. Before the fix,
// toRulesState called ResolveProfile(g.RulesProfile) on every load, which
// silently reset every knob back to the profile's shipped defaults.
func TestGameRules_UsesPersistedConfigNotProfileName(t *testing.T) {
	cfg := rules.ProfileZolikClassic
	cfg.InitialMeldMinimum = 50 // house rule: zolik_classic normally has no floor
	cfg.DiscardDrawMinRound = 4 // house rule: zolik_classic normally has no gate
	cfg.MinRunSize = 5          // a knob with no legacy column at all

	g := models.Game{RulesProfile: "zolik_classic"}
	setGameRules(&g, cfg)

	got := GameRules(g)
	if got.InitialMeldMinimum != 50 {
		t.Fatalf("InitialMeldMinimum: want the persisted override 50, got %d", got.InitialMeldMinimum)
	}
	if got.DiscardDrawMinRound != 4 {
		t.Fatalf("DiscardDrawMinRound: want the persisted override 4, got %d", got.DiscardDrawMinRound)
	}
	if got.MinRunSize != 5 {
		t.Fatalf("MinRunSize: want the persisted override 5, got %d — a knob without a legacy column must still survive", got.MinRunSize)
	}
}

// TestGameRules_SurvivesAProfileConstantChanging pins the other half of the
// same bug: because the config is stored rather than re-derived, editing a
// shipped profile cannot retroactively change a game already in progress.
// Simulated here by storing one config and then asking for the rules of a
// document whose profile name says something else entirely.
func TestGameRules_SurvivesAProfileConstantChanging(t *testing.T) {
	stored := rules.ProfileContinental
	g := models.Game{RulesProfile: "continental"}
	setGameRules(&g, stored)

	// The profile registry now says this name means something different.
	g.RulesProfile = "zolik_classic"

	got := GameRules(g)
	if got.DealSize != stored.DealSize || got.MinRunSize != stored.MinRunSize {
		t.Fatalf("expected the frozen ruleset (deal %d, minRun %d), got deal %d minRun %d",
			stored.DealSize, stored.MinRunSize, got.DealSize, got.MinRunSize)
	}
}

// TestGameRules_MigratesLegacyDocuments covers documents written before the
// ruleset was persisted: they carry only a profile name plus the two scalar
// columns that were overridable at the time, and both must be honoured.
func TestGameRules_MigratesLegacyDocuments(t *testing.T) {
	legacy := models.Game{
		RulesProfile:        "continental",
		InitialMeldMinimum:  70, // host had raised the floor
		DiscardDrawMinRound: 1,  // host had unlocked pickup
		// Rules deliberately nil: this document predates the field.
	}

	got := GameRules(legacy)
	if got.InitialMeldMinimum != 70 {
		t.Fatalf("legacy InitialMeldMinimum: want 70, got %d", got.InitialMeldMinimum)
	}
	if got.DiscardDrawMinRound != 1 {
		t.Fatalf("legacy DiscardDrawMinRound: want 1, got %d", got.DiscardDrawMinRound)
	}
	if got.DealSize != rules.ProfileContinental.DealSize {
		t.Fatalf("legacy document should keep its profile's other knobs, got deal size %d", got.DealSize)
	}
}

// TestFromRulesState_PersistsTheRuleset checks the write side of the round
// trip: applying an action must store the ruleset back, so a legacy document
// is migrated in place on its first action rather than re-migrated forever.
func TestFromRulesState_PersistsTheRuleset(t *testing.T) {
	cfg := rules.ProfileContinental
	cfg.InitialMeldMinimum = 70

	g := models.Game{RulesProfile: "continental", InitialMeldMinimum: 70}
	fromRulesState(&g, rules.GameState{Rules: cfg, Status: rules.StatusActive})

	if g.Rules == nil {
		t.Fatalf("expected the ruleset to be written onto the document")
	}
	if g.Rules.InitialMeldMinimum != 70 {
		t.Fatalf("persisted InitialMeldMinimum: want 70, got %d", g.Rules.InitialMeldMinimum)
	}
	// Legacy columns stay mirrored so an older server build still reads sane
	// values off the same document.
	if g.InitialMeldMinimum != 70 {
		t.Fatalf("legacy column not mirrored: got %d", g.InitialMeldMinimum)
	}
}

// TestRulesConfigRoundTrip guards against a field being added to
// rules.RulesConfig without being added to its persisted mirror — which would
// make that knob silently unstorable, exactly the failure this whole fix is
// about.
func TestRulesConfigRoundTrip(t *testing.T) {
	original := rules.RulesConfig{
		Profile:                "custom",
		DealSize:               11,
		MinSetSize:             4,
		MinRunSize:             5,
		InitialMeldMinimum:     51,
		DiscardDrawMinRound:    2,
		DiscardPickupMode:      rules.DiscardPickupAnyFromPile,
		JokerDiscardRestricted: false,
		FixedDealCount:         3,
		StaticContract:         rules.ContractRequirement{Sets: 1, Runs: 2, RequireCleanRun: true},
		MatchEndMode:           rules.MatchEndAtScore,
		TargetScore:            321,
	}

	if got := toRulesConfig(*fromRulesConfig(original)); got != original {
		t.Fatalf("config did not survive the persistence round trip:\n want %+v\n  got %+v", original, got)
	}
}

// TestApplyRuleOverrides_NilLeavesProfileDefaults checks the lobby path: an
// absent override must not zero out the profile's own value.
func TestApplyRuleOverrides_NilLeavesProfileDefaults(t *testing.T) {
	base := rules.ProfileContinental
	got := applyRuleOverrides(base, nil, nil)
	if got != base {
		t.Fatalf("nil overrides should leave the profile untouched, got %+v", got)
	}

	floor := 0
	got = applyRuleOverrides(base, &floor, nil)
	if got.InitialMeldMinimum != 0 {
		t.Fatalf("an explicit zero override must switch the floor off, got %d", got.InitialMeldMinimum)
	}
	if got.DiscardDrawMinRound != base.DiscardDrawMinRound {
		t.Fatalf("overriding one knob must not disturb the other, got %d", got.DiscardDrawMinRound)
	}
}

// TestBuildGameStateMsg_ReportsRulesFromTheResolvedConfig makes sure the wire
// numbers come from the ruleset the engine is actually enforcing, so a client
// can never show a floor or a lock round that the server disagrees with.
func TestBuildGameStateMsg_ReportsRulesFromTheResolvedConfig(t *testing.T) {
	cfg := rules.ProfileZolikClassic
	cfg.InitialMeldMinimum = 42
	cfg.DiscardDrawMinRound = 5

	g := models.Game{
		RulesProfile: "zolik_classic",
		Players:      []models.Player{{ID: "p1"}},
		Hands:        map[string][]string{"p1": {"7H"}},
	}
	setGameRules(&g, cfg)
	// A stale legacy column must not win over the resolved ruleset.
	g.InitialMeldMinimum = 999

	msg := BuildGameStateMsg(g, "p1")
	if msg.InitialMeldMinimum != 42 {
		t.Fatalf("wire InitialMeldMinimum: want the enforced 42, got %d", msg.InitialMeldMinimum)
	}
	if msg.DiscardDrawMinRound != 5 {
		t.Fatalf("wire DiscardDrawMinRound: want the enforced 5, got %d", msg.DiscardDrawMinRound)
	}
}

// TestHouseRuleSurvivesASaveLoadCycle is the end-to-end version of the split
// brain: a host raises the meld-value floor, the game is written to a document
// and read back, and the *engine* must still enforce the raised floor. This is
// the path that actually broke — each individual conversion looked fine, but
// the load side re-derived the ruleset from the profile name and threw the
// override away.
func TestHouseRuleSurvivesASaveLoadCycle(t *testing.T) {
	// zolik_classic normally has no floor at all; the host sets one.
	floor := 35
	cfg := applyRuleOverrides(rules.ProfileZolikClassic, &floor, nil)

	g := models.Game{
		RulesProfile: "zolik_classic",
		Status:       string(rules.StatusActive),
		GameNumber:   1,
		Round:        1,
		Phase:        string(rules.PhaseMeld),
		CurrentTurn:  "p1",
		TurnOrder:    []string{"p1", "p2"},
		// 5H-6H-7H-8H is a clean run worth 26 natural points — enough to go
		// down under stock zolik_classic, short of the house rule's 35.
		Hands:       map[string][]string{"p1": {"5H", "6H", "7H", "8H", "2C"}, "p2": {}},
		Melds:       map[string][][]string{},
		MeldMeta:    map[string][]models.MeldInfo{},
		RoundReqMet: map[string]bool{"p1": false, "p2": false},
	}
	setGameRules(&g, cfg)

	// Save/load cycle: the document is what a later request reads back.
	loaded := toRulesState(g)
	if loaded.Rules.InitialMeldMinimum != 35 {
		t.Fatalf("the house rule was lost on load: floor is %d", loaded.Rules.InitialMeldMinimum)
	}

	outcome, err := rules.ApplyAction(loaded, "p1", rules.Action{
		Type:  rules.ActionLayMeld,
		Cards: []string{"5H", "6H", "7H", "8H"},
	})
	if err != nil {
		t.Fatalf("laying the run should still be allowed: %v", err)
	}
	if outcome.State.RoundReqMet["p1"] {
		t.Fatalf("a 26-point run must not clear the house rule's 35-point floor — the engine is enforcing the profile default, not the stored ruleset")
	}

	// And the ruleset is still on the document after the write-back, so the
	// next load enforces the same thing.
	next := g
	fromRulesState(&next, outcome.State)
	if GameRules(next).InitialMeldMinimum != 35 {
		t.Fatalf("the house rule was lost on save: floor is %d", GameRules(next).InitialMeldMinimum)
	}
}

// TestStockProfileStillGoesDownWithoutAHouseRule is the control for the test
// above: the same meld under the unmodified profile does put the player down,
// so the assertion there is about the floor and not about the meld.
func TestStockProfileStillGoesDownWithoutAHouseRule(t *testing.T) {
	g := models.Game{
		RulesProfile: "zolik_classic",
		Status:       string(rules.StatusActive),
		GameNumber:   1,
		Round:        1,
		Phase:        string(rules.PhaseMeld),
		CurrentTurn:  "p1",
		TurnOrder:    []string{"p1", "p2"},
		Hands:        map[string][]string{"p1": {"5H", "6H", "7H", "8H", "2C"}, "p2": {}},
		Melds:        map[string][][]string{},
		MeldMeta:     map[string][]models.MeldInfo{},
		RoundReqMet:  map[string]bool{"p1": false, "p2": false},
	}
	setGameRules(&g, rules.ProfileZolikClassic)

	outcome, err := rules.ApplyAction(toRulesState(g), "p1", rules.Action{
		Type:  rules.ActionLayMeld,
		Cards: []string{"5H", "6H", "7H", "8H"},
	})
	if err != nil {
		t.Fatalf("laying the clean run failed: %v", err)
	}
	if !outcome.State.RoundReqMet["p1"] {
		t.Fatalf("a clean run should put a player down under stock zolik_classic")
	}
}

// TestFromRulesState_DoesNotRewriteTheProfileName pins a regression found while
// reviewing the persistence change: fromRulesState briefly stamped the resolved
// config's profile name back onto the document. Because an unrecognised name
// resolves to the default ruleset, a game the host created as "custom" was
// silently relabelled "zolik_classic" on its first action — and the clients key
// their rule summaries off that name, so the label flipped mid-game.
func TestFromRulesState_DoesNotRewriteTheProfileName(t *testing.T) {
	for _, profile := range []string{"custom", "canasta", "continental", "zolik_classic"} {
		g := models.Game{RulesProfile: profile}
		setGameRules(&g, rules.ResolveProfile(profile))

		fromRulesState(&g, toRulesState(g))

		if g.RulesProfile != profile {
			t.Fatalf("profile name rewritten mid-game: %q became %q", profile, g.RulesProfile)
		}
	}
}
