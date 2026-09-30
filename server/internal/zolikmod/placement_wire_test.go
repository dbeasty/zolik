package zolikmod

import (
	"encoding/json"
	"testing"

	"zolik/server/internal/module"
	"zolik/server/internal/rules"
)

// The adapter copies a placement field by field, which is exactly the shape
// that drops a newly added one in silence — the offer still arrives, still
// looks right, and the client quietly refuses a move the engine takes.
//
// So this walks the whole path on the board that found the bug: someone
// else's run of 5-6-7-8, and a hand holding the joker, the 10 and the 9. The
// 10 reaches the run either way, and both ways have to survive the trip out
// to JSON.
func TestPlacementAlternativesSurviveTheWire(t *testing.T) {
	cfg := rules.ProfileContinental
	cfg.InitialMeldMinimum = 0
	st := &matchState{
		Rules: rules.GameState{
			Status:      rules.StatusActive,
			Rules:       cfg,
			GameNumber:  1,
			Round:       1,
			Phase:       rules.PhaseMeld,
			CurrentTurn: "p1",
			TurnOrder:   []string{"p1", "p2"},
			Hands:       map[string][]string{"p1": {"JOKER", "TC", "9C", "2H"}, "p2": {"2D"}},
			Melds:       map[string][][]string{"p2": {{"5C", "6C", "7C", "8C"}}},
			MeldMeta: map[string][]rules.MeldInfo{"p2": {
				{MeldID: "m1", Type: rules.MeldRun, OwnerID: "p2"},
			}},
			RoundReqMet: map[string]bool{"p1": true, "p2": true},
			GameScores:  map[string][]int{},
			TotalScores: map[string]int{},
		},
		Players: []module.PlayerRef{{ID: "p1", Name: "A"}, {ID: "p2", Name: "B"}},
	}
	raw, err := encode(st)
	if err != nil {
		t.Fatal(err)
	}

	offers, err := (&Module{}).LegalActions(raw, "p1")
	if err != nil {
		t.Fatal(err)
	}
	var layOff *module.ActionOffer
	for i := range offers {
		if offers[i].ID == rules.LayOffOfferID("m1") {
			layOff = &offers[i]
		}
	}
	if layOff == nil || layOff.Source == nil {
		t.Fatal("no lay-off offer for the run on the table")
	}

	// Through JSON and back, because the wire is what the client sees and an
	// omitted struct tag fails nowhere else.
	blob, err := json.Marshal(layOff)
	if err != nil {
		t.Fatal(err)
	}
	var got module.ActionOffer
	if err := json.Unmarshal(blob, &got); err != nil {
		t.Fatal(err)
	}

	var ten *module.Placement
	for i := range got.Source.Placements {
		if got.Source.Placements[i].Card == "TC" {
			ten = &got.Source.Placements[i]
		}
	}
	if ten == nil {
		t.Fatalf("the 10 is playable with company and must reach the client: %s", blob)
	}
	if len(ten.Requires) == 0 {
		t.Fatalf("the 10 does not reach the run alone: %s", blob)
	}
	if len(ten.Alternatives) == 0 {
		t.Fatalf("the joker is company for the 10 and the client never heard: %s", blob)
	}
	found := false
	for _, alt := range ten.Alternatives {
		if len(alt) == 1 && alt[0] == "JOKER" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected the joker as an alternative companion, got %v", ten.Alternatives)
	}
}
