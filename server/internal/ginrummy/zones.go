package ginrummy

const (
	discardZoneID = "discard"
	stockZoneID   = "stock"
	meldsZoneID   = "melds"
)

func handZoneID(playerID string) string { return "hand:" + playerID }
