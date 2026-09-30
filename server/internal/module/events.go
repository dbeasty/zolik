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

// Move is one thing a player did, as one viewer may read it: a line in the
// table's "what just happened" strip.
type Move struct {
	PlayerID string `json:"playerId"`
	Fact     Fact   `json:"fact"`
	// GroupID is the group on the board the move touched, if any, so a client
	// can point at it.
	GroupID string `json:"groupId,omitempty"`
}

// EventNarrator turns a module's events into moves a person can read.
//
// A board can show what changed but not who changed it. When one player adds
// a card to another player's meld, the board looks the same as when the owner
// adds it. Only the module knows who did what, and it already said so in its
// events.
//
// The event it is handed has already been through ProjectEvent for the
// viewer, so a narrator can only say what that viewer is allowed to know.
// s is the state after the move.
type EventNarrator interface {
	NarrateEvent(s State, ev Event) (Move, bool)
}

// NarrateEvent is ev as a line for the strip, or false when m does not
// narrate it.
func NarrateEvent(m GameModule, s State, ev Event) (Move, bool) {
	if n, ok := m.(EventNarrator); ok {
		return n.NarrateEvent(s, ev)
	}
	return Move{}, false
}
