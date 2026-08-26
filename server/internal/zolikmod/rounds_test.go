package zolikmod_test

import (
	"testing"

	"zolik/server/internal/module"
	"zolik/server/internal/zolikmod"
)

// The pause between deals, for the one game an offer-driver cannot play.
//
// internal/module holds every game that can be driven from its offer list to
// the same rules; Žolíky cannot be, because going out needs a meld shape the
// offer protocol deliberately does not enumerate. So it is driven here by its
// own bot instead, and asked the same questions.

func TestARummyMatchStopsBetweenDeals(t *testing.T) {
	mod := zolikmod.New()
	players := []module.PlayerRef{
		{ID: "p1", Name: "p1", IsAI: true},
		{ID: "p2", Name: "p2", IsAI: true},
	}
	cfg := module.MatchConfig{Options: module.Options{
		module.OptPauseBetweenRounds: module.OptOn,
	}}

	state, err := mod.NewMatch(cfg, players, 1)
	if err != nil {
		t.Fatalf("NewMatch: %v", err)
	}
	bot := module.BotFor(mod)

	pauses, continues, deals := 0, 0, 0
	for step := 0; step < 20000; step++ {
		if done, _, err := mod.Finished(state); err != nil {
			t.Fatalf("Finished: %v", err)
		} else if done {
			break
		}

		log := module.RoundsFor(mod, state)
		if log == nil {
			t.Fatal("Žolíky reported no round log")
		}
		if log.Paused {
			pauses++
			if len(log.WaitingFor) == 0 {
				t.Fatalf("step %d: paused and waiting on nobody", step)
			}
			// A paused table offers exactly one thing, to exactly the seats it
			// is waiting on.
			for _, p := range players {
				offers, err := mod.LegalActions(state, p.ID)
				if err != nil {
					t.Fatalf("LegalActions: %v", err)
				}
				for _, o := range offers {
					if o.Enabled && o.Verb != module.VerbContinue {
						t.Errorf("step %d: a paused table offers %q", step, o.Verb)
					}
				}
			}
			deals = len(log.Rounds)
		}

		awaited := module.AwaitedSeats(mod, state, players[0].ID, players)
		if len(awaited) == 0 {
			t.Fatalf("step %d: a live match is waiting on nobody", step)
		}
		actor := awaited[0]

		offers, err := mod.LegalActions(state, actor)
		if err != nil {
			t.Fatalf("LegalActions: %v", err)
		}
		a, ok := bot.Act(state, actor, offers)
		if !ok {
			// The module's own bot declines during an intermission — it checks
			// whose turn it is, and between deals nobody's it is. The offer
			// list carries it, which is the point of the pause being an offer.
			if a, ok = module.ChooseAction(offers, nil); !ok {
				t.Fatalf("step %d: %s has nothing it may do", step, actor)
			}
		}
		if a.Verb == module.VerbContinue {
			continues++
		}
		next, _, err := mod.Apply(state, actor, a)
		if err != nil {
			fallback, ok := module.ChooseAction(offers, nil)
			if !ok {
				t.Fatalf("step %d: refused (%v) with no fallback", step, err)
			}
			if next, _, err = mod.Apply(state, actor, fallback); err != nil {
				t.Fatalf("step %d: fallback refused too: %v", step, err)
			}
		}
		state = next
	}

	if pauses == 0 {
		t.Fatal("a rummy match asked to pause between deals never stopped")
	}
	if continues == 0 {
		t.Error("the table paused and nobody ever agreed to go on")
	}
	if deals == 0 {
		t.Error("the table paused with no deal behind it to look at")
	}
	t.Logf("paused across %d deals, %d agreements to go on", deals, continues)
}

// TestARummyMatchStillPlaysStraightThrough — the pause is a table setting, and
// turning it off has to leave the game exactly as it was.
func TestARummyMatchStillPlaysStraightThrough(t *testing.T) {
	mod := zolikmod.New()
	players := []module.PlayerRef{
		{ID: "p1", Name: "p1", IsAI: true},
		{ID: "p2", Name: "p2", IsAI: true},
	}
	cfg := module.MatchConfig{Options: module.Options{
		module.OptPauseBetweenRounds: module.OptOff,
	}}

	state, err := mod.NewMatch(cfg, players, 1)
	if err != nil {
		t.Fatalf("NewMatch: %v", err)
	}
	bot := module.BotFor(mod)

	for step := 0; step < 20000; step++ {
		if done, _, err := mod.Finished(state); err != nil || done {
			break
		}
		if log := module.RoundsFor(mod, state); log != nil && log.Paused {
			t.Fatalf("step %d: the table stopped although the pause is off", step)
		}
		actor := module.ActiveSeat(mod, state, players[0].ID, players)
		if actor == "" {
			break
		}
		offers, _ := mod.LegalActions(state, actor)
		a, ok := bot.Act(state, actor, offers)
		if !ok {
			if a, ok = module.ChooseAction(offers, nil); !ok {
				break
			}
		}
		next, _, err := mod.Apply(state, actor, a)
		if err != nil {
			fb, ok := module.ChooseAction(offers, nil)
			if !ok {
				break
			}
			if next, _, err = mod.Apply(state, actor, fb); err != nil {
				break
			}
		}
		state = next
	}
}
