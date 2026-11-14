package gamemcp

import (
	"fmt"
	"testing"

	"zolik/server/internal/module"
)

// defaultTables is how many players each game is dealt at when the player has
// said nothing, counting the player: one in-range answer per variation, chosen
// on purpose. A new game fails here until someone has chosen its table.
var defaultTables = map[string]map[string]int{
	"zolik":      {"zolik_classic": 4, "continental": 4},
	"prsi":       {"classic": 4},
	"canasta":    {"classic": 4, "modern_american": 4, "samba": 4, "canastax": 4},
	"holdem":     {"holdem": 6, "omaha": 6, "draw": 5},
	"ginrummy":   {"standard": 2, "oklahoma": 2},
	"rummytiles": {"standard": 4},
	// Every hand is against the dealer, so another seat only adds waiting.
	"blackjack": {"vegas": 2, "atlantic": 2, "single": 2},
	// Three is the game; at four the dealer sits out each deal.
	"marias":     {"voleny": 3, "licitovany": 3},
	"klondike":   {"classic": 1, "draw3": 1, "vegas": 1},
	"lastcard":   {"classic": 4},
	"snaps":      {"snaps": 2, "sedesatSest": 2},
	"sedma":      {"classic": 4},
	"okobere":    {"classic": 4},
	"ferbl":      {"classic": 4},
	"lora":       {"classic": 4},
	"sietemedio": {"mexicana": 4, "cantina": 4, "abierta": 4},
	"pinochle":   {"partnership": 4, "cutthroat": 3, "twohanded": 2, "doubledeck": 4},
	"blackpeter": {"classic": 4},
}

// TestEveryGameDealsAtItsDefaultTable pins the table above, and proves each one
// is a table the game will really deal: Play now presses straight through.
func TestEveryGameDealsAtItsDefaultTable(t *testing.T) {
	for _, id := range registry.IDs() {
		m := registry.Get(id)
		d := m.Descriptor()
		want, ok := defaultTables[id]
		if !ok {
			t.Errorf("%s has no default table chosen: add it to defaultTables", id)
			continue
		}
		for _, v := range d.Variations {
			seats, ok := want[v.ID]
			if !ok {
				t.Errorf("%s/%s has no default table chosen", id, v.ID)
				continue
			}
			if got := d.DefaultSeats(v.ID); got != seats {
				t.Errorf("%s/%s defaults to %d players, want %d", id, v.ID, got, seats)
			}
			players := make([]module.PlayerRef, seats)
			for i := range players {
				players[i] = module.PlayerRef{ID: fmt.Sprintf("p%d", i)}
			}
			cfg := module.MatchConfig{Variation: v.ID}
			if _, err := m.NewMatch(cfg, players, 1); err != nil {
				t.Errorf("%s/%s will not deal at its default %d players: %v", id, v.ID, seats, err)
			}
		}
	}
}
