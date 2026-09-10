package canasta

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"testing"

	"zolik/server/internal/module"
)

// The golden fixture that makes the Samba refactor safe (docs/samba-plan.md §6).
//
// Widening the meld model, the deck and the seating touches code every variation
// runs through, and the failure mode that matters is not a compile error — it is
// `classic` quietly dealing one card differently and nobody noticing until a real
// table complains about a score. So the *whole* final state of a fixed corpus of
// matches is hashed here, and the hashes were recorded before any of that work
// began.
//
// A hash rather than a golden JSON file because the point is "nothing moved", not
// "here is what a deal looks like": a diff of 60 kB of cards would be read by
// nobody, whereas a changed hash is unambiguous and names the seed that moved.
//
// What is hashed is the *play* — every deal's six-part settlement, the running
// scores, the winner and the exact tally of moves it took — and deliberately not
// the encoded state. Adding a field to `GameState` changes those bytes without
// changing a single card, so a blob hash would cry wolf at every refactor and be
// re-recorded until it meant nothing. This projection only moves if the game
// does.
//
// A failure names both hashes, so re-recording a deliberate change is a copy of
// the one it reports — and the commit that does it has to say why a variation
// nobody meant to touch now plays differently.
func TestExistingVariationsAreUnchanged(t *testing.T) {
	cases := []struct {
		name    string
		players []module.PlayerRef
		cfg     module.MatchConfig
		want    []string // by seed, seeds 1..len
	}{
		{
			name:    "classic/2",
			players: refs("p1", "p2"),
			cfg:     goldenCfg("classic"),
			want: []string{
				"88cd7030ce2cdaec", "59ccd76f9f041ae4", "546a0f5146420454",
				"e3468083f87ec9b1", "7185d6042231ab59", "784352bb9f4b41f7",
			},
		},
		{
			name:    "classic/3",
			players: refs("p1", "p2", "p3"),
			cfg:     goldenCfg("classic"),
			want: []string{
				"284f63d874e1d71e", "1247c543cf17aaab", "89ef2adfec9d6609",
				"c28153ad509fef3c", "080f6737d755b0a6", "038b3d7157a34855",
			},
		},
		{
			name:    "classic/4",
			players: refs("p1", "p2", "p3", "p4"),
			cfg:     goldenCfg("classic"),
			want: []string{
				"cb125876593839bb", "9d58d1f2b741bcfb", "6e219a382095cda1",
				"487e77b09e075fe5", "d24b4d92e41b8a15", "22a3ba98486407aa",
			},
		},
		{
			name:    "modern_american/4",
			players: refs("p1", "p2", "p3", "p4"),
			cfg:     goldenCfg("modern_american"),
			// Seeds 4 and 6 were re-recorded when this branch merged main: the
			// "offer every meldable rank, not just the one a shared wild
			// favoured" fix deliberately widens the meld offers, so the driver
			// picks differently and the deals settle differently. Verified as
			// main's rather than this branch's by running these same cases on
			// plain origin/main, where both produce exactly these hashes.
			want: []string{
				"2863826a2309f055", "2657c0a78520664d", "6f07d3bfea709bff",
				"a1ce2c481c73bcf6", "89e091cc3974fda8", "b2d3c3f27f46f2b9",
			},
		},
	}

	m := New()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for i, want := range tc.want {
				seed := int64(i + 1)
				got := finalStateHash(t, m, tc.cfg, tc.players, seed)
				if got != want {
					t.Errorf("seed %d: state hash %s, recorded %s — a variation nobody meant to touch plays differently",
						seed, got, want)
				}
			}
		})
	}
}

// goldenCfg is the shortest match that still plays every rule: a 500 target ends
// in a handful of deals, which is what keeps a corpus this wide cheap enough to
// run on every commit.
func goldenCfg(variation string) module.MatchConfig {
	return module.MatchConfig{
		Variation: variation,
		Options:   module.Options{OptTargetScore: 500},
	}
}

func finalStateHash(t *testing.T, m *Module, cfg module.MatchConfig, players []module.PlayerRef, seed int64) string {
	t.Helper()
	state, err := m.NewMatch(cfg, players, seed)
	if err != nil {
		t.Fatalf("seed %d: NewMatch: %v", seed, err)
	}
	final, res, err := module.PlayWithOffers(m, state, players, module.DriverOptions{
		MaxActions: 4000, Prefer: driverPrefer,
	})
	if err != nil {
		t.Fatalf("seed %d: %v", seed, err)
	}
	if !res.Finished {
		t.Fatalf("seed %d: match did not finish", seed)
	}
	s, err := decode(final)
	if err != nil {
		t.Fatalf("seed %d: decode: %v", seed, err)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "actions=%d\n", res.Actions)
	verbs := make([]string, 0, len(res.Verbs))
	for v := range res.Verbs {
		verbs = append(verbs, v)
	}
	sort.Strings(verbs)
	for _, v := range verbs {
		fmt.Fprintf(&b, "verb %s=%d\n", v, res.Verbs[v])
	}
	fmt.Fprintf(&b, "winner=%d/%s target=%d\n", s.WinnerTeam, s.WinnerID, s.TargetScore)
	for _, d := range s.Deals {
		fmt.Fprintf(&b, "deal %d out=%q concealed=%t exhausted=%t\n",
			d.DealNumber, d.WentOut, d.Concealed, d.Exhausted)
		for _, tr := range d.Teams {
			fmt.Fprintf(&b, "  t%d meld=%d canastas=%d red=%d out=%d hand=%d total=%d running=%d\n",
				tr.TeamID, tr.MeldCards, tr.Canastas, tr.RedThrees, tr.GoingOut,
				tr.InHand, tr.Total, tr.Running)
		}
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:8])
}
