package ui

import "testing"

import "zolik/client-tui/api"

// The bot count is the one number this screen holds that the server will
// refuse if it is wrong, so the clamp is what is worth testing: a table too
// small does not deal, and a table too large is rejected a seat at a time.
func TestBotsFor(t *testing.T) {
	prsi := api.Module{ID: "prsi", MinPlayers: 2, MaxPlayers: 6}
	canasta := api.Module{ID: "canasta", MinPlayers: 2, MaxPlayers: 4}

	cases := []struct {
		name   string
		mod    api.Module
		picked map[string]int
		want   int
	}{
		// Pressing enter without touching +/− opens the smallest legal
		// table, which is what it has always done.
		{"unset is the smallest legal table", prsi, map[string]int{}, 1},
		{"a typed count is kept", prsi, map[string]int{"prsi": 4}, 4},
		{"never more than the seats left over", prsi, map[string]int{"prsi": 9}, 5},
		{"never fewer than the game needs", prsi, map[string]int{"prsi": 0}, 1},
		// One module's count is not another's: the map is keyed by id, so
		// adjusting Prší leaves Canasta where it was.
		{"another module's count does not leak", canasta, map[string]int{"prsi": 5}, 1},
		{"clamped to this module's own maximum", canasta, map[string]int{"canasta": 5}, 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := botsFor(tc.mod, tc.picked); got != tc.want {
				t.Fatalf("botsFor() = %d, want %d", got, tc.want)
			}
		})
	}
}
