package module_test

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"zolik/server/internal/canasta"
	"zolik/server/internal/ginrummy"
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
	// rounds is whether this game keeps a round-by-round history. Prší is the
	// one that does not, and the reason is recorded rather than left as an
	// omission somebody later "fixes": it is a single deal that ends the
	// moment a hand empties, so a one-row table is a worse answer than none.
	rounds bool
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
			rounds:   true,
			mod:      zolikmod.New(),
			players:  refs("p1", "p2"),
			prefer:   []string{"draw", "discard"},
			finishes: false,
		},
		{
			name:     "prsi",
			rounds:   false,
			mod:      prsi.New(),
			players:  refs("p1", "p2", "p3"),
			prefer:   []string{"play_card", "pass", "draw"},
			finishes: true,
		},
		{
			name:    "canasta",
			rounds:  true,
			mod:     canasta.New(),
			players: refs("p1", "p2"),
			// High enough that a lucky first deal (a natural canasta can
			// swing 900+ points on its own) can never end the match in one
			// deal on its own — which of these two seats deals first now
			// varies by seed, and TestAPausedTableSaysSo needs at least a
			// second deal to ever observe a pause.
			cfg:      module.MatchConfig{Options: module.Options{"targetScore": 1500}},
			prefer:   []string{"lay_meld", "lay_off", "take_pile", "draw", "discard"},
			finishes: true,
		},
		{
			name:     "holdem",
			rounds:   true,
			mod:      holdem.New(),
			players:  refs("p1", "p2", "p3"),
			cfg:      module.MatchConfig{Variation: "timed"},
			prefer:   []string{"call", "check", "raise", "fold"},
			finishes: true,
		},
		{
			name:     "ginrummy",
			rounds:   true,
			mod:      ginrummy.New(),
			players:  refs("p1", "p2"),
			prefer:   []string{"knock", "lay_off", "finish_layoff", "draw", "discard", "pass"},
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

// TestOffersOnScreenTogetherCanBeToldApart — two live controls that read the
// same and do different things.
//
// A client labels a control from its verb, which is fine until a module offers
// the same verb twice at once: Žolíky draws from the deck and from the discard
// pile, Canasta offers a meld per rank and a capture per way of capturing. The
// player sees a row of identical buttons and has to guess. Nothing else catches
// it — every offer is individually correct, and the collision only exists on
// screen.
//
// An offer may distinguish itself by its LabelKey or by the Facts printed on
// it; what it may not do is be identical to another one that is live at the
// same moment.
func TestOffersOnScreenTogetherCanBeToldApart(t *testing.T) {
	for _, g := range allModules() {
		t.Run(g.name, func(t *testing.T) {
			state, err := g.mod.NewMatch(g.cfg, g.players, 5)
			if err != nil {
				t.Fatalf("NewMatch: %v", err)
			}

			// Over several turns, because most collisions need a position a
			// deal has to be played into — a partnership has to have melded
			// before there are two lay-offs to confuse.
			for turn := 0; turn < 12; turn++ {
				for _, viewer := range g.players {
					offers, err := g.mod.LegalActions(state, viewer.ID)
					if err != nil {
						t.Fatalf("LegalActions: %v", err)
					}
					seen := map[string]string{}
					for _, o := range offers {
						// Disabled ones count too. They stay on screen with
						// their reason — that is the whole point of sending
						// them — so four greyed-out buttons all reading "Undo"
						// are just as much a guess as four live ones.
						caption := o.LabelKey
						if caption == "" {
							caption = o.Verb
						}
						for _, f := range o.Facts {
							caption += "|" + f.LabelKey + "=" + f.Value
						}
						if first, clash := seen[caption]; clash {
							t.Errorf("offers %q and %q both read %q for %s — a player sees two identical controls",
								first, o.ID, caption, viewer.ID)
						}
						seen[caption] = o.ID
					}
				}

				next, done, err := stepAnyPlayer(g.mod, state, g.players)
				if err != nil || done {
					break
				}
				state = next
			}
		})
	}
}

// stepAnyPlayer plays one move for whoever has one, so a test can walk a deal
// forward without knowing whose turn it is or what the move means.
func stepAnyPlayer(m module.GameModule, state module.State, players []module.PlayerRef) (module.State, bool, error) {
	for _, p := range players {
		offers, err := m.LegalActions(state, p.ID)
		if err != nil {
			return state, false, err
		}
		action, ok := module.ChooseAction(offers, nil)
		if !ok {
			continue
		}
		next, _, err := m.Apply(state, p.ID, action)
		if err != nil {
			return state, false, err
		}
		over, _, err := m.Finished(next)
		if err != nil {
			return state, false, err
		}
		return next, over, nil
	}
	return state, true, nil
}

// TestEveryOfferLandsSomewhereThatIsDrawn — an offer that names a zone id no
// zone has is a place to drop a card that a player can never hit.
//
// Nothing at runtime notices: the move still works from a button, the id is
// just a string nobody resolves, and the only symptom is a drop target that
// silently refuses. That makes it exactly the sort of thing to check here
// rather than discover in a browser. The same goes for a target meld id with
// no group drawn under it.
//
// Deliberately checked against the *viewer's own* view, since that is the one
// a client hit-tests against, and against a real dealt position rather than a
// constructed one.
func TestEveryOfferLandsSomewhereThatIsDrawn(t *testing.T) {
	for _, g := range allModules() {
		t.Run(g.name, func(t *testing.T) {
			state, err := g.mod.NewMatch(g.cfg, g.players, 11)
			if err != nil {
				t.Fatalf("NewMatch: %v", err)
			}
			for _, viewer := range g.players {
				vm, err := g.mod.View(state, viewer.ID)
				if err != nil {
					t.Fatalf("View: %v", err)
				}
				zones := map[string]bool{}
				groups := map[string]bool{}
				for _, z := range vm.Zones {
					zones[z.ID] = true
					for _, gr := range z.Groups {
						groups[gr.ID] = true
					}
				}

				offers, err := g.mod.LegalActions(state, viewer.ID)
				if err != nil {
					t.Fatalf("LegalActions: %v", err)
				}
				for _, o := range offers {
					for label, sel := range map[string]*module.Selector{
						"source": o.Source, "target": o.Target,
					} {
						if sel == nil {
							continue
						}
						if sel.ZoneID != "" && !zones[sel.ZoneID] {
							t.Errorf("offer %q %s names zone %q, which %s's view does not draw",
								o.ID, label, sel.ZoneID, viewer.ID)
						}
						if sel.MeldID != "" && !groups[sel.MeldID] {
							t.Errorf("offer %q %s names meld %q, which %s's view does not draw",
								o.ID, label, sel.MeldID, viewer.ID)
						}
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
				// The invariant the runtime actually depends on: a live match
				// is waiting on somebody, and the seats it is waiting on are
				// the seats it is offering moves to.
				//
				// This used to demand exactly one active seat, which held right
				// up until a match stopped between rounds and waited on
				// everybody at once. "The awaited set equals the enabled set"
				// is the stronger claim and the one the bot loop, the
				// suspension path and the offer driver are each built on.
				if len(active) == 0 {
					t.Fatalf("step %d: a live match is waiting on nobody", step)
				}
				enabledSeats := map[string]bool{}
				for _, p := range g.players {
					offers, err := g.mod.LegalActions(state, p.ID)
					if err != nil {
						t.Fatalf("LegalActions(%s): %v", p.ID, err)
					}
					for _, o := range offers {
						if o.Enabled {
							enabledSeats[p.ID] = true
							break
						}
					}
				}
				for _, id := range active {
					if !enabledSeats[id] {
						t.Fatalf("step %d: seat %q is marked active but is offered nothing", step, id)
					}
				}
				for id := range enabledSeats {
					if !contains(active, id) {
						t.Fatalf("step %d: seat %q is offered a move but is not marked active", step, id)
					}
				}

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

// TestEveryModuleKeepsAScoreboard — one screen shows who is ahead at rummy,
// canasta and poker, so every module has to answer the question in the same
// shape even though none of them measure the same thing.
func TestEveryModuleKeepsAScoreboard(t *testing.T) {
	for _, g := range allModules() {
		t.Run(g.name, func(t *testing.T) {
			state, err := g.mod.NewMatch(g.cfg, g.players, 17)
			if err != nil {
				t.Fatalf("NewMatch: %v", err)
			}
			standings := module.StandingsFor(g.mod, state)
			if len(standings) != len(g.players) {
				t.Fatalf("%d standings for %d players", len(standings), len(g.players))
			}

			seen := map[string]bool{}
			for _, s := range standings {
				if s.PlayerID == "" {
					t.Error("a standing names nobody")
				}
				if seen[s.PlayerID] {
					t.Errorf("%q appears twice", s.PlayerID)
				}
				seen[s.PlayerID] = true
				if s.Rank < 1 {
					t.Errorf("%q has rank %d", s.PlayerID, s.Rank)
				}
				if s.LabelKey == "" {
					t.Errorf("%q's score has no unit; a client cannot label it", s.PlayerID)
				}
			}
			// Ranks must be ordered and start at 1, or a scoreboard renders in
			// the wrong order.
			if standings[0].Rank != 1 {
				t.Errorf("the first row has rank %d, want 1", standings[0].Rank)
			}
			for i := 1; i < len(standings); i++ {
				if standings[i].Rank < standings[i-1].Rank {
					t.Errorf("ranks go backwards: %+v", standings)
				}
				if standings[i].Score > standings[i-1].Score {
					t.Errorf("scores are not sorted: %+v", standings)
				}
			}
		})
	}
}

// TestEveryModuleWritesItsRules — a lobby's "see the rules" screen renders
// from this and nothing else, so every module has to have something to say,
// and has to say it in keys rather than sentences it wrote itself.
// checkKey holds a label to the one rule every label in this protocol keeps:
// it is a key a client looks up, never a sentence a server wrote.
//
// Hoisted out of the rules test so the same rule reaches every new labelled
// surface — a round's name, a round's facts — rather than being restated, or
// quietly not restated, at each one.
func checkKey(t *testing.T, where, key string) {
	t.Helper()
	if key == "" {
		t.Errorf("%s: empty label key", where)
		return
	}
	if !strings.Contains(key, ".") {
		t.Errorf("%s: key %q looks like rendered text, not a message key", where, key)
	}
	if strings.Contains(key, " ") {
		t.Errorf("%s: key %q contains a space — a key, not a sentence", where, key)
	}
}

func TestEveryModuleWritesItsRules(t *testing.T) {
	for _, g := range allModules() {
		t.Run(g.name, func(t *testing.T) {
			p, ok := g.mod.(module.RulesProvider)
			if !ok {
				t.Fatalf("%s does not implement module.RulesProvider", g.name)
			}
			sections, err := p.Rules(g.cfg)
			if err != nil {
				t.Fatalf("Rules: %v", err)
			}
			if len(sections) == 0 {
				t.Fatal("a module's rules must have at least one section")
			}
			for i, s := range sections {
				checkKey(t, fmt.Sprintf("section %d title", i), s.TitleKey)
				if len(s.Items) == 0 {
					t.Errorf("section %d (%s) has no items", i, s.TitleKey)
				}
				for j, item := range s.Items {
					checkKey(t, fmt.Sprintf("section %d item %d", i, j), item.LabelKey)
				}
			}
		})
	}
}

// TestTiesShareARank is the property Canasta and poker both need: a partnership
// and a split pot are two players who genuinely came first.
func TestTiesShareARank(t *testing.T) {
	got := module.RankByScore([]string{"a", "b", "c"},
		func(id string) int {
			if id == "c" {
				return 1
			}
			return 5
		}, "unit")

	if got[0].Rank != 1 || got[1].Rank != 1 {
		t.Errorf("level players got ranks %d and %d, want both 1", got[0].Rank, got[1].Rank)
	}
	if !got[0].Won || !got[1].Won {
		t.Error("both level leaders should be marked as winning")
	}
	// The next rank skips, as places do: two firsts and then a third.
	if got[2].Rank != 3 {
		t.Errorf("the third player has rank %d, want 3", got[2].Rank)
	}
	if got[2].Won {
		t.Error("the trailing player should not be marked as winning")
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

// TestTheScoreboardAgreesWithTheWinner — a module that names a winner and a
// module that ranks the players are the same module, and they have to agree.
//
// Žolíky is why this exists. Its scoreboard ranked players by the cards they
// were *holding when the match ended* rather than by their score across the
// deals, so the row at the top of the table was not necessarily the player the
// engine had just declared the winner — and the runtime recorded that row's
// score as the player's result for the whole match. Nothing failed. The number
// was simply wrong, everywhere it was used, for as long as it existed.
//
// Stated as "every winner has rank 1" rather than "rank 1 is the winner"
// because a draw is real: several seats can share the top rank, and a module
// that ends level names all of them.
func TestTheScoreboardAgreesWithTheWinner(t *testing.T) {
	for _, g := range allModules() {
		if !g.finishes {
			continue // driven to a finish in the module's own suite instead
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
			assertScoreboardAgrees(t, g.mod, final, res.Winners)
		})
	}
}

// assertScoreboardAgrees is the check itself, exported to the modules that
// cannot be driven by offers alone and must play themselves to a finish.
func assertScoreboardAgrees(t *testing.T, mod module.GameModule, final module.State, winners []string) {
	t.Helper()

	standings := module.StandingsFor(mod, final)
	if len(standings) == 0 {
		return // a module that keeps no scoreboard has nothing to disagree with
	}

	byID := map[string]module.Standing{}
	for _, s := range standings {
		byID[s.PlayerID] = s
	}
	for _, w := range winners {
		s, ok := byID[w]
		if !ok {
			t.Errorf("the engine named %q the winner and the scoreboard does not list them", w)
			continue
		}
		if s.Rank != 1 {
			t.Errorf("the engine named %q the winner and the scoreboard ranks them %d (score %d)",
				w, s.Rank, s.Score)
		}
		if !s.Won {
			t.Errorf("the engine named %q the winner and the scoreboard does not mark them won", w)
		}
	}

	// And the other way: nobody sits at the top of a finished scoreboard
	// without the engine having named them.
	won := map[string]bool{}
	for _, w := range winners {
		won[w] = true
	}
	for _, s := range standings {
		if s.Rank == 1 && len(winners) > 0 && !won[s.PlayerID] {
			t.Errorf("the scoreboard ranks %q first and the engine did not name them a winner", s.PlayerID)
		}
	}
}

// TestOnlyTheGamesWithRoundsKeepThem — the opt-in is a decision, and a decision
// is worth asserting so it is not quietly reversed in either direction.
func TestOnlyTheGamesWithRoundsKeepThem(t *testing.T) {
	for _, g := range allModules() {
		_, keeps := g.mod.(module.Rounded)
		if keeps != g.rounds {
			t.Errorf("%s: implements Rounded=%v, table says %v", g.name, keeps, g.rounds)
		}
		state, err := g.mod.NewMatch(g.cfg, g.players, 3)
		if err != nil {
			t.Fatalf("%s: NewMatch: %v", g.name, err)
		}
		got := module.RoundsFor(g.mod, state)
		if (got != nil) != g.rounds {
			t.Errorf("%s: RoundsFor gave %v, table says rounds=%v", g.name, got, g.rounds)
		}
		if got != nil {
			// A fresh match has rounds and none of them finished. Nil and empty
			// are different answers and both are real, so the empty one must
			// not arrive as `null`.
			if got.Rounds == nil {
				t.Errorf("%s: a fresh match sends a null round list", g.name)
			}
			checkKey(t, g.name+" round label", got.LabelKey)
		}
	}
}

// TestARoundLogIsArithmetic — the property that makes a round table renderable
// at all: every seat appears in every round, and the totals are the deltas
// added up. It catches the orientation slip where a module negates its delta
// and forgets its total, which no amount of eyeballing a scoreboard finds.
func TestARoundLogIsArithmetic(t *testing.T) {
	for _, g := range allModules() {
		if !g.rounds || !g.finishes {
			continue
		}
		t.Run(g.name, func(t *testing.T) {
			final := playOut(t, g)
			log := module.RoundsFor(g.mod, final)
			if log == nil {
				t.Fatal("a game that keeps rounds returned no log")
			}
			if len(log.Rounds) == 0 {
				t.Fatal("a finished match recorded no rounds")
			}
			checkKey(t, "round label", log.LabelKey)

			running := map[string]int{}
			started := map[string]bool{}
			for n, r := range log.Rounds {
				if n > 0 && r.Number <= log.Rounds[n-1].Number {
					t.Errorf("round %d is numbered %d, after %d", n, r.Number, log.Rounds[n-1].Number)
				}
				if len(r.Scores) == 0 {
					t.Errorf("round %d scored nobody", r.Number)
				}
				for _, f := range r.Facts {
					checkKey(t, "round fact", f.LabelKey)
				}
				for _, sc := range r.Scores {
					for _, f := range sc.Facts {
						checkKey(t, "round score fact", f.LabelKey)
					}
					if !started[sc.PlayerID] {
						// The first round a seat appears in sets its baseline;
						// a game may seat somebody late or bust them out early.
						started[sc.PlayerID] = true
						running[sc.PlayerID] = sc.Total
						continue
					}
					if want := running[sc.PlayerID] + sc.Delta; sc.Total != want {
						t.Errorf("round %d: %s totals %d, but %d + %d is %d",
							r.Number, sc.PlayerID, sc.Total, running[sc.PlayerID], sc.Delta, want)
					}
					running[sc.PlayerID] = sc.Total
				}
			}
		})
	}
}

// TestTheLastRoundAgreesWithTheScoreboard — the round table and the standings
// are two views of one number, so on a finished match they have to end at the
// same place. Three lines that hold the higher-is-better orientation across a
// game nobody has written yet.
func TestTheLastRoundAgreesWithTheScoreboard(t *testing.T) {
	for _, g := range allModules() {
		if !g.rounds || !g.finishes {
			continue
		}
		t.Run(g.name, func(t *testing.T) {
			final := playOut(t, g)
			log := module.RoundsFor(g.mod, final)
			if log == nil || len(log.Rounds) == 0 {
				t.Skip("no rounds to compare")
			}
			// The last total each seat posted, whenever it last appeared.
			last := map[string]int{}
			for _, r := range log.Rounds {
				for _, sc := range r.Scores {
					last[sc.PlayerID] = sc.Total
				}
			}
			for _, s := range module.StandingsFor(g.mod, final) {
				total, played := last[s.PlayerID]
				if !played {
					continue
				}
				if total != s.Score {
					t.Errorf("%s ends the round table on %d and the scoreboard on %d",
						s.PlayerID, total, s.Score)
				}
			}
		})
	}
}

// TestARoundLogNeverNamesACard — a round log is public by construction: it
// takes no viewer and the same values are bound for a permanent record. Hold'em
// is the reason it is a rule — a hand everybody folded out of is never shown —
// but it is cheap to hold every game to.
func TestARoundLogNeverNamesACard(t *testing.T) {
	for _, g := range allModules() {
		if !g.rounds || !g.finishes {
			continue
		}
		t.Run(g.name, func(t *testing.T) {
			log := module.RoundsFor(g.mod, playOut(t, g))
			if log == nil {
				t.Skip("no log")
			}
			blob, err := json.Marshal(log)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			for _, card := range []string{"JOKER1", "JOKER2", "\"AS\"", "\"KH\"", "\"QD\"", "\"TC\""} {
				if strings.Contains(string(blob), card) {
					t.Errorf("the round log names %s; it is public and permanent", card)
				}
			}
		})
	}
}

// playOut drives a game to its finish through the offer list alone.
func playOut(t *testing.T, g hosted) module.State {
	t.Helper()
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
	return final
}

func contains(ids []string, id string) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

// TestAPausingGameStillPlaysItselfOut — the whole claim that an intermission is
// an ordinary turn rather than a new primitive.
//
// The driver has never heard of a round, an intermission or the continue verb.
// It reads offers and presses one. If a game that stops between every round
// still plays to a finish under it, and the verb shows up in the tally, then
// the pause needed nothing taught to anything: not to this driver, not to a
// bot, not to a client.
func TestAPausingGameStillPlaysItselfOut(t *testing.T) {
	for _, g := range allModules() {
		if !g.rounds || !g.finishes || !pauses(g.mod) {
			continue
		}
		t.Run(g.name, func(t *testing.T) {
			cfg := g.cfg
			if cfg.Options == nil {
				cfg.Options = module.Options{}
			}
			// Ask for the pause explicitly rather than relying on this game's
			// default, so the test says what it is testing.
			cfg.Options[module.OptPauseBetweenRounds] = module.OptOn

			state, err := g.mod.NewMatch(cfg, g.players, 5)
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
				t.Fatalf("a pausing match did not finish in %d actions", res.Actions)
			}

			log := module.RoundsFor(g.mod, final)
			if log == nil || len(log.Rounds) < 2 {
				t.Skip("this match had no round to pause between")
			}
			if res.Verbs[module.VerbContinue] == 0 {
				t.Errorf("%d rounds were played and nobody ever agreed to go on — the table never paused",
					len(log.Rounds))
			}
		})
	}
}

// TestAPausedTableSaysSo — a client puts a results screen up because the module
// says the table is paused, never because it noticed the controls went quiet.
// So the module has to say it, and say who it is waiting on.
func TestAPausedTableSaysSo(t *testing.T) {
	for _, g := range allModules() {
		if !g.rounds || !g.finishes || !pauses(g.mod) {
			continue
		}
		t.Run(g.name, func(t *testing.T) {
			cfg := g.cfg
			if cfg.Options == nil {
				cfg.Options = module.Options{}
			}
			cfg.Options[module.OptPauseBetweenRounds] = module.OptOn

			state, err := g.mod.NewMatch(cfg, g.players, 5)
			if err != nil {
				t.Fatalf("NewMatch: %v", err)
			}

			paused := false
			for step := 0; step < 8000; step++ {
				if done, _, err := g.mod.Finished(state); err != nil || done {
					break
				}
				if log := module.RoundsFor(g.mod, state); log != nil && log.Paused {
					paused = true
					if len(log.WaitingFor) == 0 {
						t.Fatalf("step %d: the table is paused and waiting for nobody", step)
					}
					// Everyone it is waiting on is offered the way to go on,
					// and nobody else is offered anything at all.
					for _, p := range g.players {
						offers, err := g.mod.LegalActions(state, p.ID)
						if err != nil {
							t.Fatalf("LegalActions: %v", err)
						}
						enabled := false
						for _, o := range offers {
							if o.Enabled {
								enabled = true
								if o.Verb != module.VerbContinue {
									t.Errorf("a paused table offers %q", o.Verb)
								}
							}
						}
						if want := contains(log.WaitingFor, p.ID); enabled != want {
							t.Errorf("step %d: %s enabled=%v, waitingFor says %v",
								step, p.ID, enabled, want)
						}
					}
				}
				actor := module.ActiveSeat(g.mod, state, g.players[0].ID, g.players)
				if actor == "" {
					break
				}
				offers, err := g.mod.LegalActions(state, actor)
				if err != nil {
					break
				}
				a, ok := module.ChooseAction(offers, g.prefer)
				if !ok {
					break
				}
				next, _, err := g.mod.Apply(state, actor, a)
				if err != nil {
					break
				}
				state = next
			}
			if !paused {
				t.Error("a game asked to pause between rounds never reported a pause")
			}
		})
	}
}

// pauses reports a module offering the pause as a table setting at all. A game
// that does not declare the option is not expected to honour it — asking it to
// would be asserting a setting it never advertised.
func pauses(m module.GameModule) bool {
	return m.Descriptor().Option(module.OptPauseBetweenRounds) != nil
}
