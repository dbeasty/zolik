package prsi

import (
	"fmt"
	"testing"

	"zolik/server/internal/module"
)

// The falsification tests.
//
// module.PlayWithOffers is a driver that may read a module's offer list and
// ViewModel and nothing else — it never decodes State, never names a rank or a
// suit, never mentions sevens, aces or wilds. It was written against Žolíky.
// If it can play Prší to completion, the offer protocol is game-agnostic in
// the only sense that matters.

func refs(ids ...string) []module.PlayerRef {
	out := make([]module.PlayerRef, 0, len(ids))
	for _, id := range ids {
		out = append(out, module.PlayerRef{ID: id, Name: id})
	}
	return out
}

func TestGameAgnosticDriver_PlaysPrsiToCompletion(t *testing.T) {
	m := New()
	players := refs("p1", "p2", "p3")

	completed := 0
	for seed := int64(1); seed <= 25; seed++ {
		t.Run(fmt.Sprintf("seed-%d", seed), func(t *testing.T) {
			state, err := m.NewMatch(module.MatchConfig{}, players, seed)
			if err != nil {
				t.Fatalf("NewMatch: %v", err)
			}
			// Prefer playing over drawing: a driver that always drew would
			// technically "play" a shedding game without ever finishing one.
			_, res, err := module.PlayWithOffers(m, state, players, module.DriverOptions{
				MaxActions: 1500,
				Prefer:     []string{VerbPlay, VerbPass, VerbDraw},
			})
			if err != nil {
				t.Fatalf("seed %d: %v", seed, err)
			}
			if res.Actions == 0 {
				t.Fatal("the offer list never produced a single applicable action")
			}
			if res.Finished {
				completed++
				if res.WinnerID == "" {
					t.Error("finished with no winner")
				}
			}
			t.Logf("seed %d: %d actions, finished=%v winner=%s verbs=%v",
				seed, res.Actions, res.Finished, res.WinnerID, res.Verbs)
		})
	}

	if completed == 0 {
		t.Error("no seed ever reached a finished game — the offers did not carry the driver to a win")
	}
}

func TestGameAgnosticDriver_ExercisesEveryVerb(t *testing.T) {
	// A run that only ever played cards would not prove the draw and skip
	// paths are reachable from the offers alone.
	m := New()
	players := refs("p1", "p2", "p3")
	seen := map[string]int{}

	for seed := int64(1); seed <= 40; seed++ {
		state, err := m.NewMatch(module.MatchConfig{}, players, seed)
		if err != nil {
			t.Fatalf("NewMatch: %v", err)
		}
		_, res, err := module.PlayWithOffers(m, state, players, module.DriverOptions{
			MaxActions: 1500,
			Prefer:     []string{VerbPlay, VerbPass, VerbDraw},
		})
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		for verb, n := range res.Verbs {
			seen[verb] += n
		}
	}

	for _, verb := range []string{VerbPlay, VerbDraw, VerbPass} {
		if seen[verb] == 0 {
			t.Errorf("verb %q was never reachable from the offer list across 40 games", verb)
		}
	}
	t.Logf("verbs exercised: %v", seen)
}

func TestLegalActions_NeverOffersAnActionTheEngineRefuses(t *testing.T) {
	// The safety half of the same guarantee, swept broadly: across real
	// gameplay, every card every offer advertises must be accepted.
	m := New()
	players := refs("p1", "p2")
	checked := 0

	for seed := int64(1); seed <= 15; seed++ {
		state, err := m.NewMatch(module.MatchConfig{}, players, seed)
		if err != nil {
			t.Fatalf("NewMatch: %v", err)
		}
		_, _, err = module.PlayWithOffers(m, state, players, module.DriverOptions{
			MaxActions: 400,
			Prefer:     []string{VerbPlay, VerbPass, VerbDraw},
			OnAction:   func(_ string, _ module.Action) {},
		})
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}

		// Re-walk the same match, checking every advertised card at each step.
		state, _ = m.NewMatch(module.MatchConfig{}, players, seed)
		for step := 0; step < 200; step++ {
			done, _, _ := m.Finished(state)
			if done {
				break
			}
			for _, p := range players {
				offers, err := m.LegalActions(state, p.ID)
				if err != nil {
					t.Fatalf("LegalActions: %v", err)
				}
				for _, o := range offers {
					if !o.Enabled || o.Source == nil {
						continue
					}
					for _, card := range o.Source.Cards {
						a := module.Action{Verb: o.Verb, Cards: []string{card}}
						for _, ps := range o.Params {
							if a.Params == nil {
								a.Params = map[string]string{}
							}
							a.Params[ps.Name] = ps.Choices[0].Value
						}
						if _, _, err := m.Apply(state, p.ID, a); err != nil {
							t.Errorf("offer %q advertised %s for %s but the engine refused: %v",
								o.ID, card, p.ID, err)
						}
						checked++
					}
				}
			}
			// Advance one step so the sweep sees varied states.
			actor := ""
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
			var next module.State
			advanced := false
			for _, verb := range []string{VerbPlay, VerbPass, VerbDraw} {
				for _, o := range offers {
					if o.Verb != verb || !o.Enabled {
						continue
					}
					a := module.Action{Verb: o.Verb}
					if o.Source != nil && o.Source.MinCards > 0 {
						if len(o.Source.Cards) == 0 {
							continue
						}
						a.Cards = []string{o.Source.Cards[0]}
					}
					for _, ps := range o.Params {
						if a.Params == nil {
							a.Params = map[string]string{}
						}
						a.Params[ps.Name] = ps.Choices[0].Value
					}
					if s2, _, err := m.Apply(state, actor, a); err == nil {
						next, advanced = s2, true
					}
					break
				}
				if advanced {
					break
				}
			}
			if !advanced {
				break
			}
			state = next
		}
	}

	if checked == 0 {
		t.Fatal("no advertised cards were checked — the sweep is vacuous")
	}
	t.Logf("verified %d advertised card actions against the engine", checked)
}

func TestView_HidesEveryOtherPlayersHand(t *testing.T) {
	// The whole anti-cheat surface for this game. Asserted directly rather
	// than by reading the code, because View is where a module could leak.
	m := New()
	players := refs("p1", "p2", "p3")
	state, err := m.NewMatch(module.MatchConfig{}, players, 7)
	if err != nil {
		t.Fatalf("NewMatch: %v", err)
	}
	s := stateOf(t, state)

	for _, viewer := range players {
		vm, err := m.View(state, viewer.ID)
		if err != nil {
			t.Fatalf("View(%s): %v", viewer.ID, err)
		}
		for _, z := range vm.Zones {
			if z.Kind != module.ZoneHand || z.OwnerID == viewer.ID {
				continue
			}
			if len(z.Cards) != 0 {
				t.Errorf("%s can see %s's cards: %v", viewer.ID, z.OwnerID, z.Cards)
			}
			if z.Count != len(s.Hands[z.OwnerID]) {
				t.Errorf("%s sees %s holding %d cards, actually %d",
					viewer.ID, z.OwnerID, z.Count, len(s.Hands[z.OwnerID]))
			}
		}
	}
}

func TestView_ShowsOnlyTheTopOfTheDiscardPile(t *testing.T) {
	m := New()
	raw := withState(t, func(s *GameState) {
		s.DiscardPile = []string{"2H", "3H", "9S"}
	})
	vm, err := m.View(raw, "p1")
	if err != nil {
		t.Fatalf("View: %v", err)
	}
	for _, z := range vm.Zones {
		if z.ID != "discard" {
			continue
		}
		if len(z.Cards) != 1 || z.Cards[0].Card != "9S" {
			t.Errorf("discard zone shows %v, want just the top card 9S", z.Cards)
		}
	}
}

func TestDescriptor_IsCoherent(t *testing.T) {
	d := New().Descriptor()
	if d.ID == "" || d.Label == "" {
		t.Fatal("a module must name itself")
	}
	if d.MinPlayers < 2 || d.MaxPlayers < d.MinPlayers {
		t.Errorf("player range %d..%d makes no sense", d.MinPlayers, d.MaxPlayers)
	}
	// Every variation's defaults must be values the option schema allows,
	// or a lobby cannot save the state it is already showing.
	for _, v := range d.Variations {
		for name, val := range v.Defaults {
			spec := d.Option(name)
			if spec == nil {
				t.Errorf("variation %q defaults unknown option %q", v.ID, name)
				continue
			}
			if !spec.Allows(val) {
				t.Errorf("variation %q defaults %s=%d, which the schema disallows", v.ID, name, val)
			}
		}
	}
	if err := d.ValidateOptions(map[string]*int{OptHandSize: intp(99)}); err == nil {
		t.Error("an undeclared hand size was accepted")
	}
	if err := d.ValidateOptions(map[string]*int{OptHandSize: intp(defaultHandSize)}); err != nil {
		t.Errorf("the default hand size was rejected: %v", err)
	}
}

func intp(v int) *int { return &v }
