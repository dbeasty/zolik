package canasta

import (
	"encoding/json"

	"zolik/server/internal/module"
)

// The shipped Canasta model was trained on classic, modern American and
// Samba. CanastaX is Samba plus house rules the network has never seen
// (poaching, dirty sequences, rearranging a canasta), and variationName shows
// it to the network as plain Samba. At a partnership table that costs badly.
// On held-out seeds (gamebench, first seed 1,000,000) the model's side lost
// 2,468 ± 291 a match to Hard and 2,585 ± 291 to Medium at four seats, and
// 4,315 ± 300 to Hard at six. Without partners it still won: +6,417 ± 254 at
// two seats and +4,827 ± 246 at three.
//
// So at a CanastaX partnership table an AI seat plays Hard (learn.Scoped).
// Lift this when a model trained on CanastaX passes the bench at four and six.
func modelTrainedFor(variation string, seats int) bool {
	return variation != variationCanastaX || seats <= 3
}

// trainedTable reads only the two fields the decision needs, since it runs on
// every AI move. A state that does not decode goes to the model, which already
// falls back on anything it cannot read.
func trainedTable(s module.State) bool {
	var t struct {
		Variation string   `json:"variation"`
		TurnOrder []string `json:"turnOrder"`
	}
	if err := json.Unmarshal(s, &t); err != nil {
		return true
	}
	return modelTrainedFor(t.Variation, len(t.TurnOrder))
}
