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
	// DealDiscards is every discard of the current deal, oldest first, with
	// the seat that made it.
	//
	// It replaced a PlayerDiscards map, and the difference is *order*. A map
	// could say that somebody had passed on a king at some point; it could
	// not say whether that was one lap ago or fifteen, which is exactly the
	// distinction Profile.Recall is drawn along. A weak player remembers the
	// last lap; a strong one remembers the deal.
	//
	// It is also, deliberately, the whole history rather than the part any
	// one agent is entitled to. VisibleState is what is *publicly knowable*;
	// how much of that a given strength actually looks at is the profile's
	// business, not the adapter's. See DiscardsBy.
	DealDiscards []SeenDiscard
	// KnownHeld is, per seat, the cards that seat was publicly seen to take
	// off the discard pile (or reclaim off the table) and has not since put
	// back down.
	//
	// It is not a peek. Every card in it was face up on the pile in front of
	// everybody when it was taken; a human opponent watching the table knows
	// exactly the same thing, and the ledger that builds it never touches a
	// hand. What it buys is the difference between "that card is somewhere"
	// and "that card is in Karel's hand and he wants a nine" — the first is
	// all the agent had, and it is the reason it could never count outs.
	KnownHeld map[string][]string
	// HandCounts is how many cards each seat holds, which every client
	// already renders (zone.opponentHand, seat.cards). Without it the agent
	// cannot tell a table that is about to end from one that just started,
	// and so keeps building a hand while somebody goes out on it.
	HandCounts map[string]int
	// DeckRemaining is how many cards are left in the stock, again public.
	DeckRemaining int
	// DeckCount is how many packs were shuffled together, which fixes the
	// number of copies of every card in play and therefore makes counting
	// outs exact rather than approximate. See rules.DeckCountForPlayers.
	DeckCount   int
	Melds       map[string][][]string
	MeldMeta    map[string][]rules.MeldInfo
	RoundReqMet map[string]bool
	TotalScores map[string]int
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
	// PendingMeldCard is a card taken off the discard pile that the engine
	// now *requires* to be part of this turn's initial meld — see
	// rules.GameState.DiscardDrawnCardPendingMeld. Empty once the obligation
	// is satisfied, or moot because the player is already down.
	//
	// An agent that ignores it can plan a perfectly good qualifying
	// combination that happens not to use the card it owes, lay it, and then
	// be refused its own discard with no move left that would satisfy
	// anybody — the turn is over and there is nothing legal in it.
	PendingMeldCard string
	// PendingJokers holds the jokers the current player has taken off the
	// table this turn and not yet played back into a meld — see
	// rules.GameState.JokersReclaimedPendingMeld. Under
	// Rules.JokerReclaimMustPlay the engine refuses the turn-ending discard
	// while any remain, so an agent that ignores this owes a move it will
	// never make and wedges its own turn.
	PendingJokers []string
}

// SeenDiscard is one card put on the pile, and by whom.
type SeenDiscard struct {
	Player string `json:"p"`
	Card   string `json:"c"`
}

// DiscardsBy is each seat's discard history as far back as recall reaches,
// oldest first.
//
// recall is in laps of the table, so it is comparable across tables of two
// players and of six; RecallPerfect returns the whole deal. A recall of zero
// returns nothing at all, which is what makes "does not remember" a setting
// rather than an absence of code.
func (v VisibleState) DiscardsBy(recall, seats int) map[string][]string {
	out := map[string][]string{}
	if recall <= 0 || len(v.DealDiscards) == 0 {
		return out
	}
	from := 0
	if recall < RecallPerfect {
		if seats < 1 {
			seats = 1
		}
		if span := recall * seats; span < len(v.DealDiscards) {
			from = len(v.DealDiscards) - span
		}
	}
	for _, d := range v.DealDiscards[from:] {
		out[d.Player] = append(out[d.Player], d.Card)
	}
	return out
}

// Agent is a player of this game.
type Agent interface {
	ChooseAction(visible VisibleState, hand []string) rules.Action
	Difficulty() string
}
