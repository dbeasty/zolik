package module

import "testing"

func TestCheckLines(t *testing.T) {
	three := 3
	cases := []struct {
		name string
		rs   RoundScore
		ok   bool
	}{
		{"no lines is no claim", RoundScore{Delta: 50}, true},
		{"balances", RoundScore{Delta: 30, Lines: []ScoreLine{
			{LabelKey: "a", Points: 50, Sub: []ScoreLine{{LabelKey: "a1", Points: 20}, {LabelKey: "a2", Points: 30}}},
			{LabelKey: "b", Points: -20},
			{LabelKey: "note"},
		}}, true},
		{"total off", RoundScore{Delta: 31, Lines: []ScoreLine{{LabelKey: "a", Points: 30}}}, false},
		{"parts off", RoundScore{Delta: 50, Lines: []ScoreLine{
			{LabelKey: "a", Points: 50, Sub: []ScoreLine{{LabelKey: "a1", Points: 20}}},
		}}, false},
		{"shown wins over delta", RoundScore{Delta: -3, Shown: &three, Lines: []ScoreLine{{LabelKey: "a", Points: 3}}}, true},
	}
	for _, c := range cases {
		err := CheckLines(c.rs)
		if (err == nil) != c.ok {
			t.Errorf("%s: err = %v, want ok=%v", c.name, err, c.ok)
		}
	}
}
