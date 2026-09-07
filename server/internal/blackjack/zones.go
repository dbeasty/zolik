package blackjack

const (
	dealerZoneID = "dealer"
	shoeZoneID   = "shoe"
)

func boxZoneID(playerID string) string { return "box:" + playerID }
