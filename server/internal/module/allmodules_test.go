package module_test

import (
	"encoding/json"
	"strconv"
	"testing"

	"zolik/server/internal/canasta"
	"zolik/server/internal/holdem"
	"zolik/server/internal/module"
	"zolik/server/internal/prsi"
	"zolik/server/internal/zolikmod"
)

// The contract, checked once against every game the server hosts.
//
// This file is the point of the whole architecture. Before it, each module's
// suite re-stated the shared terms in its own words and a fifth game could
// quietly satisfy none of them; here a module that breaks a term the runtime
// depends on fails somebody else's test, which is exactly what a contract is
// for.
//
// It lives in package module_test rather than module so it may import the
// modules themselves. The production package still imports no game.

type hosted struct {
	name string
	mod  module.GameModule
	// players is a table size this game accepts.
	players []module.PlayerRef
	// cfg keeps the match short enough to finish inside a test.
	cfg module.MatchConfig
	// prefer is a play style that gets this game moving; a driver with no
	// preferences plays every game badly.
	prefer []string
	// finishes is whether an offer-only driver can play this game to a result.
	// Žolíky is the one that cannot, and the reason is documented rather than
	// hidden: going out needs a meld *shape* the offer protocol deliberately
	// does not enumerate (extensibility-plan.md §1.1).
	finishes bool
}

func refs(ids ...string) []module.PlayerRef {
	out := make([]module.PlayerRef, 0, len(ids))
	for _, id := range ids {
		out = append(out, module.PlayerRef{ID: id, Name: id})
	}
	return out
}

func allModules() []hosted {
	return []hosted{
		{
			name:     "zolik",
			mod:      zolikmod.New(),
			players:  refs("p1", "p2"),
			prefer:   []string{"draw", "discard"},
			finishes: false,
		},
		{
			name:     "prsi",
			mod:      prsi.New(),
			players:  refs("p1", "p2", "p3"),
			prefer:   []string{"play_card", "pass", "draw"},
			finishes: true,
		},
		{
			name:     "canasta",
			mod:      canasta.New(),
			players:  refs("p1", "p2"),
			cfg:      module.MatchConfig{Options: module.Options{"targetScore": 500}},
			prefer:   []string{"lay_meld", "lay_off", "take_pile", "draw", "discard"},
			finishes: true,
		},
		{
			name:     "holdem",
			mod:      holdem.New(),
			players:  refs("p1", "p2", "p3"),
			cfg:      module.MatchConfig{Variation: "timed"},
			prefer:   []string{"call", "check", "raise", "fold"},
			finishes: true,
		},
	}
}

// TestEveryModuleDescribesItself — a lobby renders the game picker and the
// new-match form from this and nothing else, so a gap here is a screen that
// cannot be drawn.
func TestEveryModuleDescribesItself(t *testing.T) {
	seen := map[string]bool{}
	for _, g := range allModules() {
		t.Run(g.name, func(t *testing.T) {
			d := g.mod.Descriptor()
			if d.ID == "" || d.Label == "" {
				t.Fatal("a module must name itself")
			}
			if seen[d.ID] {
				t.Fatalf("two modules claim the id %q", d.ID)
			}
			seen[d.ID] = true

			if d.MinPlayers < 2 || d.MaxPlayers < d.MinPlayers {
				t.Errorf("player range %d..%d makes no sense", d.MinPlayers, d.MaxPlayers)
			}
			for _, v := range d.Variations {
				if v.ID == "" || v.Label == "" {
					t.Errorf("variation %+v is not renderable", v)
				}
				for opt, val := range v.Defaults {
					spec := d.Option(opt)
					if spec == nil {
						t.Errorf("variation %q defaults an undeclared option %q", v.ID, opt)
						continue
					}
					if !spec.Allows(val) {
						t.Errorf("variation %q defaults %s=%d, which is not an allowed value", v.ID, opt, val)
					}
				}
			}
			for _, o := range d.Options {
				if o.Name == "" || len(o.Choices) == 0 {
					t.Errorf("option %+v cannot be rendered as a control", o)
				}
			}
		})
	}
}

// TestEveryModuleHidesOtherPlayersCards is the one contract term with a
// security consequence, and the reason it is checked here rather than four
// times: a fifth game must not be able to leak just by forgetting.
func TestEveryModuleHidesOtherPlayersCards(t *testing.T) {
	for _, g := range allModules() {
		t.Run(g.name, func(t *testing.T) {
			state, err := g.mod.NewMatch(g.cfg, g.players, 7)
			if err != nil {
				t.Fatalf("NewMatch: %v", err)
			}
			for _, viewer := range g.players {
				vm, err := g.mod.View(state, viewer.ID)
				if err != nil {
					t.Fatalf("View: %v", err)
				}
				for _, z := range vm.Zones {
					if z.Kind != module.ZoneHand {
						continue
					}
					if z.OwnerID == viewer.ID {
						continue
					}
					if len(z.Cards) > 0 {
						t.Errorf("%s can see %s's cards: %v", viewer.ID, z.OwnerID, z.Cards)
					}
				}
			}
		})
	}
}

// TestEveryModuleAgreesWithItsOwnOffers — an enabled offer the engine then
// refuses is a control that does nothing, and no client-side check would catch
// it. Every module is held to this, on positions that really occur.
func TestEveryModuleAgreesWithItsOwnOffers(t *testing.T) {
	for _, g := range allModules() {
		t.Run(g.name, func(t *testing.T) {
			state, err := g.mod.NewMatch(g.cfg, g.players, 3)
			if err != nil {
				t.Fatalf("NewMatch: %v", err)
			}

			checked := 0
			for step := 0; step < 150; step++ {
				done, _, err := g.mod.Finished(state)
				if err != nil {
					t.Fatalf("Finished: %v", err)
				}
				if done {
					break
				}

				var actor string
				for _, p := range g.players {
					offers, err := g.mod.LegalActions(state, p.ID)
					if err != nil {
						t.Fatalf("LegalActions: %v", err)
					}
					for _, o := range offers {
						if !o.Enabled {
							if o.WhyNot == "" {
								t.Errorf("offer %q is off with no reason given", o.ID)
							}
							continue
						}
						if actor == "" {
							actor = p.ID
						}
						a, ok := module.SubmissionFor(o)
						if !ok {
							// A composite offer is a combination only a person
							// can compose. Declining is correct; what would be
							// wrong is an offer that is neither composite nor
							// submittable, which is a control nothing can press.
							if !o.Composite {
								t.Errorf("offer %q is on, is not composite, and describes no submission: %+v", o.ID, o)
							}
							continue
						}
						if _, _, err := g.mod.Apply(state, p.ID, a); err != nil {
							t.Fatalf("offer %q was enabled but the engine refused %+v: %v",
								o.ID, a, err)
						}
						checked++
					}
				}
				if actor == "" {
					break
				}

				offers, _ := g.mod.LegalActions(state, actor)
				a, ok := module.ChooseAction(offers, g.prefer)
				if !ok {
					break
				}
				next, _, err := g.mod.Apply(state, actor, a)
				if err != nil {
					t.Fatalf("refused an offered action: %v", err)
				}
				state = next
			}
			if checked < 10 {
				t.Errorf("only %d offers checked; the test proved little", checked)
			}
		})
	}
}

// TestEveryModelSeatsItsPlayers — Seats is what a generic client draws a table
// from, so every module has to fill it in, and exactly one seat may be active
// while a match is running.
//
// The active-seat rule matters beyond rendering: it is the property the runtime
// relies on to work out whose turn it is, and Hold'em shipped a real bug — every
// betting street silently skipping its first player — that the offer-driven
// driver could not see, because a driver plays whoever is offered a move. This
// checks the module's own claim instead.
func TestEveryModuleSeatsItsPlayers(t *testing.T) {
	for _, g := range allModules() {
		t.Run(g.name, func(t *testing.T) {
			state, err := g.mod.NewMatch(g.cfg, g.players, 11)
			if err != nil {
				t.Fatalf("NewMatch: %v", err)
			}

			for step := 0; step < 40; step++ {
				done, _, err := g.mod.Finished(state)
				if err != nil || done {
					break
				}
				vm, err := g.mod.View(state, g.players[0].ID)
				if err != nil {
					t.Fatalf("View: %v", err)
				}
				if len(vm.Seats) != len(g.players) {
					t.Fatalf("step %d: %d seats for %d players", step, len(vm.Seats), len(g.players))
				}

				var active []string
				for _, seat := range vm.Seats {
					if seat.PlayerID == "" {
						t.Fatalf("step %d: a seat names nobody", step)
					}
					if seat.Active {
						active = append(active, seat.PlayerID)
					}
				}
				if len(active) != 1 {
					t.Fatalf("step %d: %d seats are active, want exactly one (%v)", step, len(active), active)
				}

				// And the seat the module says is active is the one it is
				// actually offering moves to.
				offers, err := g.mod.LegalActions(state, active[0])
				if err != nil {
					t.Fatalf("LegalActions: %v", err)
				}
				enabled := false
				for _, o := range offers {
					if o.Enabled {
						enabled = true
					}
				}
				if !enabled {
					t.Fatalf("step %d: seat %q is marked active but is offered nothing", step, active[0])
				}

				a, ok := module.ChooseAction(offers, g.prefer)
				if !ok {
					break
				}
				next, _, err := g.mod.Apply(state, active[0], a)
				if err != nil {
					t.Fatalf("step %d: refused an offered action: %v", step, err)
				}
				state = next
			}
		})
	}
}

// TestEveryModuleNamesItsWinners — the shape that replaced a single winner id,
// checked for the games that can actually be driven to a result.
func TestEveryModuleNamesItsWinners(t *testing.T) {
	for _, g := range allModules() {
		if !g.finishes {
			continue
		}
		t.Run(g.name, func(t *testing.T) {
			state, err := g.mod.NewMatch(g.cfg, g.players, 5)
			if err != nil {
				t.Fatalf("NewMatch: %v", err)
			}
			final, res, err := module.PlayWithOffers(g.mod, state, g.players, module.DriverOptions{
				MaxActions: 8000, Prefer: g.prefer,
			})
			if err != nil {
				t.Fatalf("%v", err)
			}
			if !res.Finished {
				t.Fatalf("did not finish in %d actions", res.Actions)
			}
			if len(res.Winners) == 0 {
				t.Fatal("finished naming nobody")
			}

			known := map[string]bool{}
			for _, p := range g.players {
				known[p.ID] = true
			}
			for _, w := range res.Winners {
				if !known[w] {
					t.Errorf("winner %q is not at this table", w)
				}
			}

			// A finished match offers nobody anything.
			for _, p := range g.players {
				offers, err := g.mod.LegalActions(final, p.ID)
				if err != nil {
					t.Fatalf("LegalActions: %v", err)
				}
				for _, o := range offers {
					if o.Enabled {
						t.Errorf("%s is still offered %q after the match ended", p.ID, o.ID)
					}
				}
			}
		})
	}
}

// TestEveryModuleStateSurvivesARoundTrip — the runtime persists State as bytes
// and hands it back later, so a module whose state does not survive JSON is one
// that works in memory and breaks in Mongo.
func TestEveryModuleStateSurvivesARoundTrip(t *testing.T) {
	for _, g := range allModules() {
		t.Run(g.name, func(t *testing.T) {
			state, err := g.mod.NewMatch(g.cfg, g.players, 13)
			if err != nil {
				t.Fatalf("NewMatch: %v", err)
			}
			// Through a generic decode and re-encode, which is what the
			// database does to it.
			var anyState any
			if err := json.Unmarshal(state, &anyState); err != nil {
				t.Fatalf("state is not valid JSON: %v", err)
			}
			round, err := json.Marshal(anyState)
			if err != nil {
				t.Fatalf("re-marshal: %v", err)
			}

			before, err := g.mod.View(state, g.players[0].ID)
			if err != nil {
				t.Fatalf("View: %v", err)
			}
			after, err := g.mod.View(round, g.players[0].ID)
			if err != nil {
				t.Fatalf("View after round trip: %v", err)
			}
			b, _ := json.Marshal(before)
			a, _ := json.Marshal(after)
			if string(b) != string(a) {
				t.Error("the board changed by being persisted and read back")
			}
		})
	}
}

// TestNumericParametersAreUsable checks the protocol addition poker forced, at
// the level any client would meet it: a declared range has to be one a control
// can render and a submission can satisfy.
func TestNumericParametersAreUsable(t *testing.T) {
	m := holdem.New()
	players := refs("p1", "p2", "p3")
	state, err := m.NewMatch(module.MatchConfig{Variation: "timed"}, players, 2)
	if err != nil {
		t.Fatalf("NewMatch: %v", err)
	}

	found := 0
	for step := 0; step < 60; step++ {
		done, _, err := m.Finished(state)
		if err != nil || done {
			break
		}
		var actor string
		for _, p := range players {
			offers, _ := m.LegalActions(state, p.ID)
			for _, o := range offers {
				if o.Enabled {
					actor = p.ID
					break
				}
			}
			if actor != "" {
				break
			}
		}
		if actor == "" {
			break
		}

		offers, _ := m.LegalActions(state, actor)
		for _, o := range offers {
			for _, p := range o.Params {
				if p.Kind != module.ParamKindInt {
					continue
				}
				found++
				if p.Min > p.Max {
					t.Fatalf("offer %q declares an empty range %d..%d", o.ID, p.Min, p.Max)
				}
				if p.Default < p.Min || p.Default > p.Max {
					t.Fatalf("offer %q defaults to %d, outside %d..%d", o.ID, p.Default, p.Min, p.Max)
				}
				// Both ends of the declared range must actually be accepted;
				// a range whose edges are refused is a control that lies.
				for _, v := range []int{p.Min, p.Max} {
					a := module.Action{OfferID: o.ID, Verb: o.Verb,
						Params: map[string]string{p.Name: strconv.Itoa(v)}}
					if _, _, err := m.Apply(state, actor, a); err != nil {
						t.Fatalf("offer %q declared %d..%d but refused %d: %v",
							o.ID, p.Min, p.Max, v, err)
					}
				}
			}
		}

		a, ok := module.ChooseAction(offers, []string{"call", "check", "raise", "fold"})
		if !ok {
			break
		}
		next, _, err := m.Apply(state, actor, a)
		if err != nil {
			t.Fatalf("refused an offered action: %v", err)
		}
		state = next
	}
	if found == 0 {
		t.Error("no numeric parameter was ever offered; the addition is untested")
	}
}
