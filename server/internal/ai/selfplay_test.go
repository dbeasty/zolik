package ai

import (
	"fmt"
	"testing"

	"zolik/server/internal/rules"
)

// selfPlayResult summarises one simulated all-AI deal.
type selfPlayResult struct {
	melds      int
	layOffs    int
	turns      int
	deals      int // deals that actually finished (someone went out)
	downCount  int
	ended      bool
	stalled    bool
	lastErr    error
	rejections int
}

// visibleFor mirrors game.aiVisibleFromGame: it is the exact snapshot the
// server hands the agent, rebuilt from a pure rules.GameState so the AI can
// be driven through a whole deal without a database.
func visibleFor(st rules.GameState, playerDiscards map[string][]string) VisibleState {
	return VisibleState{
		GameNumber:     st.GameNumber,
		Round:          st.Round,
		Phase:          string(st.Phase),
		CurrentTurn:    st.CurrentTurn,
		DiscardPile:    st.DiscardPile,
		PlayerDiscards: playerDiscards,
		Melds:          st.Melds,
		MeldMeta:       st.MeldMeta,
		RoundReqMet:    st.RoundReqMet,
		TotalScores:    st.TotalScores,
		Rules:          st.Rules,

		DiscardTakenCard: st.DiscardTakenCard,
	}
}

// playDeal runs an all-AI deal to completion (or until maxActions) using the
// real engine, and reports what the agents actually did.
func playDeal(t *testing.T, cfg rules.RulesConfig, seed int64, players []string, difficulty string, maxActions int) selfPlayResult {
	t.Helper()
	st, err := rules.StartMatch(cfg, players, seed)
	if err != nil {
		t.Fatalf("StartMatch: %v", err)
	}
	agent := NewHeuristicAgent(difficulty)
	playerDiscards := map[string][]string{}
	res := selfPlayResult{}
	prevTurn := ""

	for i := 0; i < maxActions; i++ {
		if st.Status != rules.StatusActive {
			res.ended = true
			break
		}
		actor := st.CurrentTurn
		if actor != prevTurn {
			res.turns++
			prevTurn = actor
		}
		action := agent.ChooseAction(visibleFor(st, playerDiscards), append([]string(nil), st.Hands[actor]...))
		outcome, err := rules.ApplyAction(st, actor, action)
		if err != nil {
			res.rejections++
			res.lastErr = fmt.Errorf("%s %+v (hand %v): %w", actor, action, st.Hands[actor], err)
			// Mirror the server's fallback so one rejection doesn't wedge the sim.
			fallback := rules.Action{
				Type: rules.ActionDiscard,
				Card: PickWorstDiscard(st.Hands[actor], cfg, len(st.Hands[actor]) == 1 && st.RoundReqMet[actor], st.DiscardTakenCard),
			}
			outcome, err = rules.ApplyAction(st, actor, fallback)
			if err != nil {
				res.stalled = true
				break
			}
			action = fallback
		}
		switch action.Type {
		case rules.ActionLayMeld:
			res.melds++
		case rules.ActionLayOff:
			res.layOffs++
		case rules.ActionDiscard:
			playerDiscards[actor] = append(playerDiscards[actor], action.Card)
		}
		for _, e := range outcome.Events {
			if e.Type == "deal_ended" {
				res.deals++
			}
		}
		st = outcome.State
	}
	for _, p := range players {
		if st.RoundReqMet[p] {
			res.downCount++
		}
	}
	return res
}

// TestSelfPlay_AIMakesProgress is the end-to-end guard behind the "the AI
// never melds" report. Every single-decision unit test in this package
// passed while the agents were, in a real deal, incapable of finishing one:
// they would go down, then trade one useless card around the table forever.
// Only playing whole matches under the shipped profiles catches that class
// of bug, so this asserts the two properties that matter — melds reach the
// table, and deals actually end.
func TestSelfPlay_AIMakesProgress(t *testing.T) {
	profiles := []rules.RulesConfig{rules.ProfileZolikClassic, rules.ProfileContinental}
	for _, cfg := range profiles {
		cfg := cfg
		t.Run(cfg.Profile, func(t *testing.T) {
			// Whole matches against the real engine are not cheap; -short
			// keeps a representative slice for the fast path.
			runs := 20
			if testing.Short() {
				runs = 5
			}
			totalMelds, runsWithAMeld, totalDeals, runsThatDealt := 0, 0, 0, 0
			for seed := int64(1); seed <= int64(runs); seed++ {
				r := playDeal(t, cfg, seed, []string{"ai1", "ai2", "ai3"}, "medium", 4000)
				totalMelds += r.melds
				totalDeals += r.deals
				if r.melds > 0 {
					runsWithAMeld++
				}
				if r.deals > 0 {
					runsThatDealt++
				}
				if r.stalled {
					t.Errorf("seed %d: the agents wedged — no legal action left: %v", seed, r.lastErr)
				}
				t.Logf("seed %d: melds=%d layoffs=%d deals=%d turns=%d rejections=%d lastErr=%v",
					seed, r.melds, r.layOffs, r.deals, r.turns, r.rejections, r.lastErr)
			}
			t.Logf("%s: %d/%d runs melded (%d melds), %d/%d runs finished a deal (%d deals)",
				cfg.Profile, runsWithAMeld, runs, totalMelds, runsThatDealt, runs, totalDeals)

			if runsWithAMeld < runs {
				t.Errorf("%s: %d of %d runs never put a single meld on the table",
					cfg.Profile, runs-runsWithAMeld, runs)
			}
			// The livelock guard. Before the discard-pickup fix this was
			// zero across every seed of both profiles: the agents drew and
			// discarded the same card for 4000 actions without a deal ever
			// finishing. A handful of unfinished runs is fine (the budget is
			// finite and some deals are genuinely slow) — none finishing is
			// the bug.
			if runsThatDealt*2 < runs {
				t.Errorf("%s: only %d of %d runs finished a deal — the agents are not converging on going out",
					cfg.Profile, runsThatDealt, runs)
			}
		})
	}
}
