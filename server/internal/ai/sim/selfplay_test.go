package sim

import (
	"testing"

	"zolik/server/internal/module"
	"zolik/server/internal/rules"
)

// rulesets is every configuration these tests sweep.
//
// A shipped profile is not the only thing a table can run: the lobby lets the
// host layer house rules on top, and the combinations are what reach real
// players. Žolík Classic under a 35-point floor is the configuration of game
// 6a8aa17ff767a3c62209d475, where the agent laid A-2-3 (6 points) it could
// never come down with — a bug both shipped profiles hid, since one has no
// floor and the other has no free melds.
func rulesets() []ruleset {
	floored := rules.ProfileZolikClassic
	floored.InitialMeldMinimum = 35
	floored.DiscardDrawMinRound = 3
	return []ruleset{
		{"zolik_classic", rules.ProfileZolikClassic, true},
		{"zolik_classic+floor35", floored, true},
		// Continental is measured but not policed — see the note on
		// ladderIsPoliced.
		{"continental", rules.ProfileContinental, false},
	}
}

type ruleset struct {
	name string
	cfg  rules.RulesConfig
	// ladderIsPoliced says whether the strength gate asserts an ordering on
	// this ruleset, as opposed to merely reporting one.
	//
	// It is false for exactly one ruleset, and the reason is a finding rather
	// than a convenience. Continental is not Žolíky with different numbers:
	// its contract rotates per deal and asks for a quota of meld *types*, so
	// the thing Hard is good at — protecting unfinished material and pricing
	// it by what is still live — competes with a contract that wants specific
	// shapes rather than any shapes. Across repeated 150–200 deal sweeps Hard
	// beat Medium and Easy comfortably on penalty points (227 against 255 and
	// 245 in one) and not at all on wins (45 against 37 and 50), and the two
	// runs disagreed with each other by more than either gap. That is a table
	// too noisy to assert an ordering on and too interesting to drop, so it is
	// run, logged, and held to the legality invariants — which are absolute
	// everywhere — while the ordering is asserted where the signal is
	// unambiguous.
	//
	// The honest summary is that these strengths are tuned for Žolíky and
	// currently mean less at a Continental table. Fixing that means making
	// keepValue contract-aware, which is a piece of work with its own
	// measurement, not a constant to nudge.
	ladderIsPoliced bool
}

// TestSelfPlay_AIMakesProgress is the end-to-end guard behind the "the AI
// never melds" report. Every single-decision unit test in internal/ai passed
// while the agents were, in a real deal, incapable of finishing one: they
// would go down, then trade one useless card around the table forever. Only
// playing whole matches under the shipped profiles catches that class of bug,
// so this asserts the two properties that matter — melds reach the table, and
// deals actually end.
//
// It now runs at every strength, because the failure it guards against is
// exactly the one a new strength is most likely to reintroduce: a bot that
// keeps more of its hand, or that misses a lay-off on purpose, is a bot that
// can stop converging on going out.
func TestSelfPlay_AIMakesProgress(t *testing.T) {
	for _, p := range rulesets() {
		for _, skill := range module.Skills {
			cfg, skill := p.cfg, skill
			t.Run(p.name+"/"+string(skill), func(t *testing.T) {
				// Whole matches against the real engine are not cheap;
				// -short keeps a representative slice for the fast path.
				runs := 20
				if testing.Short() {
					runs = 5
				}
				totalMelds, runsWithAMeld, totalDeals, runsThatDealt := 0, 0, 0, 0
				for seed := int64(1); seed <= int64(runs); seed++ {
					r := Play(Options{
						Rules: cfg, Seed: seed, MaxActions: 4000,
						Seats: []Seat{
							{ID: "ai1", Skill: skill},
							{ID: "ai2", Skill: skill},
							{ID: "ai3", Skill: skill},
						},
					})
					totalMelds += r.Melds
					totalDeals += r.Deals
					if r.Melds > 0 {
						runsWithAMeld++
					}
					if r.Deals > 0 {
						runsThatDealt++
					}
					if r.Stalled {
						t.Errorf("seed %d: the agents wedged — no legal action left: %v", seed, r.LastErr)
					}
					if r.StrandedLays > 0 {
						t.Errorf("seed %d: %d turn(s) ended with cards laid but the player still short of the floor: %v",
							seed, r.StrandedLays, r.FirstStranded)
					}
				}
				t.Logf("%s/%s: %d/%d runs melded (%d melds), %d/%d runs finished a deal (%d deals)",
					p.name, skill, runsWithAMeld, runs, totalMelds, runsThatDealt, runs, totalDeals)

				if runsWithAMeld < runs {
					t.Errorf("%s/%s: %d of %d runs never put a single meld on the table",
						p.name, skill, runs-runsWithAMeld, runs)
				}
				// The livelock guard. Before the discard-pickup fix this was
				// zero across every seed of both profiles: the agents drew and
				// discarded the same card for 4000 actions without a deal ever
				// finishing. A handful of unfinished runs is fine (the budget
				// is finite and some deals are genuinely slow) — none
				// finishing is the bug.
				if runsThatDealt*2 < runs {
					t.Errorf("%s/%s: only %d of %d runs finished a deal — the agents are not converging on going out",
						p.name, skill, runsThatDealt, runs)
				}
			})
		}
	}
}

// TestEveryStrengthPlaysLegally is the floor under the whole skill ladder.
//
// A weak bot is one that plays badly. It is never one that proposes moves the
// engine refuses, and never one that strands its own turn and stops the table
// for everybody else — the deliberate mistakes are drawn from the same
// filtered, legal candidate list the best move came from, and this is what
// proves it. Rejections and stalls must be zero at *every* strength, which is
// a stricter thing to assert than that the strong ones are strong.
func TestEveryStrengthPlaysLegally(t *testing.T) {
	runs := 12
	if testing.Short() {
		runs = 4
	}
	for _, p := range rulesets() {
		for _, skill := range module.Skills {
			cfg, skill := p.cfg, skill
			t.Run(p.name+"/"+string(skill), func(t *testing.T) {
				for seed := int64(1); seed <= int64(runs); seed++ {
					r := Play(Options{
						Rules: cfg, Seed: seed, MaxActions: 3000,
						Seats: []Seat{
							{ID: "a", Skill: skill},
							{ID: "b", Skill: skill},
							{ID: "c", Skill: skill},
						},
					})
					if r.Rejections > 0 {
						t.Errorf("seed %d: %d action(s) refused by the engine; last: %v",
							seed, r.Rejections, r.LastErr)
					}
					if r.Stalled {
						t.Errorf("seed %d: the table wedged: %v", seed, r.LastErr)
					}
				}
			})
		}
	}
}
