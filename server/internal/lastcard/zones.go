package lastcard

// The ids this module renders its zones under. View draws them and
// LegalActions points offers at them, so the two must agree.
const (
	drawZoneID    = "draw"
	discardZoneID = "discard"
)

func handZoneID(playerID string) string { return "hand:" + playerID }
