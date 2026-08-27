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
	// strandedLays counts turns that ended with cards laid but the player
	// still short of the table's initial-meld point floor — the shape of
	// the "Karel laid A-2-3 against a 35-point floor" report.
	strandedLays  int
	firstStranded error
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
	st, err := rules.StartMatch(cfg, players, seed, "")
	if err != nil {
		t.Fatalf("StartMatch: %v", err)
	}
	agent := NewHeuristicAgent(difficulty)
	playerDiscards := map[string][]string{}
	laidWhileShort := map[string]bool{}
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
		// A deal that ends inside this outcome has already re-dealt: the
		// table is cleared and everyone's down status reset, so nothing in
		// outcome.State describes the deal the action was taken in.
		dealEnded := false
		for _, e := range outcome.Events {
			if e.Type == "deal_ended" {
				dealEnded = true
			}
		}
		switch action.Type {
		case rules.ActionLayMeld:
			res.melds++
			// Cards went down while the player was still short of the
			// table's point floor. Legitimate mid-plan (the floor is summed
			// across melds, so 27 + 21 clears 35), but only if the rest of
			// the plan lands this turn — see the discard case below.
			if !st.RoundReqMet[actor] && cfg.InitialMeldMinimum > 0 {
				laidWhileShort[actor] = true
			}
		case rules.ActionLayOff:
			res.layOffs++
		case rules.ActionDiscard:
			playerDiscards[actor] = append(playerDiscards[actor], action.Card)
			// The discard ends the turn. Anything laid this turn that did
			// not bring the player down is stranded on the table: it cannot
			// make them down, it spent cards a qualifying combination might
			// have wanted, and it is a lay-off target for every opponent.
			if laidWhileShort[actor] && !dealEnded && !outcome.State.RoundReqMet[actor] {
				res.strandedLays++
				if res.firstStranded == nil {
					res.firstStranded = fmt.Errorf("%s finished a turn having laid melds %v while still short of the %d-point floor",
						actor, outcome.State.Melds[actor], cfg.InitialMeldMinimum)
				}
			}
			delete(laidWhileShort, actor)
		}
		if dealEnded {
			res.deals++
			// A new deal clears the table and everyone's down status, so
			// mid-plan state from the deal just finished says nothing about
			// the next one.
			laidWhileShort = map[string]bool{}
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
	// A shipped profile is not the only thing a table can run: the lobby
	// lets the host layer house rules on top, and the combinations are what
	// reach real players. Žolík Classic under a 35-point floor is the
	// configuration of game 6a8aa17ff767a3c62209d475, where the agent laid
	// A-2-3 (6 points) it could never come down with — a bug both shipped
	// profiles hid, since one has no floor and the other has no free melds.
	zolikFloored := rules.ProfileZolikClassic
	zolikFloored.InitialMeldMinimum = 35
	zolikFloored.DiscardDrawMinRound = 3

	profiles := []struct {
		name string
		cfg  rules.RulesConfig
	}{
		{"zolik_classic", rules.ProfileZolikClassic},
		{"zolik_classic+floor35", zolikFloored},
		{"continental", rules.ProfileContinental},
	}
	for _, p := range profiles {
		cfg := p.cfg
		t.Run(p.name, func(t *testing.T) {
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
				if r.strandedLays > 0 {
					t.Errorf("seed %d: %d turn(s) ended with cards laid but the player still short of the floor: %v",
						seed, r.strandedLays, r.firstStranded)
				}
				t.Logf("seed %d: melds=%d layoffs=%d deals=%d turns=%d rejections=%d lastErr=%v",
					seed, r.melds, r.layOffs, r.deals, r.turns, r.rejections, r.lastErr)
			}
			t.Logf("%s: %d/%d runs melded (%d melds), %d/%d runs finished a deal (%d deals)",
				p.name, runsWithAMeld, runs, totalMelds, runsThatDealt, runs, totalDeals)

			if runsWithAMeld < runs {
				t.Errorf("%s: %d of %d runs never put a single meld on the table",
					p.name, runs-runsWithAMeld, runs)
			}
			// The livelock guard. Before the discard-pickup fix this was
			// zero across every seed of both profiles: the agents drew and
			// discarded the same card for 4000 actions without a deal ever
			// finishing. A handful of unfinished runs is fine (the budget is
			// finite and some deals are genuinely slow) — none finishing is
			// the bug.
			if runsThatDealt*2 < runs {
				t.Errorf("%s: only %d of %d runs finished a deal — the agents are not converging on going out",
					p.name, runsThatDealt, runs)
			}
		})
	}
}
