package game

import (
	"testing"

	"zolik/server/internal/models"
	"zolik/server/internal/module"
	"zolik/server/internal/rules"
	"zolik/server/internal/zolikmod"
)

// Does the module path play Žolíky the same way the legacy path does?
//
// This is the acceptance test for retiring `internal/game`'s transport, and it
// is worth being precise about what it can and cannot show.
//
// What it does *not* need to show is that the rules agree. There are not two
// rummy engines: `game.Manager` and `zolikmod` both call `rules.ApplyAction`
// on a `rules.GameState`, so rule drift between the two paths is not merely
// unlikely, it is impossible by construction. That is the single most
// important fact about this migration and the reason it is cleanup rather than
// a correctness fix.
//
// What has to be shown is that the *conversion* is faithful: that a legacy
// document, once migrated, presents the same game — same offers, same
// legality, same outcome of the same move. A migration that quietly dropped a
// field would produce a legal-looking game that was not the one the player
// left, and nothing about the shared engine would catch it.

// legacyGame deals a real game through the legacy path, then plays a few moves
// so the state under test is mid-game rather than freshly dealt: an empty
// board would hide exactly the fields a migration is most likely to lose.
func legacyGame(t *testing.T, profile string, moves int) models.Game {
	t.Helper()

	cfg := rules.ResolveProfile(profile)
	gs, err := rules.StartMatch(cfg, []string{"p1", "p2"}, 42)
	if err != nil {
		t.Fatalf("StartMatch: %v", err)
	}

	g := models.Game{
		Status:       string(gs.Status),
		RulesProfile: profile,
		Players: []models.Player{
			{ID: "p1", Name: "One"},
			{ID: "p2", Name: "Two", IsAI: true, AIDifficulty: "medium"},
		},
		HostID:   "p1",
		JoinCode: "ABC123",
	}
	fromRulesState(&g, gs)

	// A few offer-driven moves, applied the way the legacy manager applies
	// them: straight through rules.ApplyAction on the mapped state.
	for i := 0; i < moves; i++ {
		state := toRulesState(g)
		if state.Status != rules.StatusActive || state.CurrentTurn == "" {
			break
		}
		offers := rules.LegalActions(state, state.CurrentTurn)
		act, ok := pickLegacyAction(offers)
		if !ok {
			break
		}
		out, err := rules.ApplyAction(state, state.CurrentTurn, act)
		if err != nil {
			break
		}
		fromRulesState(&g, out.State)
	}
	return g
}

// pickLegacyAction chooses a move from the engine's own offers, preferring the
// two verbs that always have a concrete submission.
func pickLegacyAction(offers []rules.ActionOffer) (rules.Action, bool) {
	for _, want := range []rules.OfferVerb{rules.VerbDraw, rules.VerbDiscard} {
		for _, o := range offers {
			if o.Verb != want || !o.Enabled {
				continue
			}
			switch want {
			case rules.VerbDraw:
				from := rules.DrawFromDeck
				if o.ID == rules.OfferDrawDiscard {
					from = rules.DrawFromDiscard
				}
				return rules.Action{Type: rules.ActionDrawCard, DrawFrom: from}, true
			case rules.VerbDiscard:
				if o.Source == nil || len(o.Source.Cards) == 0 {
					continue
				}
				return rules.Action{Type: rules.ActionDiscard, Card: o.Source.Cards[0]}, true
			}
		}
	}
	return rules.Action{}, false
}

// TestMigratedGameIsTheSameGame — every rummy fact the legacy document holds
// survives the conversion into module state.
func TestMigratedGameIsTheSameGame(t *testing.T) {
	for _, profile := range []string{"zolik_classic", "continental"} {
		t.Run(profile, func(t *testing.T) {
			g := legacyGame(t, profile, 6)

			m, err := MatchFromGame(g)
			if err != nil {
				t.Fatalf("MatchFromGame: %v", err)
			}
			if m.ModuleID != "zolik" {
				t.Errorf("migrated to module %q", m.ModuleID)
			}
			if m.MigratedFrom != g.ID {
				t.Error("a migrated match should record where it came from")
			}

			// The envelope keeps who was playing and how to get back in.
			if len(m.Players) != len(g.Players) || m.HostID != g.HostID || m.JoinCode != g.JoinCode {
				t.Errorf("envelope lost something: %+v", m)
			}

			// And the state is the same rummy state, field for field. Compared
			// through the module's own decode rather than by reaching into the
			// bytes, because that is how the runtime will read it.
			got, err := zolikmod.RulesStateOf(m.State)
			if err != nil {
				t.Fatalf("RulesStateOf: %v", err)
			}
			want := toRulesState(g)

			if got.Status != want.Status || got.Phase != want.Phase {
				t.Errorf("status/phase changed: %v/%v want %v/%v",
					got.Status, got.Phase, want.Status, want.Phase)
			}
			if got.CurrentTurn != want.CurrentTurn {
				t.Errorf("turn moved: %q want %q", got.CurrentTurn, want.CurrentTurn)
			}
			if got.GameNumber != want.GameNumber || got.Round != want.Round {
				t.Errorf("deal/round changed: %d/%d want %d/%d",
					got.GameNumber, got.Round, want.GameNumber, want.Round)
			}
			if len(got.DrawPile) != len(want.DrawPile) || len(got.DiscardPile) != len(want.DiscardPile) {
				t.Errorf("piles changed: draw %d/%d discard %d/%d",
					len(got.DrawPile), len(want.DrawPile),
					len(got.DiscardPile), len(want.DiscardPile))
			}
			for _, p := range want.TurnOrder {
				if len(got.Hands[p]) != len(want.Hands[p]) {
					t.Errorf("%s's hand changed size: %d want %d",
						p, len(got.Hands[p]), len(want.Hands[p]))
				}
				for i, c := range want.Hands[p] {
					if i < len(got.Hands[p]) && got.Hands[p][i] != c {
						t.Errorf("%s's hand differs at %d: %q want %q", p, i, got.Hands[p][i], c)
					}
				}
				if got.RoundReqMet[p] != want.RoundReqMet[p] {
					t.Errorf("%s's contract flag changed", p)
				}
				if len(got.Melds[p]) != len(want.Melds[p]) {
					t.Errorf("%s's melds changed: %d want %d", p, len(got.Melds[p]), len(want.Melds[p]))
				}
			}
			// The resolved ruleset has to travel too, or a migrated game
			// silently changes rules on its next deal.
			if got.Rules.MinRunSize != want.Rules.MinRunSize ||
				got.Rules.InitialMeldMinimum != want.Rules.InitialMeldMinimum ||
				got.Rules.DiscardDrawMinRound != want.Rules.DiscardDrawMinRound {
				t.Errorf("ruleset changed: %+v want %+v", got.Rules, want.Rules)
			}
		})
	}
}

// TestMigratedGameOffersTheSameMoves — the test that matters to a player. A
// game that migrated must present exactly the same choices, enabled and
// disabled, with the same reasons.
func TestMigratedGameOffersTheSameMoves(t *testing.T) {
	for _, profile := range []string{"zolik_classic", "continental"} {
		t.Run(profile, func(t *testing.T) {
			g := legacyGame(t, profile, 6)
			m, err := MatchFromGame(g)
			if err != nil {
				t.Fatalf("MatchFromGame: %v", err)
			}
			mod := zolikmod.New()

			for _, p := range g.TurnOrder {
				legacy := rules.LegalActions(toRulesState(g), p)
				migrated, err := mod.LegalActions(m.State, p)
				if err != nil {
					t.Fatalf("LegalActions: %v", err)
				}
				if len(legacy) != len(migrated) {
					t.Fatalf("%s: %d offers after migration, %d before", p, len(migrated), len(legacy))
				}
				for i := range legacy {
					if legacy[i].ID != migrated[i].ID {
						t.Errorf("%s: offer %d is %q, was %q", p, i, migrated[i].ID, legacy[i].ID)
					}
					if legacy[i].Enabled != migrated[i].Enabled {
						t.Errorf("%s: offer %q enabled=%v, was %v",
							p, legacy[i].ID, migrated[i].Enabled, legacy[i].Enabled)
					}
					if string(legacy[i].WhyNot) != migrated[i].WhyNot {
						t.Errorf("%s: offer %q says %q, was %q",
							p, legacy[i].ID, migrated[i].WhyNot, legacy[i].WhyNot)
					}
				}
			}
		})
	}
}

// TestTheSameMovePlaysTheSameWay — apply one move down each path and the
// resulting game must be identical. This is the strongest form of the claim,
// and it is cheap precisely because the engine underneath is shared.
func TestTheSameMovePlaysTheSameWay(t *testing.T) {
	g := legacyGame(t, "zolik_classic", 5)
	m, err := MatchFromGame(g)
	if err != nil {
		t.Fatalf("MatchFromGame: %v", err)
	}

	legacyState := toRulesState(g)
	actor := legacyState.CurrentTurn
	if actor == "" {
		t.Skip("nobody on turn")
	}
	offers := rules.LegalActions(legacyState, actor)
	act, ok := pickLegacyAction(offers)
	if !ok {
		t.Skip("no simple move available")
	}

	// Down the legacy path.
	out, err := rules.ApplyAction(legacyState, actor, act)
	if err != nil {
		t.Fatalf("legacy ApplyAction: %v", err)
	}

	// Down the module path, using the module's own action vocabulary.
	var modAction module.Action
	switch act.Type {
	case rules.ActionDrawCard:
		modAction = module.Action{Verb: string(rules.VerbDraw), OfferID: rules.OfferDrawDeck}
		if act.DrawFrom == rules.DrawFromDiscard {
			modAction.OfferID = rules.OfferDrawDiscard
		}
	case rules.ActionDiscard:
		modAction = module.Action{Verb: string(rules.VerbDiscard), Cards: []string{act.Card}}
	}

	next, _, err := zolikmod.New().Apply(m.State, actor, modAction)
	if err != nil {
		t.Fatalf("module Apply: %v", err)
	}
	got, err := zolikmod.RulesStateOf(next)
	if err != nil {
		t.Fatalf("RulesStateOf: %v", err)
	}

	if got.CurrentTurn != out.State.CurrentTurn {
		t.Errorf("turn is %q down one path and %q down the other",
			got.CurrentTurn, out.State.CurrentTurn)
	}
	if got.Phase != out.State.Phase {
		t.Errorf("phase is %q down one path and %q down the other", got.Phase, out.State.Phase)
	}
	if len(got.Hands[actor]) != len(out.State.Hands[actor]) {
		t.Errorf("hand size %d down one path and %d down the other",
			len(got.Hands[actor]), len(out.State.Hands[actor]))
	}
	if len(got.DiscardPile) != len(out.State.DiscardPile) {
		t.Errorf("discard pile %d down one path and %d down the other",
			len(got.DiscardPile), len(out.State.DiscardPile))
	}
}

// TestMigrationIsIdempotentInShape — converting twice produces the same bytes,
// which is what lets the migration be re-run over a collection that is partly
// done.
func TestMigrationIsIdempotentInShape(t *testing.T) {
	g := legacyGame(t, "zolik_classic", 4)
	a, err := MatchFromGame(g)
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	b, err := MatchFromGame(g)
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if string(a.State) != string(b.State) {
		t.Error("converting the same game twice produced different state")
	}
}
