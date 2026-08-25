package zolikmod

import (
	"testing"

	"zolik/server/internal/module"
	"zolik/server/internal/prsi"
	"zolik/server/internal/rules"
)

func refs(ids ...string) []module.PlayerRef {
	out := make([]module.PlayerRef, 0, len(ids))
	for _, id := range ids {
		out = append(out, module.PlayerRef{ID: id, Name: id})
	}
	return out
}

// TestOneDriverPlaysBothGames is the point of the whole exercise.
//
// module.PlayWithOffers reads a module's offer list and nothing else — it
// never decodes State, never names a rank, a suit, a meld or a phase. Here it
// is handed two games that share almost no vocabulary and asked to play both.
//
// Žolíky is not expected to *finish* under it: going out requires assembling a
// contract, and the driver has no idea what a meld is, which is exactly the
// limitation the offer protocol documents (a meld is a shape offer, not an
// enumerated one). What it must do is take real turns without ever being
// refused an action the module offered it. Prší, whose every move is a single
// enumerated card, it plays to a winner.
func TestOneDriverPlaysBothGames(t *testing.T) {
	games := []struct {
		name       string
		mod        module.GameModule
		players    []module.PlayerRef
		prefer     []string
		mustFinish bool
		minActions int
	}{
		{
			name:    "zolik",
			mod:     New(),
			players: refs("p1", "p2"),
			// Draw then discard: enough to take real turns without needing to
			// understand contracts.
			prefer:     []string{string(rules.VerbDraw), string(rules.VerbDiscard)},
			mustFinish: false,
			minActions: 20,
		},
		{
			name:       "prsi",
			mod:        prsi.New(),
			players:    refs("p1", "p2", "p3"),
			prefer:     []string{prsi.VerbPlay, prsi.VerbPass, prsi.VerbDraw},
			mustFinish: true,
			minActions: 5,
		},
	}

	for _, g := range games {
		t.Run(g.name, func(t *testing.T) {
			state, err := g.mod.NewMatch(module.MatchConfig{}, g.players, 7)
			if err != nil {
				t.Fatalf("NewMatch: %v", err)
			}
			_, res, err := module.PlayWithOffers(g.mod, state, g.players, module.DriverOptions{
				MaxActions: 400,
				Prefer:     g.prefer,
			})
			if err != nil {
				t.Fatalf("%s: %v", g.name, err)
			}
			if res.Actions < g.minActions {
				t.Errorf("only %d actions applied, expected at least %d", res.Actions, g.minActions)
			}
			if g.mustFinish && !res.Finished {
				t.Errorf("expected the driver to reach a winner, got %d actions", res.Actions)
			}
			t.Logf("%s: %d actions, finished=%v winner=%v verbs=%v",
				g.name, res.Actions, res.Finished, res.Winners, res.Verbs)
		})
	}
}

// TestBothModulesSatisfyTheSameContract keeps the interface honest about the
// things every module must get right, rather than testing each game twice.
func TestBothModulesSatisfyTheSameContract(t *testing.T) {
	mods := map[string]module.GameModule{"zolik": New(), "prsi": prsi.New()}

	for name, m := range mods {
		t.Run(name, func(t *testing.T) {
			d := m.Descriptor()
			if d.ID == "" || d.Label == "" {
				t.Error("a module must name itself")
			}
			if d.MinPlayers < 2 || d.MaxPlayers < d.MinPlayers {
				t.Errorf("player range %d..%d makes no sense", d.MinPlayers, d.MaxPlayers)
			}
			for _, v := range d.Variations {
				for opt, val := range v.Defaults {
					spec := d.Option(opt)
					if spec == nil {
						t.Errorf("variation %q defaults unknown option %q", v.ID, opt)
						continue
					}
					if !spec.Allows(val) {
						t.Errorf("variation %q defaults %s=%d, which its own schema disallows",
							v.ID, opt, val)
					}
				}
			}

			players := refs("p1", "p2")
			state, err := m.NewMatch(module.MatchConfig{}, players, 3)
			if err != nil {
				t.Fatalf("NewMatch: %v", err)
			}

			// A fresh match is not over.
			if done, _, err := m.Finished(state); err != nil || done {
				t.Errorf("a freshly dealt match reports done=%v err=%v", done, err)
			}

			// Exactly one player is on turn, and every module answers both.
			onTurn := 0
			for _, p := range players {
				offers, err := m.LegalActions(state, p.ID)
				if err != nil {
					t.Fatalf("LegalActions(%s): %v", p.ID, err)
				}
				if len(offers) == 0 {
					t.Errorf("%s got an empty offer list", p.ID)
				}
				enabled := false
				for _, o := range offers {
					if o.ID == "" || o.Verb == "" {
						t.Errorf("offer with no id or verb: %+v", o)
					}
					if !o.Enabled && o.WhyNot == "" {
						t.Errorf("offer %q is disabled with no reason", o.ID)
					}
					if o.Enabled && o.WhyNot != "" {
						t.Errorf("offer %q is enabled but carries a reason %q", o.ID, o.WhyNot)
					}
					enabled = enabled || o.Enabled
				}
				if enabled {
					onTurn++
				}
			}
			if onTurn != 1 {
				t.Errorf("%d players have enabled offers at once, want exactly 1", onTurn)
			}

			// Every module hides every other player's cards. This is the one
			// contract term with a security consequence, so it is checked for
			// all modules rather than trusted per game.
			for _, viewer := range players {
				vm, err := m.View(state, viewer.ID)
				if err != nil {
					t.Fatalf("View(%s): %v", viewer.ID, err)
				}
				if len(vm.Zones) == 0 {
					t.Errorf("%s got an empty board", viewer.ID)
				}
				for _, z := range vm.Zones {
					if z.Kind == module.ZoneHand && z.OwnerID != "" && z.OwnerID != viewer.ID && len(z.Cards) > 0 {
						t.Errorf("%s can see %s's cards: %v", viewer.ID, z.OwnerID, z.Cards)
					}
				}
			}
		})
	}
}

// TestApplyNeverMutatesTheCallersState holds every module to the property that
// made the opaque-state decision worth making.
//
// The rummy engine mutates in place and needed a regression test to stop a
// read-only-looking function corrupting a document. Behind the module
// interface, state is bytes: Apply decodes a fresh value, so no amount of
// speculative calling can reach the caller's copy.
func TestApplyNeverMutatesTheCallersState(t *testing.T) {
	mods := map[string]module.GameModule{"zolik": New(), "prsi": prsi.New()}
	for name, m := range mods {
		t.Run(name, func(t *testing.T) {
			players := refs("p1", "p2")
			state, err := m.NewMatch(module.MatchConfig{}, players, 11)
			if err != nil {
				t.Fatalf("NewMatch: %v", err)
			}
			before := string(state)

			for _, p := range players {
				offers, _ := m.LegalActions(state, p.ID)
				for _, o := range offers {
					a := module.Action{OfferID: o.ID, Verb: o.Verb}
					if o.Source != nil && len(o.Source.Cards) > 0 {
						a.Cards = []string{o.Source.Cards[0]}
					}
					for _, ps := range o.Params {
						if len(ps.Choices) > 0 {
							a.Params = map[string]string{ps.Name: ps.Choices[0].Value}
						}
					}
					_, _, _ = m.Apply(state, p.ID, a)
				}
			}
			if string(state) != before {
				t.Error("Apply mutated the state it was given")
			}
		})
	}
}

// TestRegistry_HostsBothGames checks the runtime can hold more than one.
func TestRegistry_HostsBothGames(t *testing.T) {
	reg := module.NewRegistry(New(), prsi.New())

	if got := reg.IDs(); len(got) != 2 {
		t.Fatalf("registry holds %v, want two modules", got)
	}
	for _, id := range []string{"zolik", "prsi"} {
		if reg.Get(id) == nil {
			t.Errorf("registry has no module %q", id)
		}
	}
	if reg.Get("marias") != nil {
		t.Error("registry invented a module it was never given")
	}
	// A game-picker renders straight off this.
	for _, d := range reg.Descriptors() {
		if d.ID == "" || d.Label == "" || d.MinPlayers == 0 {
			t.Errorf("descriptor is not renderable: %+v", d)
		}
	}
}
