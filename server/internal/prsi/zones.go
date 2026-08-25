package prsi

// The ids this module renders its zones under.
//
// They live in their own file because two places need to agree on them: View,
// which draws the zones, and LegalActions, which points offers at them so an
// interface knows where a move lands. An offer naming a zone id that no zone
// has is a drop target nobody can hit, and nothing at runtime would notice —
// which is what the conformance suite checks for.
const (
	drawZoneID    = "draw"
	discardZoneID = "discard"
)

func handZoneID(playerID string) string { return "hand:" + playerID }
