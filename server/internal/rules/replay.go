package rules

import "fmt"

// LoggedAction pairs an Action with the player who submitted it, the unit
// ReplayActions and a stored action log (see game.RawAction) work in.
type LoggedAction struct {
	PlayerID string
	Action   Action
}

// ReplayActions re-applies a sequence of previously-accepted actions to
// initial, in order, through the same ApplyAction path that produced them
// originally. It is pure and deterministic: given the same initial state
// (including DeckSeed) and the same actions, it reproduces the same final
// state every time — no live randomness or IO involved. Used for verifying
// a stored game log is internally consistent, and as the basis for
// takeback (truncate the log, replay what's left).
//
// An action that fails to apply is treated as a bug in the stored log (it
// was accepted once; it must still be valid against the state that log
// produced) rather than a normal error, so ReplayActions stops immediately
// and reports which entry broke.
func ReplayActions(initial GameState, actions []LoggedAction) (GameState, error) {
	state := initial
	for i, la := range actions {
		outcome, err := ApplyAction(state, la.PlayerID, la.Action)
		if err != nil {
			return state, fmt.Errorf("replay: action %d (%s by %s) rejected: %w", i, la.Action.Type, la.PlayerID, err)
		}
		state = outcome.State
	}
	return state, nil
}
