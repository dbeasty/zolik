package module

// OptOpenDiscardPile decides whether a discard pile may be looked through, or
// shows only the card on top.
//
// Declared here rather than in each game's own option list for the same reason
// OptPauseBetweenRounds is: what a pile *publishes* is a property of how a
// match is presented, and every module's engine option list stays about rules.
// Four games deal a discard pile and each one answered the question with a
// constant — Žolíky sent the whole pile, Canasta, Gin Rummy and Prší sent the
// top card — which is exactly the shape of a house rule left as a hard-coded
// decision.
//
// It changes what a viewer is *sent*, never what anybody may do: an offer that
// was legal with a folded pile is legal with an open one. A client already
// folds a pile down to its top card and opens it on a tap (ZoneView), so the
// option decides whether that control exists at all.
const OptOpenDiscardPile = "openDiscardPile"

// OpenDiscardPileOption is the ready-made spec a module drops into its
// descriptor.
func OpenDiscardPileOption() OptionSpec {
	return OptionSpec{
		Name:  OptOpenDiscardPile,
		Type:  OptionEnumInt,
		Label: "Discard pile",
		Help:  "Whether players may look under the top card at everything the pile holds.",
		Choices: []OptionChoice{
			{Value: OptOn, Label: "Open to look through"},
			{Value: OptOff, Label: "Top card only"},
		},
	}
}

// OpenDiscardPile reads the option, against the module's own default — which
// differs per game, because what the pile has always shown differs per game.
func (c MatchConfig) OpenDiscardPile(dflt bool) bool {
	return c.Opt(OptOpenDiscardPile, BoolOpt(dflt)) == OptOn
}
