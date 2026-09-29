package zolikmod

import "zolik/server/internal/module"

// ProjectEvent keeps the card drawn blind from the deck with the player who
// drew it. Everyone else learns that a card was drawn from the player_drew
// event that follows it, which is all their board shows either.
func (m *Module) ProjectEvent(ev module.Event, viewerID string) (module.Event, bool) {
	if ev.Type == "draw_deck" && ev.Data["playerId"] != viewerID {
		return ev, false
	}
	return ev, true
}
