package ai

import (
	"zolik/server/internal/rules"
)

// What everybody at the table saw.
//
// The agent is handed a VisibleState rebuilt from scratch on every call, which
// is the right shape for almost everything — the pile, the melds and the hand
// counts are all readable off the current state, and re-deriving them means
// there is no second copy to drift. Two things are not readable off the
// current state, because they are history rather than position:
//
//	Discards  who threw what, in order. The pile says what is on it now; it
//	          cannot say that Karel passed on a king eleven turns ago, and it
//	          loses the whole lot when the stock runs out and the pile is
//	          recycled.
//	Held      which cards a seat was *seen* to take. When Karel takes the nine
//	          of hearts off the pile, everyone watching knows where that nine
//	          is. Nothing in the position remembers it a moment later.
//
// So those two accumulate here, and everything else stays derived. That split
// is what makes the reshuffle case correct for free: unseen cards are counted
// against the pile *as it stands*, so the moment the pile is recycled into the
// stock every card in it goes back to being drawable without a line of code
// noticing (see knowledge.buildUnseen).
//
// Nothing here reads a hand. The ledger is fed the before and after of an
// action that has already been applied and works out what was public about it,
// which is the same thing a person at the table does.
type Ledger struct {
	// Deal is the deal these observations belong to. A new deal is a new
	// table: the pile is fresh, the melds are gone, and remembering last
	// deal's discards would be worse than remembering nothing.
	Deal int `json:"deal,omitempty"`
	// Discards is every discard of this deal, oldest first.
	Discards []SeenDiscard `json:"discards,omitempty"`
	// Held is, per seat, the cards that seat was seen to take and has not
	// since put back down.
	Held map[string][]string `json:"held,omitempty"`
	// heldAtTurnStart is Held as it was when the current turn's draw
	// happened, so that an undo — which rewinds every meld and lay-off back
	// to the draw — rewinds this too. Undo cannot reach past the draw
	// (rules.ValidateUndoTurn), so one snapshot is exactly deep enough.
	HeldAtTurnStart map[string][]string `json:"heldAtTurnStart,omitempty"`
}

// Snapshot is the little of a state an observation needs from *before* the
// action.
//
// Three fields rather than a whole cloned GameState, because that is genuinely
// all of it — and because cloning every hand on the table, on every action of
// every match, to look at a discard pile would be an odd price to pay for a
// memory.
type Snapshot struct {
	Deal          int
	DiscardPile   []string
	PendingJokers []string
}

// Before captures what Observe will need. Call it immediately before applying
// the action; the slices are copied, because the engine mutates in place.
func Before(gs rules.GameState) Snapshot {
	return Snapshot{
		Deal:          gs.GameNumber,
		DiscardPile:   append([]string(nil), gs.DiscardPile...),
		PendingJokers: append([]string(nil), gs.JokersReclaimedPendingMeld...),
	}
}

// Observe records what was public about one applied action.
//
// before and after straddle it. Taking both is what lets this work without
// restating a single rule: how many cards a discard-pile draw actually took,
// whether a lay-off bought a joker back, and whether the deal ended are all
// differences between two states, and the engine has already decided every one
// of them.
func (l *Ledger) Observe(before Snapshot, after rules.GameState, playerID string, a rules.Action) {
	if l.Held == nil {
		l.Held = map[string][]string{}
	}
	// A new deal wipes the slate. Checked first, because an action that ends
	// a deal has already re-dealt by the time `after` is handed over — so
	// anything it did belongs to a table that no longer exists.
	if after.GameNumber != l.Deal {
		l.reset(after.GameNumber)
		return
	}

	switch a.Type {
	case rules.ActionDiscard:
		l.Discards = append(l.Discards, SeenDiscard{Player: playerID, Card: a.Card})
		l.drop(playerID, []string{a.Card})

	case rules.ActionDrawCard:
		if a.DrawFrom == rules.DrawFromDiscard {
			// How many cards came off the pile is the engine's decision, not
			// this file's: under DiscardPickupAnyFromPile taking one card takes
			// everything above it, and re-deriving that here would be a second
			// copy of a rule that already exists. The difference between the two
			// piles says it exactly.
			if n := len(before.DiscardPile) - len(after.DiscardPile); n > 0 {
				l.take(playerID, before.DiscardPile[len(after.DiscardPile):])
			}
		}
		// The rewind point goes here, *after* the pickup is recorded rather
		// than before it. Undo restores the table to the state right after
		// the draw (rules.ValidateUndoTurn), and the card taken off the pile
		// is part of that state — everybody watched it move. Snapshotting a
		// moment earlier quietly un-learned it.
		l.snapshotTurn()

	case rules.ActionLayMeld:
		l.drop(playerID, a.Cards)

	case rules.ActionLayOff:
		l.drop(playerID, layOffCards(a))
		// A lay-off that lands a natural in a joker's exact place buys the
		// joker back into the hand, in front of everybody.
		if after.LastLayOff != nil {
			l.take(playerID, after.LastLayOff.ReclaimedJokers)
		}

	case rules.ActionSwapJoker:
		// The natural goes into the meld and the joker comes out into the
		// hand, where the engine records it as a debt. LastLayOff is
		// deliberately cleared by a swap, so the debt is the signal — and it
		// is the same signal in both directions, since playing the joker back
		// clears it again.
		l.drop(playerID, []string{a.Card})
		l.take(playerID, addedJokers(before.PendingJokers, after.JokersReclaimedPendingMeld))

	case rules.ActionUndoDrawDiscard:
		// The cards go back on the pile in front of everybody, so what was
		// known about them stops being known about a hand. The pile grew by
		// exactly what returned.
		if n := len(after.DiscardPile) - len(before.DiscardPile); n > 0 {
			l.drop(playerID, after.DiscardPile[len(before.DiscardPile):])
		}

	case rules.ActionUndoTurn, rules.ActionUndoLayOff, rules.ActionUndoLayMeld:
		// Undo puts this turn's melds and lay-offs back in the hand. Every
		// one of those cards was face up on the table, so what is publicly
		// known about them is exactly what was known at the draw.
		l.restoreTurn()
	}
}

// reset starts a fresh deal.
func (l *Ledger) reset(deal int) {
	l.Deal = deal
	l.Discards = nil
	l.Held = map[string][]string{}
	l.HeldAtTurnStart = nil
}

func (l *Ledger) snapshotTurn() {
	l.HeldAtTurnStart = cloneHeld(l.Held)
}

func (l *Ledger) restoreTurn() {
	if l.HeldAtTurnStart != nil {
		l.Held = cloneHeld(l.HeldAtTurnStart)
	}
}

// take records cards seen entering a seat's hand.
func (l *Ledger) take(playerID string, cards []string) {
	for _, c := range cards {
		if c == "" {
			continue
		}
		l.Held[playerID] = append(l.Held[playerID], c)
	}
}

// drop records cards leaving a seat's hand in public — one copy per card, so a
// hand holding two black kings that plays one is still known to hold the
// other.
func (l *Ledger) drop(playerID string, cards []string) {
	if len(l.Held[playerID]) == 0 {
		return
	}
	rest := l.Held[playerID]
	for _, c := range cards {
		rest = removeCardsOnce(rest, []string{c})
	}
	if len(rest) == 0 {
		delete(l.Held, playerID)
		return
	}
	l.Held[playerID] = rest
}

func cloneHeld(in map[string][]string) map[string][]string {
	out := make(map[string][]string, len(in))
	for k, v := range in {
		out[k] = append([]string(nil), v...)
	}
	return out
}

// addedJokers is what a swap put into the hand: the multiset difference
// between the joker debt before and after.
func addedJokers(before, after []string) []string {
	return removeCardsOnce(append([]string(nil), after...), before)
}

// layOffCards is the cards a lay-off action puts down, whichever field the
// caller used to name them.
func layOffCards(a rules.Action) []string {
	if len(a.Cards) > 0 {
		return a.Cards
	}
	if a.Card != "" {
		return []string{a.Card}
	}
	return nil
}

// VisibleFor is the snapshot the agent is allowed to see.
//
// One function, used by the module that serves real matches and by the
// simulator that measures strength, so the bot people play against and the bot
// the benchmark scores are provably the same bot looking at provably the same
// board. It reads the engine's state directly rather than a translation of it;
// there is deliberately no second representation to get out of step.
//
// What it must never contain is a hand other than the viewer's own.
// TestAgentDoesNotPeek permutes the hidden hands and pins that every field
// here comes out identical.
func VisibleFor(gs rules.GameState, l Ledger, playerID string) VisibleState {
	counts := make(map[string]int, len(gs.Hands))
	for seat, h := range gs.Hands {
		counts[seat] = len(h) // a count, never the cards
	}
	return VisibleState{
		GameNumber:       gs.GameNumber,
		Round:            gs.Round,
		Phase:            string(gs.Phase),
		CurrentTurn:      gs.CurrentTurn,
		DiscardPile:      gs.DiscardPile,
		DealDiscards:     l.discardsFor(gs.GameNumber),
		KnownHeld:        l.heldFor(gs.GameNumber),
		HandCounts:       counts,
		DeckRemaining:    len(gs.DrawPile),
		DeckCount:        rules.DeckCountForPlayers(len(gs.TurnOrder)),
		Melds:            gs.Melds,
		MeldMeta:         gs.MeldMeta,
		RoundReqMet:      gs.RoundReqMet,
		TotalScores:      gs.TotalScores,
		Rules:            rules.ResolveConfig(gs.Rules),
		DiscardTakenCard: gs.DiscardTakenCard,
		PendingMeldCard:  gs.DiscardDrawnCardPendingMeld,
		PendingJokers:    gs.JokersReclaimedPendingMeld,
	}
}

// discardsFor and heldFor refuse to serve observations from a deal that has
// already ended. Belt and braces next to Observe's own reset: a ledger read
// back off a document written by an older build has no Deal recorded, and
// answering "here is last deal's history" would be worse than answering
// nothing.
func (l Ledger) discardsFor(deal int) []SeenDiscard {
	if l.Deal != deal {
		return nil
	}
	return l.Discards
}

func (l Ledger) heldFor(deal int) map[string][]string {
	if l.Deal != deal {
		return nil
	}
	return l.Held
}
