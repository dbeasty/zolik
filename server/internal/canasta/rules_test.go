package canasta

import (
	"testing"

	"zolik/server/internal/module"
)

// The rules screen is where a player finds out that Modern American will never
// let a black three leave their hand — there is no other way to learn it short
// of trying the move and being refused.
//
// So the sentence is asserted per variation, both that the right one is stated
// and that the other is not: a bug that stated both would read as a rule sheet
// contradicting itself, and one that stated neither would leave the refusal
// with nothing behind it.
func TestEachVariationStatesItsOwnBlackThreeRule(t *testing.T) {
	const (
		goOut = "canasta.rules.blackThreesGoOut"
		never = "canasta.rules.blackThreesNeverMeld"
	)
	cases := []struct {
		variation  string
		want, gone string
	}{
		{"classic", goOut, never},
		{"samba", goOut, never},
		{"modern_american", never, goOut},
	}

	m := New()
	for _, tc := range cases {
		t.Run(tc.variation, func(t *testing.T) {
			sections, err := m.Rules(module.MatchConfig{Variation: tc.variation})
			if err != nil {
				t.Fatalf("Rules: %v", err)
			}

			var stated *module.RuleItem
			for _, sec := range sections {
				for i, it := range sec.Items {
					if it.LabelKey == tc.gone {
						t.Errorf("states %q, which is the other variation's rule", tc.gone)
					}
					if it.LabelKey == tc.want {
						stated = &sec.Items[i]
					}
				}
			}
			if stated == nil {
				t.Fatalf("states no black-three rule at all")
			}
			// The sentence carries the number, so a repricing cannot leave the
			// rules screen quoting the old one.
			if got := stated.Params["n"]; got != blackThreeValue {
				t.Errorf("the sentence says %v points, want %d", got, blackThreeValue)
			}
		})
	}
}

// The written rules used to say "play in partnerships" at every table, though
// two, three and five players play alone (seatsToTeams), and never stated what
// a canasta or a red three pays. Each is now read off the engine's numbers.
func TestRulesStatePartnershipsAndBonuses(t *testing.T) {
	find := func(sections []module.RuleSection, key string) (map[string]any, bool) {
		for _, sec := range sections {
			for _, it := range sec.Items {
				if it.LabelKey == key {
					return it.Params, true
				}
			}
		}
		return nil, false
	}
	for name, v := range variations {
		t.Run(name, func(t *testing.T) {
			sections, err := New().Rules(module.MatchConfig{Variation: name})
			if err != nil {
				t.Fatal(err)
			}
			r := resolveRules(module.MatchConfig{Variation: name}, 0)
			partners := "canasta.rules.partnerships"
			if v.MaxSeats >= 6 {
				partners = "canasta.rules.partnershipsSix"
			}
			if _, ok := find(sections, partners); !ok {
				t.Errorf("expected %s", partners)
			}
			// What the sentences claim about who partners whom.
			for seats, teams := range map[int]int{2: 2, 3: 3, 4: 2, 5: 5, 6: 3} {
				if got := seatsToTeams(seats); got != teams {
					t.Errorf("%d seats: %d sides, the rule says %d", seats, got, teams)
				}
			}
			p, ok := find(sections, "canasta.rules.canastaBonus")
			if !ok || p["natural"] != r.NaturalCanastaBonus || p["mixed"] != r.MixedCanastaBonus {
				t.Errorf("canastaBonus = %v, want %d/%d", p, r.NaturalCanastaBonus, r.MixedCanastaBonus)
			}
			p, ok = find(sections, "canasta.rules.redThreeValues")
			if !ok || p["n"] != redThreeValue || p["count"] != r.redThrees() || p["all"] != r.redThreeAllBonus() {
				t.Errorf("redThreeValues = %v", p)
			}

			// The red-three sentence matches redThreeScore: one canasta short
			// of RedThreesNeed costs what the sentence says.
			key := "canasta.rules.redThrees"
			if r.RedThreesNeed > 1 {
				key = "canasta.rules.redThreesNeedTwo"
				if !r.RedThreePenaltyFlat {
					t.Fatalf("%s says each three counts %d against you, but the penalty is not flat", key, redThreeValue)
				}
				team := &Team{RedThrees: []string{"3H", "3D"}}
				if got := redThreeScore(r, team); got != -2*redThreeValue {
					t.Errorf("two red threes, no canastas: %d, want %d", got, -2*redThreeValue)
				}
			}
			if _, ok := find(sections, key); !ok {
				t.Errorf("expected %s", key)
			}
		})
	}
}
