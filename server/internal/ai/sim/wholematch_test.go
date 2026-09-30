package sim

import (
	"fmt"
	"testing"

	"zolik/server/internal/module"
)

// Budgets for TestWholeMatchesFinish. A deal that has not ended after
// wholeMatchDealBudget actions has stopped converging; a match past
// wholeMatchBudget has done so somewhere without any one deal looking stuck.
//
// Both sit well clear of the slowest seed the sweep plays today (the numbers
// are logged on every run), so they are a stall detector rather than a speed
// target: over the full sweep the longest deal takes a fraction of the
// per-deal budget and the longest match a fraction of the match budget.
const (
	wholeMatchDealBudget = 1500
	wholeMatchBudget     = 12000
)

// TestWholeMatchesFinish plays every deal of every match, where the other
// self-play tests only need deal one to finish.
//
// That difference is the whole reason this test exists. Continental rotates
// its contract per deal, and deal seven's three runs is a contract nothing
// else in the suite ever reached: every hard bot at the table converged on a
// hand of 2s, 3s, 4s and jokers — protecting low sets a runs-only contract has
// no use for, shedding the high cards the 35-point floor needed — and the deal
// never ended. The gamebench harness on claude/learn-core found it stalling 55
// seeds in 60; TestSelfPlay_AIMakesProgress was green throughout, because a
// match that finishes deal one and then wedges still "finished a deal".
//
// Deal one had its own wedges once the whole match was played: a floor a hand
// of low sets could never reach, a down seat holding a pair it could never lay,
// and tables where every seat was down and every set full, with nobody left
// holding enough cards to open a new one.
//
// So: whole matches, hard against hard and hard against easy, two to four
// seats, seeds 1 to 100 of every ruleset the other sweeps use — and not one of
// them may stall, wedge, or be refused a move.
func TestWholeMatchesFinish(t *testing.T) {
	seeds := int64(100)
	if testing.Short() {
		seeds = 10
	}
	pairings := []struct {
		name string
		b    module.Skill
	}{
		{"hard-vs-hard", module.SkillHard},
		{"hard-vs-easy", module.SkillEasy},
	}
	for _, p := range rulesets() {
		for _, pair := range pairings {
			for seats := 2; seats <= 4; seats++ {
				p, pair, seats := p, pair, seats
				t.Run(fmt.Sprintf("%s/%s/%d-seats", p.name, pair.name, seats), func(t *testing.T) {
					t.Parallel()
					longestDeal, longestMatch := 0, 0
					for seed := int64(1); seed <= seeds; seed++ {
						// Hard holds the first seat and every other one after
						// it, so a table of three is hard, easy, hard.
						table := make([]Seat, seats)
						for i := range table {
							skill := module.SkillHard
							if i%2 == 1 {
								skill = pair.b
							}
							table[i] = Seat{ID: fmt.Sprintf("p%d", i+1), Skill: skill}
						}
						r := Play(Options{
							Rules: p.cfg, Seed: seed, Seats: table,
							WholeMatch: true, MaxActions: wholeMatchBudget, DealBudget: wholeMatchDealBudget,
						})
						switch {
						case r.StalledDeal > 0:
							t.Errorf("seed %d: deal %d ran %d actions without anybody going out:\n%s",
								seed, r.StalledDeal, wholeMatchDealBudget, r.Wedge)
						case r.Stalled:
							t.Errorf("seed %d: the table wedged — no legal action left: %v", seed, r.LastErr)
						case !r.MatchOver:
							t.Errorf("seed %d: the match was still going after %d actions (%d deals finished)",
								seed, wholeMatchBudget, r.Deals)
						}
						if r.Rejections > 0 {
							t.Errorf("seed %d: %d action(s) refused by the engine; last: %v", seed, r.Rejections, r.LastErr)
						}
						if r.StrandedLays > 0 {
							t.Errorf("seed %d: %d turn(s) ended with melds laid and the contract unmet: %v",
								seed, r.StrandedLays, r.FirstStranded)
						}
						total := 0
						for _, n := range r.DealActions {
							total += n
							longestDeal = max(longestDeal, n)
						}
						longestMatch = max(longestMatch, total)
					}
					t.Logf("longest deal %d actions, longest match %d", longestDeal, longestMatch)
				})
			}
		}
	}
}
