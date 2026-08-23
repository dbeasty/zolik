package ai

import (
	"testing"

	"zolik/server/internal/rules"
)

// zolikWithFloor is Žolík Classic as a host may actually configure it: no
// per-type contract quota, but a point floor on the initial meld. Neither
// shipped profile is this shape — Continental has the floor but a quota,
// stock Žolík Classic has the quota-free shortcut but no floor — which is
// why nothing caught the two bugs below until a real table ran it.
func zolikWithFloor(min int) rules.RulesConfig {
	cfg := rules.ProfileZolikClassic
	cfg.InitialMeldMinimum = min
	cfg.DiscardDrawMinRound = 3
	return cfg
}

func meldPhaseState(cfg rules.RulesConfig, actor string) VisibleState {
	return VisibleState{
		GameNumber:  1,
		Round:       9,
		Phase:       string(rules.PhaseMeld),
		CurrentTurn: actor,
		Melds:       map[string][][]string{},
		MeldMeta:    map[string][]rules.MeldInfo{},
		RoundReqMet: map[string]bool{actor: false},
		TotalScores: map[string]int{},
		Rules:       cfg,
	}
}

func naturalValueOf(cards []string) int {
	sum := 0
	for _, c := range cards {
		sum += rules.NaturalCardValue(c, true)
	}
	return sum
}

// The reported case, from game 6a8aa17ff767a3c62209d475: the agent laid
// A-2-3 — six natural points against a 35-point floor. It is a legal action
// (a quota-free profile accepts any complete meld), but it cannot bring the
// player down, so it only spends cards and gives every opponent a lay-off
// target. The quota-free shortcut has to respect the floor.
func TestHeuristicAgent_WontLayBelowTheInitialMeldFloor(t *testing.T) {
	const actor = "ai:medium:1:0"
	cfg := zolikWithFloor(35)
	// The exact hand Clever Karel held, plus the run it laid.
	hand := []string{"AC", "2C", "3C", "8D", "JH", "JH", "2H", "5S", "8D", "6H", "3S", "5D", "6C"}

	act := NewHeuristicAgent("medium").ChooseAction(meldPhaseState(cfg, actor), hand)

	if act.Type == rules.ActionLayMeld {
		t.Fatalf("laid %v, worth %d natural points, against a %d-point floor: it cannot come down with that",
			act.Cards, naturalValueOf(act.Cards), cfg.InitialMeldMinimum)
	}
}

// The other half: refusing to lay junk is only right if the agent can still
// recognise a hand that does clear the floor. The floor is summed across
// every meld the player lays (see the RoundReqMet assignment in
// rules.ValidateMeldAction), so a plan may reach it with several melds that
// fall short individually — 29 + 18 clears 35 where neither does alone.
// The plan search used to stop the moment the contract's shape was met and
// so could only ever clear a floor with one single big meld.
func TestFindInitialMeldPlan_ReachesTheFloorAcrossSeveralMelds(t *testing.T) {
	const actor = "ai:medium:1:0"
	cfg := zolikWithFloor(35)
	st := rulesStateForAI(meldPhaseState(cfg, actor), actor)

	runA := []string{"9H", "TH", "JH"} // 29
	runB := []string{"5C", "6C", "7C"} // 18
	if naturalValueOf(runA) >= cfg.InitialMeldMinimum || naturalValueOf(runB) >= cfg.InitialMeldMinimum {
		t.Fatalf("test setup is not exercising the multi-meld path: %d and %d against a %d floor",
			naturalValueOf(runA), naturalValueOf(runB), cfg.InitialMeldMinimum)
	}
	hand := append(append(append([]string{}, runA...), runB...), "2D")

	combo, ok := findInitialMeldPlan(st, actor, hand)
	if !ok {
		t.Fatalf("no plan found for %v, but %v + %v clears the %d-point floor",
			hand, runA, runB, cfg.InitialMeldMinimum)
	}
	total := 0
	for _, m := range combo {
		total += naturalValueOf(m)
	}
	if total < cfg.InitialMeldMinimum {
		t.Errorf("plan %v is worth %d, short of the %d-point floor", combo, total, cfg.InitialMeldMinimum)
	}
	if len(combo) < 2 {
		t.Errorf("plan %v uses one meld; the floor here is only reachable across two", combo)
	}
}

// A quota profile (Continental) rejects any meld past its contract with
// MELD_NO_CONTRIBUTION, so topping the value up with an extra meld is not
// available there and planning one would just get the action bounced.
func TestFindInitialMeldPlan_DoesNotTopUpUnderAQuotaProfile(t *testing.T) {
	const actor = "ai:medium:1:0"
	cfg := rules.ProfileContinental // deal 3 wants two runs, 4+ cards each
	visible := meldPhaseState(cfg, actor)
	visible.GameNumber = 3
	st := rulesStateForAI(visible, actor)

	// Two low runs that satisfy the shape but not the 35-point floor, and a
	// third high run that would top it up if extra melds were allowed.
	hand := []string{"2H", "3H", "4H", "5H", "2S", "3S", "4S", "5S", "TD", "JD", "QD", "KD", "8C"}
	combo, ok := findInitialMeldPlan(st, actor, hand)
	if !ok {
		t.Skip("no plan for this hand; the assertion below has nothing to check")
	}
	if len(combo) > cfg.ContractFor(3).Runs {
		t.Errorf("plan %v lays %d melds under a contract wanting %d runs — the extras would be rejected",
			combo, len(combo), cfg.ContractFor(3).Runs)
	}
}
