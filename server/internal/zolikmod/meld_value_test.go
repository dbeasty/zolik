package zolikmod

import (
	"testing"

	"zolik/server/internal/module"
	"zolik/server/internal/rules"
)

// Every meld on the table carries what it is worth as laid, so a player can
// read the opening minimum off the board instead of adding it up.
func TestMeldsCarryTheirValue(t *testing.T) {
	cfg := resolveConfig(module.MatchConfig{Variation: "continental"})
	gs := hintState(cfg)
	gs.Melds["p2"] = [][]string{{"9D", "9H", "9S"}, {"TD", "JD", "QD", "KD"}}
	gs.MeldMeta["p2"] = []rules.MeldInfo{
		{MeldID: "m1", Type: rules.MeldSet, OwnerID: "p2"},
		{MeldID: "m2", Type: rules.MeldRun, OwnerID: "p2"},
	}
	players := []module.PlayerRef{{ID: "p1", Name: "A"}, {ID: "p2", Name: "B"}}
	st, err := encode(&matchState{Rules: gs, Players: players})
	if err != nil {
		t.Fatal(err)
	}
	vm, err := (&Module{}).View(st, "p1")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, z := range vm.Zones {
		if z.ID == meldsZoneID("p2") {
			for _, g := range z.Groups {
				got[g.ID] = g.Value
			}
		}
	}
	if got["m1"] != "27" || got["m2"] != "40" {
		t.Errorf("meld values %v, want m1=27 m2=40", got)
	}
}
