package module

// ServerDealt is implemented by a module whose deal must never leave the
// server: a game played against the deck alone, where the deck is the only
// opponent there is. Solitaire is the first.
//
// In a game with other players, every seat is the defence against a seat that
// learns the deal. With one seat there is nobody to defend, so a device that
// held the match state, or chose the seed, would have beaten the game before
// its first move. A module that declares this is never synced to a device,
// never handed over to one, and never accepted back as an offline bundle.
//
// The switch is declared by the module rather than listed by id, so a later
// FreeCell or Spider opts in the same way.
type ServerDealt interface {
	ServerDealt() bool
}

// IsServerDealt reports whether m's deal must stay on the server.
func IsServerDealt(m GameModule) bool {
	d, ok := m.(ServerDealt)
	return ok && d.ServerDealt()
}
