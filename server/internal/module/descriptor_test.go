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

// A default is a suggestion inside the seat range, never a way out of it.
func TestDefaultSeatsStaysInsideTheSeatRange(t *testing.T) {
	d := ModuleDescriptor{
		MinPlayers:     2,
		MaxPlayers:     6,
		DefaultPlayers: 4,
		Variations: []VariationSpec{
			{ID: "silent"},
			{ID: "own", DefaultPlayers: 5},
			{ID: "pair", MinPlayers: 2, MaxPlayers: 2},
			{ID: "six", MinPlayers: 6, MaxPlayers: 6},
		},
	}
	for variation, want := range map[string]int{"silent": 4, "own": 5, "pair": 2, "six": 6, "": 4} {
		if got := d.DefaultSeats(variation); got != want {
			t.Errorf("DefaultSeats(%q) = %d, want %d", variation, got, want)
		}
	}
	if got := (ModuleDescriptor{MinPlayers: 3, MaxPlayers: 4}).DefaultSeats(""); got != 3 {
		t.Errorf("a module naming no default = %d, want its smallest table, 3", got)
	}
}

func TestVariationAnswersToItsFormerIDs(t *testing.T) {
	d := ModuleDescriptor{Variations: []VariationSpec{
		{ID: "a"},
		{ID: "b", Formerly: []string{"old-b", "a"}},
	}}
	if v := d.Variation("old-b"); v == nil || v.ID != "b" {
		t.Errorf("a retired ID should resolve to the variation that took it over, got %+v", v)
	}
	// A current ID always wins over somebody's history.
	if v := d.Variation("a"); v == nil || v.ID != "a" {
		t.Errorf("a current ID should resolve to itself, got %+v", v)
	}
	if d.Variation("never") != nil {
		t.Error("an unknown ID should not resolve")
	}
}
