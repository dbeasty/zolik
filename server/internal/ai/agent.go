package ai

import "zolik/server/internal/rules"

// VisibleState is the public board snapshot accessible to agents.
type VisibleState struct {
	// GameNumber is which deal of the match this is (1-7).
	GameNumber int
	// Round is the lap-around-the-table counter within the current deal;
	// gates DiscardDrawMinRound below.
	Round       int
	Phase       string
	CurrentTurn string

	DiscardPile []string
	// PlayerDiscards is each player's discard history this game, oldest
	// first, derived from the action log. Lets an agent avoid re-offering
	// a rank/suit a player has already passed on, and avoid feeding cards
	// into melds already on the table.
	PlayerDiscards map[string][]string
	Melds          map[string][][]string
	MeldMeta       map[string][]rules.MeldInfo
	RoundReqMet    map[string]bool
	TotalScores    map[string]int
	// Rules is the game's resolved ruleset — see rules.RulesConfig. Agents
	// must use this instead of any hardcoded set/run size, meld-value floor,
	// discard-lock round or contract assumption so they behave correctly
	// under every profile. It is the single source for all of those; there
	// are deliberately no InitialMeldMinimum/DiscardDrawMinRound fields
	// alongside it to drift out of sync with it.
	Rules rules.RulesConfig
	// DiscardTakenCard is the card the current player took off the discard
	// pile this turn, if any — see rules.GameState.DiscardTakenCard. The
	// engine will not accept it back as this turn's discard, so an agent
	// that ignores it can talk itself into a hand it has no legal move out
	// of. Empty when the turn's draw came from the deck.
	DiscardTakenCard string
	// PendingJokers holds the jokers the current player has taken off the
	// table this turn and not yet played back into a meld — see
	// rules.GameState.JokersReclaimedPendingMeld. Under
	// Rules.JokerReclaimMustPlay the engine refuses the turn-ending discard
	// while any remain, so an agent that ignores this owes a move it will
	// never make and wedges its own turn.
	PendingJokers []string
}

type Agent interface {
	ChooseAction(visible VisibleState, hand []string) rules.Action
	Difficulty() string
}
