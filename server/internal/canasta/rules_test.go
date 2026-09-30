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
