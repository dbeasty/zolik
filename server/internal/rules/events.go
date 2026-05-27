package rules

// StateEvent is a public or semi-public game occurrence produced by the rules engine.
type StateEvent struct {
	Type string
	Data map[string]interface{}
}

// ApplyOutcome is the result of applying one player action.
type ApplyOutcome struct {
	State  GameState
	Events []StateEvent
}

func ev(t string, data map[string]interface{}) StateEvent {
	if data == nil {
		data = map[string]interface{}{}
	}
	return StateEvent{Type: t, Data: data}
}
