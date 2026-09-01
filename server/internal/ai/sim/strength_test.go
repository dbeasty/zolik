package sim

import (
	"testing"

	"zolik/server/internal/ai"

	"zolik/server/internal/module"
	"zolik/server/internal/rules"
)

// TestSkillLadderIsMonotonic is the test the whole feature rests on.
//
// Every other test here asserts that a bot plays *legally*. This one asserts
// that the strengths are actually ordered — that Hard beats Medium and Expert
// beats Hard over a fixed seed sweep — which is the only evidence that any of
// the perception and planning work did anything. Without it, "we made the AI
// smarter" is a claim about a diff rather than about a bot, and a later change
// could quietly invert the ladder with every unit test still green.
//
// Adjacent pairs only. Comparing Expert against Easy would be an easier test
// and a weaker one: the ladder is broken by neighbours swapping, and a gap of
// three levels would hide that.
//
// The margin is deliberately loose. Rummy is a high-variance game — the deal
// decides a lot of deals — so the assertion is "the stronger profile does not
// come out behind", not a required winning margin. A sweep that is merely
// level is not evidence of a broken ladder; a sweep that is backwards is.
func TestSkillLadderIsMonotonic(t *testing.T) {
	if testing.Short() {
		t.Skip("a strength sweep is not a fast test")
	}
	const seeds = 200
	for _, p := range rulesets() {
		p, cfg := p, p.cfg
		t.Run(p.name, func(t *testing.T) {
			t.Parallel()
			cs := make([]Contender, 0, len(module.Skills))
			for _, s := range module.Skills {
				cs = append(cs, Contender{Skill: s})
			}
			ts := Table(cfg, cs, seeds, 4000)
			for _, tl := range ts {
				t.Logf("%-14s %-7s %3d wins of %3d (%.0f%%)  mean %.1f pts",
					p.name, tl.Skill, tl.Wins, tl.Deals, 100*tl.WinRate(), tl.MeanPoints())
				assertClean(t, tl)
			}
			if ts[0].Deals == 0 {
				t.Fatalf("no deal finished in %d runs — the sweep measured nothing", seeds)
			}
			// Two assertions, and they are deliberately different in
			// strictness.
			//
			// Wins are the headline number and the noisiest: rummy is a
			// high-variance game, so adjacent levels are only required not to
			// come out *behind* by more than the sweep's own noise. Mean
			// penalty points are far steadier — every deal contributes, not
			// just the ones somebody won — so that is where the ladder is
			// required to be strictly ordered.
			if !p.ladderIsPoliced {
				t.Logf("%s: ordering reported, not asserted — see ruleset.ladderIsPoliced", p.name)
				return
			}
			for i := 0; i+1 < len(ts); i++ {
				weaker, stronger := ts[i], ts[i+1]
				if stronger.MeanPoints() > weaker.MeanPoints() {
					t.Errorf("%s finishes with more penalty points than %s (%.1f vs %.1f): the ladder is upside down",
						stronger.Skill, weaker.Skill, stronger.MeanPoints(), weaker.MeanPoints())
				}
				if stronger.Wins < weaker.Wins-winNoise {
					t.Errorf("%s won %d to %s's %d: further behind than the sweep's noise explains",
						stronger.Skill, stronger.Wins, weaker.Skill, weaker.Wins)
				}
			}
			// End to end, the ladder has to be unambiguous even if a
			// neighbouring pair is close.
			weakest, strongest := ts[0], ts[len(ts)-1]
			if strongest.Wins <= weakest.Wins {
				t.Errorf("%s (%d wins) did not beat %s (%d): the ladder is flat",
					strongest.Skill, strongest.Wins, weakest.Skill, weakest.Wins)
			}
		})
	}
}

// winNoise is how far behind an adjacent level may come out on wins alone
// before it counts as evidence rather than variance.
//
// A three-seat sweep of 200 deals gives each level somewhere near sixty wins,
// with a standard deviation around seven; two of those is the allowance. The
// mean-points assertion above is the one that actually polices the ordering,
// and it has no allowance at all.
const winNoise = 14

// assertClean pins the properties that hold at *every* strength. A weak bot
// plays badly; it never plays illegally, and it never stops the table.
func assertClean(t *testing.T, tl Tally) {
	t.Helper()
	if tl.Rejections > 0 {
		t.Errorf("%s: %d action(s) refused by the engine", tl.Skill, tl.Rejections)
	}
	if tl.Stalls > 0 {
		t.Errorf("%s: %d run(s) wedged", tl.Skill, tl.Stalls)
	}
	if tl.StrandedLays > 0 {
		t.Errorf("%s: %d turn(s) ended with melds down and the contract unmet", tl.Skill, tl.StrandedLays)
	}
}

// TestMediumIsUnchanged pins the reference point.
//
// Medium is defined as the agent exactly as it played before strengths
// existed, and everything else is measured against it. If a later change
// improves Medium, the improvement is welcome and the ladder still has to be
// re-measured — so this is a canary rather than a prohibition: it fails if
// Medium's own profile grows a capability, which is the change that would
// silently move every other level's baseline.
func TestMediumIsUnchanged(t *testing.T) {
	p := ProfileForSkill(module.SkillMedium)
	if p.Recall != 0 || p.ReadHandCounts || p.ReadPickups || p.KeepPartials != ai.KeepFinished {
		t.Errorf("medium has grown a capability it did not have: %+v", p)
	}
	if !p.ReadTableDanger {
		t.Error("medium must still avoid feeding live melds — that was its whole distinguishing signal")
	}
}

// TestUnknownSkillPlaysAtMedium is the compatibility rule.
//
// Every bot seated before any of this existed carries an empty aiDifficulty,
// and every one of them has been playing the agent now called Medium.
// Defaulting an unknown strength *down* would silently weaken every table
// already in the database.
func TestUnknownSkillPlaysAtMedium(t *testing.T) {
	for _, s := range []module.Skill{"", "unspecified", "nonsense"} {
		if got := ProfileForSkill(s).Skill; got != module.SkillMedium {
			t.Errorf("skill %q resolved to %q, want medium", s, got)
		}
	}
}

var _ = rules.ProfileZolikClassic
