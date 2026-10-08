package prsi

import (
	"testing"

	"zolik/server/internal/module"
)

// The queen's suit starts on the suit the hand holds most of, queens aside —
// not on hearts because hearts heads the list.
func TestTheQueensSuitStartsOnTheLongestSuit(t *testing.T) {
	raw := withState(t, func(s *GameState) {
		s.DiscardPile = []string{"9S"}
		s.Hands["p1"] = []string{"QH", "QD", "8C", "TC"}
	})
	offers, err := New().LegalActions(raw, "p1")
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range offers {
		if o.ID != OfferPlay {
			continue
		}
		if len(o.Params) != 1 || o.Params[0].DefaultChoice != "C" {
			t.Fatalf("suit param %+v, want default C", o.Params)
		}
		if a, ok := module.SubmissionFor(o); !ok || a.Params["suit"] != "C" {
			t.Errorf("a submission built from the offer names %q, want C", a.Params["suit"])
		}
		return
	}
	t.Fatal("no play offer")
}
