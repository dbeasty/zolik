package canasta

import "strconv"

// The ids this module renders its zones under.
//
// They live in their own file because two places need to agree on them: View,
// which draws the zones, and LegalActions, which points offers at them so an
// interface knows where a move lands. An offer naming a zone id that no zone
// has is a drop target nobody can hit, and nothing at runtime would notice —
// which is what the conformance suite checks for.
//
// Melds belong to a partnership rather than a player here, so the spread is
// keyed by team. That is the same fact that made Canasta a module rather than
// a configuration of the rummy engine, showing up again in the zone ids.
const (
	drawZoneID    = "draw"
	discardZoneID = "discard"
)

func handZoneID(playerID string) string { return "hand:" + playerID }
func meldsZoneID(teamID int) string     { return "melds:t" + strconv.Itoa(teamID) }
func redThreesZoneID(teamID int) string { return "redThrees:t" + strconv.Itoa(teamID) }
