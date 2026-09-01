package rummytiles

const (
	poolZoneID  = "pool"
	tableZoneID = "table"
	trayZoneID  = "tray"
)

func handZoneID(playerID string) string { return "hand:" + playerID }
