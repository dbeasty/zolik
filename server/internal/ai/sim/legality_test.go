package sim

import (
	"testing"

	"zolik/server/internal/module"
)

// TestNoPairingEverProposesAnIllegalMove.
//
// TestSkillLadderIsMonotonic already calls assertClean on everything it plays,
// and it did not catch the bug this file was added for: a two-handed deal
// between Easy and Medium, on a ruleset with a point floor, where the agent
// ended up holding two jokers and proposed discarding one. Under
// JokerDiscardRestricted a joker may be discarded only as the card that empties
// an already-down player's hand, so the engine refused it, the fallback discard
// had nothing legal to name either, and the deal wedged for both seats.
//
// The difference is the table. The ladder sweep seats every strength at once,
// which is a different distribution of positions from two seats head to head —
// and head to head is what most tables actually are. So this walks the pairings
// directly.
//
// A rejection is not a weakness, it is a bug, at any strength: a legal player
// proposes legal moves. The same goes for a stall, which is a turn the agent
// could not end at all.
//
// Sixty seeds rather than the couple of hundred cmd/aibench sweeps with,
// because this is nine pairings across every ruleset and the whole package is
// already the slow one. aibench is where a wider sweep belongs; internal/ai's
// own TestLayOffSpendsTheJokerInAWildCrunch is the fast guard on the specific
// position.
func TestNoPairingEverProposesAnIllegalMove(t *testing.T) {
	if testing.Short() {
		t.Skip("a seed sweep is not a fast test")
	}
	for _, rs := range rulesets() {
		rs := rs
		t.Run(rs.name, func(t *testing.T) {
			t.Parallel()
			for i, a := range module.Skills {
				for _, b := range module.Skills[i:] {
					for seed := int64(1); seed <= 60; seed++ {
						r := Play(Options{
							Rules: rs.cfg,
							Seed:  seed,
							Seats: []Seat{
								{ID: "a", Skill: a},
								{ID: "b", Skill: b},
							},
							MaxActions: 4000,
						})
						if r.Rejections > 0 || r.Stalled {
							t.Fatalf("%s vs %s, seed %d: %d refused, stalled=%v: %v",
								a, b, seed, r.Rejections, r.Stalled, r.LastErr)
						}
					}
				}
			}
		})
	}
}
