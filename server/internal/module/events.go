package module

// EventProjector filters a module's events per viewer, the way View filters
// its board.
//
// The runtime publishes every Event from Apply to every seated player. Most
// events are public by construction — a meld laid, a card discarded, a count
// of cards drawn — but a few name a card only its owner may see, such as the
// card drawn blind from the deck. A module with any such event implements
// this, and the runtime sends each viewer only what ProjectEvent returns.
//
// It is optional because most games have nothing to hide in their events;
// allmodules_test checks that every module's events, projected, never name a
// card the viewer's own board does not show.
type EventProjector interface {
	// ProjectEvent returns ev as viewerID may see it, and false when that
	// viewer should not receive it at all.
	ProjectEvent(ev Event, viewerID string) (Event, bool)
}

// ProjectEvent is ev as viewerID may see it under m, or false to drop it.
func ProjectEvent(m GameModule, ev Event, viewerID string) (Event, bool) {
	if p, ok := m.(EventProjector); ok {
		return p.ProjectEvent(ev, viewerID)
	}
	return ev, true
}
