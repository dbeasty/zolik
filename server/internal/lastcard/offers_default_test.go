package lastcard

import "testing"

// A wild's colour starts on the colour the hand holds most of — not on coral
// because coral heads the list. A wild played without a second thought then
// names a colour the player can follow.
func TestTheWildsColourStartsOnTheLongestColour(t *testing.T) {
	raw := withState(t, func(s *GameState) {
		s.Hands["p1"] = []string{cardWild, "A-1", "A-2", "V-3"}
	})
	play := offerIDs(t, raw, "p1")[OfferPlay]
	if len(play.Params) != 1 || play.Params[0].DefaultChoice != "A" {
		t.Fatalf("colour param %+v, want default A", play.Params)
	}
}
