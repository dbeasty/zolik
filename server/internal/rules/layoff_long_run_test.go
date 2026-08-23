package rules

import "testing"

// A run that has grown all the way from the deuce to the ten keeps accepting
// the jack, and it does so through the lay-off path a drag-and-drop actually
// takes — not just through ValidateMeld. The run's top slot here is a joker
// standing in for the ten, which is the shape that made the old bridge check
// read "low natural plus high natural" and refuse the drop outright.
func TestLayOffGrowsALongRunPastTheJack(t *testing.T) {
	const me = "david"
	const karel = "karel"
	run := []string{"2D", "3D", "JOKER2", "5D", "6D", "7D", "JOKER2", "9D", "JOKER1"}

	st := GameState{
		Status:      StatusActive,
		Phase:       PhaseMeld,
		CurrentTurn: me,
		Hands:       map[string][]string{me: {"QD", "JD"}},
		Melds:       map[string][][]string{karel: {run}},
		RoundReqMet: map[string]bool{me: true},
		MeldMeta:    map[string][]MeldInfo{karel: {{MeldID: "m1"}}},
	}
	const meldID = "m1"

	if _, err := ValidateLayOff(st, me, meldID, []string{"JD"}, "end"); err != nil {
		t.Fatalf("dropping JD on the end of the run was refused: %v", err)
	}
}
