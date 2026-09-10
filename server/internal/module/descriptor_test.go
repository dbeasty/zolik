package module

import "testing"

// SeatRange is the one answer to "how many seats does this table have", and the
// property that matters is that a variation which says nothing keeps the answer
// it had before variations could say anything at all.
func TestSeatRangeFallsBackToTheModule(t *testing.T) {
	d := ModuleDescriptor{
		MinPlayers: 2,
		MaxPlayers: 6,
		Variations: []VariationSpec{
			{ID: "silent"},
			{ID: "narrowed", MaxPlayers: 4},
			{ID: "both", MinPlayers: 3, MaxPlayers: 5},
		},
	}

	cases := []struct {
		variation string
		min, max  int
		why       string
	}{
		{"silent", 2, 6, "a variation that names no range is the module's range"},
		{"narrowed", 2, 4, "a variation may cap the seats without touching the floor"},
		{"both", 3, 5, "a variation may narrow both ends"},
		{"", 2, 6, "no variation at all is the module's range"},
		{"nonexistent", 2, 6, "an unknown variation cannot narrow anything"},
	}
	for _, tc := range cases {
		min, max := d.SeatRange(tc.variation)
		if min != tc.min || max != tc.max {
			t.Errorf("SeatRange(%q) = %d..%d, want %d..%d — %s",
				tc.variation, min, max, tc.min, tc.max, tc.why)
		}
	}
}
