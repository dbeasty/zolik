package module

// OptHints decides whether a player may ask for a suggested move.
//
// A hint is the move this seat's own bot would make right now, shown and never
// made. Whether a table allows that is a house rule, and like every house rule
// it is a declared option rather than a constant. It is declared once here
// because it is one setting with one name on the wire whichever game it is
// set on, the same way PauseOption is.
//
// On by default: the people who asked for it are the ones still learning the
// game, and a host who wants a table without it turns it off in the lobby.
const OptHints = "hints"

// HintsOption is the ready-made spec a module drops into its descriptor.
func HintsOption() OptionSpec {
	return OptionSpec{
		Name:  OptHints,
		Type:  OptionEnumInt,
		Label: "Hints",
		Help:  "Let players ask for a suggested move when they are stuck.",
		Choices: []OptionChoice{
			{Value: OptOn, Label: "Allowed"},
			{Value: OptOff, Label: "Off"},
		},
	}
}

// HintsAllowed reads the option. A match created before the option existed has
// no value for it, and gets hints.
func (c MatchConfig) HintsAllowed() bool {
	return c.Opt(OptHints, OptOn) == OptOn
}
