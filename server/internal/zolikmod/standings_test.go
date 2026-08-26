package zolikmod_test

import (
	"testing"

	"zolik/server/internal/module"
	"zolik/server/internal/zolikmod"
)

// The scoreboard and the winner, checked against a match that was really
// played.
//
// This is the Žolíky half of TestTheScoreboardAgreesWithTheWinner in
// internal/module: the shared conformance test drives a game from its offer
// list alone, and Žolíky is the one game that cannot be — going out needs a
// meld *shape* the offer protocol deliberately does not enumerate. So it is
// driven here by its own heuristic bot instead, and asked the same question.
//
// The question is worth asking because the answer used to be no. Standings
// ranked players by the cards they were holding at the instant the match
// ended, which is the leftovers of one deal out of seven and not the match
// score at all. A player who went out on the last deal took rank 1 and a gold
// badge no matter how the other six deals had gone, while the banner beside
// them named somebody else the winner. The runtime then recorded the rank-1
// row's score as that player's result for the entire match.
func TestTheScoreboardAgreesWithTheWinner(t *testing.T) {
	mod := zolikmod.New()
	players := []module.PlayerRef{
		{ID: "p1", Name: "p1", IsAI: true},
		{ID: "p2", Name: "p2", IsAI: true},
		{ID: "p3", Name: "p3", IsAI: true},
	}

	// Several seeds, because one deal's leftovers agreeing with a match total
	// by luck is exactly the coincidence this test has to survive.
	for _, seed := range []int64{1, 7, 42, 1979} {
		final, winners, ok := playToTheEnd(t, mod, players, seed)
		if !ok {
			continue
		}
		standings := module.StandingsFor(mod, final)
		if len(standings) != len(players) {
			t.Fatalf("seed %d: %d standings for %d players", seed, len(standings), len(players))
		}

		byID := map[string]module.Standing{}
		for _, s := range standings {
			byID[s.PlayerID] = s
		}
		for _, w := range winners {
			s, listed := byID[w]
			if !listed {
				t.Errorf("seed %d: the engine named %q the winner and the scoreboard omits them", seed, w)
				continue
			}
			if s.Rank != 1 || !s.Won {
				t.Errorf("seed %d: the engine named %q the winner; the scoreboard ranks them %d (won=%v)",
					seed, w, s.Rank, s.Won)
			}
		}

		// And the score on the row is the match total, not one deal's
		// leftovers. Checked against the engine's own totals rather than
		// recomputed, so this cannot drift into a second opinion.
		gs, err := zolikmod.RulesStateOf(final)
		if err != nil {
			t.Fatalf("seed %d: RulesStateOf: %v", seed, err)
		}
		for _, s := range standings {
			if want := -gs.TotalScores[s.PlayerID]; s.Score != want {
				t.Errorf("seed %d: %s scored %d on the scoreboard, %d over the match",
					seed, s.PlayerID, s.Score, want)
			}
			// Shown is what a player reads, and rummy is written upwards.
			if s.Shown == nil {
				t.Errorf("seed %d: %s has no shown score", seed, s.PlayerID)
			} else if *s.Shown != gs.TotalScores[s.PlayerID] {
				t.Errorf("seed %d: %s shows %d, total is %d",
					seed, s.PlayerID, *s.Shown, gs.TotalScores[s.PlayerID])
			}
		}
	}
}

// TestTheScoreboardOutlastsTheLastDeal is the same bug stated as its symptom:
// the player who goes out holds nothing, and holding nothing is not winning.
func TestTheScoreboardOutlastsTheLastDeal(t *testing.T) {
	mod := zolikmod.New()
	players := []module.PlayerRef{
		{ID: "p1", Name: "p1", IsAI: true},
		{ID: "p2", Name: "p2", IsAI: true},
	}
	final, winners, ok := playToTheEnd(t, mod, players, 3)
	if !ok || len(winners) == 0 {
		t.Skip("no finished match to inspect")
	}
	gs, err := zolikmod.RulesStateOf(final)
	if err != nil {
		t.Fatalf("RulesStateOf: %v", err)
	}

	// Find whoever emptied their hand on the final deal. If they are not the
	// match winner, the old implementation would have ranked them first anyway.
	for _, pid := range gs.TurnOrder {
		if len(gs.Hands[pid]) != 0 {
			continue
		}
		if winners[0] == pid {
			continue // they did win; this seed proves nothing either way
		}
		for _, s := range module.StandingsFor(mod, final) {
			if s.PlayerID == pid && s.Rank == 1 {
				t.Errorf("%s emptied their hand and is ranked first; the match was won by %s",
					pid, winners[0])
			}
		}
	}
}

// playToTheEnd runs a whole match with the module's own bot, the way the
// runtime's bot loop does.
func playToTheEnd(t *testing.T, mod module.GameModule, players []module.PlayerRef, seed int64) (module.State, []string, bool) {
	t.Helper()

	state, err := mod.NewMatch(module.MatchConfig{}, players, seed)
	if err != nil {
		t.Fatalf("NewMatch: %v", err)
	}
	bot := module.BotFor(mod)

	for step := 0; step < 20000; step++ {
		done, winners, err := mod.Finished(state)
		if err != nil {
			t.Fatalf("Finished: %v", err)
		}
		if done {
			return state, winners, true
		}
		actor := module.ActiveSeat(mod, state, players[0].ID, players)
		if actor == "" {
			break
		}
		offers, err := mod.LegalActions(state, actor)
		if err != nil {
			break
		}
		action, ok := bot.Act(state, actor, offers)
		if !ok {
			if action, ok = module.ChooseAction(offers, nil); !ok {
				break
			}
		}
		next, _, err := mod.Apply(state, actor, action)
		if err != nil {
			// A heuristic agent may propose a meld the validator dislikes;
			// recover the way the runtime's loop does rather than giving up.
			fallback, ok := module.ChooseAction(offers, nil)
			if !ok {
				break
			}
			if next, _, err = mod.Apply(state, actor, fallback); err != nil {
				break
			}
		}
		state = next
	}
	t.Logf("seed %d: the bots did not finish a match; skipping this seed", seed)
	return state, nil, false
}
