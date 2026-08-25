package match

import (
	"testing"

	"zolik/server/internal/canasta"
	"zolik/server/internal/holdem"
	"zolik/server/internal/module"
	"zolik/server/internal/prsi"
	"zolik/server/internal/zolikmod"
)

// Bots, checked for every game the server hosts.
//
// The loop itself needs Mongo and is covered by the e2e suite; what is worth
// pinning here is the part that used to be missing — that every module has a
// bot at all, that it produces moves the engine accepts, and that it can carry
// a game rather than stalling on the first turn it does not understand.

type botCase struct {
	name    string
	mod     module.GameModule
	players []module.PlayerRef
	cfg     module.MatchConfig
	// carries is whether a table of nothing but bots can play this game to a
	// result. Žolíky cannot: going out needs a meld shape, and while its bot is
	// the real heuristic agent, this harness drives it without the action-log
	// history the agent uses to pick melds well.
	carries bool
}

func botRefs(ids ...string) []module.PlayerRef {
	out := make([]module.PlayerRef, 0, len(ids))
	for _, id := range ids {
		out = append(out, module.PlayerRef{ID: id, Name: id, IsAI: true})
	}
	return out
}

func botCases() []botCase {
	return []botCase{
		{name: "zolik", mod: zolikmod.New(), players: botRefs("b1", "b2"), carries: false},
		{name: "prsi", mod: prsi.New(), players: botRefs("b1", "b2", "b3"), carries: true},
		{
			name: "canasta", mod: canasta.New(), players: botRefs("b1", "b2"),
			cfg: module.MatchConfig{Options: module.Options{"targetScore": 500}}, carries: true,
		},
		{
			name: "holdem", mod: holdem.New(), players: botRefs("b1", "b2", "b3"),
			cfg: module.MatchConfig{Variation: "timed"}, carries: true,
		},
	}
}

// TestEveryModuleHasABot — the claim add-bot now makes. A module that declares
// none still gets one, which is the point of the default.
func TestEveryModuleHasABot(t *testing.T) {
	for _, tc := range botCases() {
		t.Run(tc.name, func(t *testing.T) {
			if module.BotFor(tc.mod) == nil {
				t.Fatal("no bot, not even the default")
			}
		})
	}
}

// TestBotsProduceMovesTheEngineAccepts is the property the runtime's loop
// depends on. A bot that proposes illegal moves would burn its whole step
// budget without ever ending its turn — which is exactly what the rummy runtime
// had a game-specific recovery path for, and what this checks does not happen.
func TestBotsProduceMovesTheEngineAccepts(t *testing.T) {
	for _, tc := range botCases() {
		t.Run(tc.name, func(t *testing.T) {
			state, err := tc.mod.NewMatch(tc.cfg, tc.players, 9)
			if err != nil {
				t.Fatalf("NewMatch: %v", err)
			}
			bot := module.BotFor(tc.mod)

			accepted, refused := 0, 0
			for step := 0; step < 300; step++ {
				done, _, err := tc.mod.Finished(state)
				if err != nil {
					t.Fatalf("Finished: %v", err)
				}
				if done {
					break
				}
				actor := module.ActiveSeat(tc.mod, state, tc.players[0].ID, tc.players)
				if actor == "" {
					t.Fatalf("step %d: nobody is on turn but the match is not over", step)
				}

				offers, err := tc.mod.LegalActions(state, actor)
				if err != nil {
					t.Fatalf("LegalActions: %v", err)
				}
				action, ok := bot.Act(state, actor, offers)
				if !ok {
					// Declining is allowed; the runtime falls back to the offer
					// list. What is not allowed is declining every single time,
					// which the count below catches.
					action, ok = module.ChooseAction(offers, nil)
					if !ok {
						break
					}
				}

				next, _, err := tc.mod.Apply(state, actor, action)
				if err != nil {
					refused++
					// Recover the way the runtime does, so one bad suggestion
					// does not end the test.
					fallback, ok := module.ChooseAction(offers, nil)
					if !ok {
						break
					}
					next, _, err = tc.mod.Apply(state, actor, fallback)
					if err != nil {
						t.Fatalf("step %d: even the offer-list fallback was refused: %v", step, err)
					}
				} else {
					accepted++
				}
				state = next
			}

			if accepted < 10 {
				t.Errorf("only %d bot moves were accepted; the bot is not carrying the game", accepted)
			}
			// A few refusals are tolerable — a heuristic agent may propose a
			// meld the validator dislikes — but a bot that is wrong more often
			// than right is not a bot.
			if refused > accepted {
				t.Errorf("%d moves refused against %d accepted", refused, accepted)
			}
			t.Logf("%s: %d accepted, %d refused", tc.name, accepted, refused)
		})
	}
}

// TestBotsFinishAGameOfTheirOwn — a table of nothing but bots reaches a result,
// which is what a human sitting down against three of them needs to be true.
func TestBotsFinishAGameOfTheirOwn(t *testing.T) {
	for _, tc := range botCases() {
		if !tc.carries {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			state, err := tc.mod.NewMatch(tc.cfg, tc.players, 4)
			if err != nil {
				t.Fatalf("NewMatch: %v", err)
			}
			bot := module.BotFor(tc.mod)

			var winners []string
			finished := false
			for step := 0; step < 5000; step++ {
				done, w, err := tc.mod.Finished(state)
				if err != nil {
					t.Fatalf("Finished: %v", err)
				}
				if done {
					finished, winners = true, w
					break
				}
				actor := module.ActiveSeat(tc.mod, state, tc.players[0].ID, tc.players)
				if actor == "" {
					break
				}
				offers, _ := tc.mod.LegalActions(state, actor)
				action, ok := bot.Act(state, actor, offers)
				if !ok {
					action, ok = module.ChooseAction(offers, nil)
					if !ok {
						break
					}
				}
				next, _, err := tc.mod.Apply(state, actor, action)
				if err != nil {
					fallback, ok := module.ChooseAction(offers, nil)
					if !ok {
						break
					}
					if next, _, err = tc.mod.Apply(state, actor, fallback); err != nil {
						break
					}
				}
				state = next
			}

			if !finished {
				t.Fatal("a table of bots never reached a result")
			}
			if len(winners) == 0 {
				t.Error("finished naming nobody")
			}
		})
	}
}

// TestActiveSeatMatchesTheOffers — the runtime uses Seat.Active to decide whose
// turn it is, so a module whose view disagreed with its own offers would have
// its bots driving the wrong seat.
func TestActiveSeatMatchesTheOffers(t *testing.T) {
	for _, tc := range botCases() {
		t.Run(tc.name, func(t *testing.T) {
			state, err := tc.mod.NewMatch(tc.cfg, tc.players, 6)
			if err != nil {
				t.Fatalf("NewMatch: %v", err)
			}
			for step := 0; step < 40; step++ {
				done, _, err := tc.mod.Finished(state)
				if err != nil || done {
					break
				}
				actor := module.ActiveSeat(tc.mod, state, tc.players[0].ID, tc.players)
				if actor == "" {
					t.Fatalf("step %d: no active seat in a live match", step)
				}
				offers, err := tc.mod.LegalActions(state, actor)
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
					t.Fatalf("step %d: %q is the active seat but is offered nothing", step, actor)
				}
				// And nobody else is.
				for _, p := range tc.players {
					if p.ID == actor {
						continue
					}
					other, _ := tc.mod.LegalActions(state, p.ID)
					for _, o := range other {
						if o.Enabled {
							t.Fatalf("step %d: %q is offered %q while %q is the active seat",
								step, p.ID, o.ID, actor)
						}
					}
				}

				action, ok := module.ChooseAction(offers, nil)
				if !ok {
					break
				}
				next, _, err := tc.mod.Apply(state, actor, action)
				if err != nil {
					break
				}
				state = next
			}
		})
	}
}
