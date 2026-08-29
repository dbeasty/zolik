package rules

import (
	"fmt"
	"strings"
)

// ValidateDraw draws a card for playerID. targetCard is only meaningful for
// DrawFromDiscard under DiscardPickupAnyFromPile: it names which pile card to
// take, along with every card stacked above it (empty targetCard = top card,
// which is also all that DiscardPickupTopOnly ever allows).
func ValidateDraw(state GameState, playerID string, from DrawFrom, targetCard string) (GameState, string, *string, error) {
	if state.Status != StatusActive {
		return state, "", nil, RulesError{Code: ErrGameNotActive}
	}
	if state.Phase == PhaseSuspended || state.Status == StatusSuspended {
		return state, "", nil, RulesError{Code: ErrGameSuspended}
	}
	if state.Phase != PhaseDraw {
		return state, "", nil, RulesError{Code: ErrWrongPhase}
	}
	if state.CurrentTurn != playerID {
		return state, "", nil, RulesError{Code: ErrNotYourTurn}
	}
	cfg := effectiveRules(state)

	switch from {
	case DrawFromDeck:
		state, err := ensureDrawPile(state)
		if err != nil {
			return state, "", nil, err
		}
		if len(state.DrawPile) == 0 {
			return state, "", nil, RulesError{Code: ErrNoCardsLeft}
		}
		card := state.DrawPile[len(state.DrawPile)-1]
		state.DrawPile = state.DrawPile[:len(state.DrawPile)-1]
		state.Hands[playerID] = append(state.Hands[playerID], card)
		state.Phase = PhaseMeld
		state.MeldsLaidThisTurn = 0
		state.DiscardDrawnCardPendingMeld = ""
		state.DiscardTakenCard = ""
		state.DiscardDrawnCards = nil
		state.LastLayOff = nil
		state.LastMeldLaid = nil
		state.TurnMeldSnapshot = snapshotTurnMeld(state, playerID)
		return state, card, nil, nil

	case DrawFromDiscard:
		// The shared helper, fed from the resolved ruleset: the duplicate
		// GameState.DiscardDrawMinRound field it used to read no longer
		// exists — RulesConfig is the only home for a rule value.
		if IsDiscardLocked(state.Round, cfg.DiscardDrawMinRound) {
			return state, "", nil, RulesError{Code: ErrDiscardLocked}
		}
		if len(state.DiscardPile) == 0 {
			return state, "", nil, RulesError{Code: ErrDiscardPileEmpty}
		}

		takeFrom := len(state.DiscardPile) - 1 // index of the top card by default
		if cfg.DiscardPickupMode == DiscardPickupAnyFromPile && targetCard != "" {
			idx := -1
			for i, c := range state.DiscardPile {
				if c == targetCard {
					idx = i
					break
				}
			}
			if idx == -1 {
				return state, "", nil, RulesError{Code: ErrDiscardPileEmpty, Message: "requested card not in discard pile"}
			}
			takeFrom = idx
		}

		taken := append([]string(nil), state.DiscardPile[takeFrom:]...)
		state.DiscardPile = state.DiscardPile[:takeFrom]
		state.Hands[playerID] = append(state.Hands[playerID], taken...)
		state.Phase = PhaseMeld
		state.MeldsLaidThisTurn = 0
		state.DiscardDrawnCards = taken
		state.LastLayOff = nil
		state.LastMeldLaid = nil
		// Before a player has gone down, a discard-pile pickup obligates
		// them to lay the requested (bottom-most taken) card into their
		// initial meld this turn — see ValidateDiscard. Once already down
		// it's a free pickup.
		primary := taken[0]
		if !state.RoundReqMet[playerID] {
			state.DiscardDrawnCardPendingMeld = primary
		} else {
			state.DiscardDrawnCardPendingMeld = ""
		}
		// Tracked whether or not they're down: a down player owes nothing to
		// their initial meld, but still may not simply hand the card back
		// this turn — see GameState.DiscardTakenCard.
		state.DiscardTakenCard = primary
		state.TurnMeldSnapshot = snapshotTurnMeld(state, playerID)
		return state, primary, nil, nil
	default:
		return state, "", nil, fmt.Errorf("unknown draw source")
	}
}

// ValidateUndoDrawDiscard reverses a same-turn discard-pile pickup: it
// returns the taken card(s) to the top of the discard pile, in their
// original order, and puts the player back in the draw phase so they can
// draw again — from the deck, or a different card off the discard pile.
// Only available in the window right after the pickup, before any
// lay_meld/lay_off this turn has had a chance to use the drawn card(s).
func ValidateUndoDrawDiscard(state GameState, playerID string) (GameState, error) {
	if state.Status != StatusActive {
		return state, RulesError{Code: ErrGameNotActive}
	}
	if state.Phase == PhaseSuspended || state.Status == StatusSuspended {
		return state, RulesError{Code: ErrGameSuspended}
	}
	if state.CurrentTurn != playerID {
		return state, RulesError{Code: ErrNotYourTurn}
	}
	if state.Phase != PhaseMeld {
		return state, RulesError{Code: ErrWrongPhase}
	}
	if len(state.DiscardDrawnCards) == 0 {
		return state, RulesError{Code: ErrNothingToUndo}
	}
	if err := requireCardsInHand(state.Hands[playerID], state.DiscardDrawnCards); err != nil {
		return state, err
	}

	state.Hands[playerID] = removeCards(state.Hands[playerID], state.DiscardDrawnCards)
	state.DiscardPile = append(state.DiscardPile, state.DiscardDrawnCards...)
	state.DiscardDrawnCards = nil
	state.DiscardDrawnCardPendingMeld = ""
	state.DiscardTakenCard = ""
	state.MeldsLaidThisTurn = 0
	state.Phase = PhaseDraw

	return state, nil
}

func ValidateMeldAction(state GameState, playerID string, cards []string) (GameState, string, MeldType, error) {
	if state.Status != StatusActive {
		return state, "", "", RulesError{Code: ErrGameNotActive}
	}
	if state.Phase != PhaseMeld {
		// Distinguish the one case a player runs into constantly — melding
		// before drawing — from every other wrong phase, so the UI can say
		// what to do instead of only that something is unavailable.
		if state.Phase == PhaseDraw && state.CurrentTurn == playerID {
			return state, "", "", RulesError{Code: ErrMustDrawFirst}
		}
		return state, "", "", RulesError{Code: ErrWrongPhase}
	}
	if state.CurrentTurn != playerID {
		return state, "", "", RulesError{Code: ErrNotYourTurn}
	}

	if err := requireCardsInHand(state.Hands[playerID], cards); err != nil {
		return state, "", "", err
	}

	wasReqMet := state.RoundReqMet[playerID]
	prevMeldsLaidThisTurn := state.MeldsLaidThisTurn
	prevDiscardDrawnCardPendingMeld := state.DiscardDrawnCardPendingMeld
	prevDiscardTakenCard := state.DiscardTakenCard
	prevJokersReclaimed := append([]string(nil), state.JokersReclaimedPendingMeld...)
	cfg := effectiveRules(state)

	mv, err := ValidateMeld(cards, cfg)
	if err != nil {
		return state, "", "", err
	}
	// Store the meld in its sorted, canonical order (e.g. a run played as
	// 6-8-7 is stored — and so displayed — as 6-7-8) rather than whatever
	// order the cards were selected in.
	orderedCards := OrderMeldForDisplay(cards, mv)

	if !MeldContributesTowardRequirement(state, playerID, mv.Type, len(cards), mv.WildCount) {
		return state, "", "", RulesError{
			Code:    ErrMeldNoContribution,
			Message: "meld does not advance round requirement",
		}
	}

	// Dry-run only to check the "must discard your last card to go out"
	// rule below — the minimum-value floor is no longer checked here (see
	// the RoundReqMet assignment after the real apply, below).
	sim := cloneState(state)
	sim.Hands[playerID] = removeCards(sim.Hands[playerID], cards)
	if sim.Melds == nil {
		sim.Melds = map[string][][]string{}
	}
	if sim.MeldMeta == nil {
		sim.MeldMeta = map[string][]MeldInfo{}
	}
	sim.Melds[playerID] = append(sim.Melds[playerID], append([]string(nil), orderedCards...))
	nextID := sim.NextMeldSeq + 1
	meldID := fmt.Sprintf("meld_%d", nextID)
	sim.MeldMeta[playerID] = append(sim.MeldMeta[playerID], MeldInfo{
		MeldID: meldID, Type: mv.Type, OwnerID: playerID, WildCount: mv.WildCount,
	})

	// A profile's non-final deals require a final discard to go out; cannot
	// meld away every card (Žolík Classic never has a "final deal", so this
	// applies to every one of its deals — see RulesConfig.IsFinalDeal).
	if !cfg.IsFinalDeal(state.GameNumber) && len(sim.Hands[playerID]) == 0 {
		return state, "", "", RulesError{Code: ErrInvalidMeld, Message: "must discard your last card to go out"}
	}

	// Apply for real.
	state.Hands[playerID] = removeCards(state.Hands[playerID], cards)
	if state.Melds == nil {
		state.Melds = map[string][][]string{}
	}
	if state.MeldMeta == nil {
		state.MeldMeta = map[string][]MeldInfo{}
	}
	state.Melds[playerID] = append(state.Melds[playerID], append([]string(nil), orderedCards...))
	state.NextMeldSeq = nextID
	state.MeldMeta[playerID] = append(state.MeldMeta[playerID], MeldInfo{
		MeldID: meldID, Type: mv.Type, OwnerID: playerID, WildCount: mv.WildCount,
	})

	// A meld can always be laid onto the table (subject to the checks above);
	// whether it actually puts the player "down" depends on both the shape
	// requirement (sets/runs/clean run) and, if configured, the point-value
	// floor summed across every meld the player has laid so far — checked
	// together here, on the real post-apply state, so that a shape-complete
	// but under-value combination doesn't lock the player out of laying a
	// later meld to top up the total (e.g. two clean runs of 27 and 21
	// points: the first alone already satisfies "at least one clean run",
	// but neither meld alone clears a 35-point floor — only their sum does).
	refreshRoundReqMet(state, playerID)
	if !wasReqMet {
		state.MeldsLaidThisTurn++
	}
	if state.DiscardDrawnCardPendingMeld != "" {
		for _, c := range cards {
			if c == state.DiscardDrawnCardPendingMeld {
				state.DiscardDrawnCardPendingMeld = ""
				break
			}
		}
	}
	state.DiscardTakenCard = clearIfSpent(state.DiscardTakenCard, cards)
	// A reclaimed joker used in this brand-new meld pays off its
	// play-it-this-turn debt.
	state.JokersReclaimedPendingMeld = removeCards(state.JokersReclaimedPendingMeld, cards)
	// Once a meld has been laid this turn, the discard-pile pickup (if any)
	// can no longer be cleanly undone.
	state.DiscardDrawnCards = nil
	state.LastLayOff = nil
	state.LastMeldLaid = &MeldLaidSnapshot{
		PlayerID:                        playerID,
		MeldID:                          meldID,
		Cards:                           append([]string(nil), orderedCards...),
		PrevRoundReqMet:                 wasReqMet,
		PrevMeldsLaidThisTurn:           prevMeldsLaidThisTurn,
		PrevDiscardDrawnCardPendingMeld: prevDiscardDrawnCardPendingMeld,
		PrevDiscardTakenCard:            prevDiscardTakenCard,
		PrevJokersReclaimedPendingMeld:  prevJokersReclaimed,
	}

	return state, meldID, mv.Type, nil
}

// ValidateUndoLayMeld reverses the most recent brand-new lay_meld this turn:
// the meld comes off the table, its cards go back to the player's hand, and
// RoundReqMet/MeldsLaidThisTurn/DiscardDrawnCardPendingMeld all revert to
// exactly what they were beforehand. Only available in the window right
// after that lay_meld, before anything else this turn (a fresh draw, a
// lay_off, a swap_joker, or another lay_meld) has had a chance to build on
// top of it.
func ValidateUndoLayMeld(state GameState, playerID string) (GameState, error) {
	if state.Status != StatusActive {
		return state, RulesError{Code: ErrGameNotActive}
	}
	if state.Phase == PhaseSuspended || state.Status == StatusSuspended {
		return state, RulesError{Code: ErrGameSuspended}
	}
	if state.CurrentTurn != playerID {
		return state, RulesError{Code: ErrNotYourTurn}
	}
	if state.Phase != PhaseMeld {
		return state, RulesError{Code: ErrWrongPhase}
	}
	snap := state.LastMeldLaid
	if snap == nil || snap.PlayerID != playerID {
		return state, RulesError{Code: ErrNothingToUndo}
	}
	owner, idx := findMeldByID(state, snap.MeldID)
	if owner == "" {
		return state, RulesError{Code: ErrNothingToUndo, Message: "that meld no longer exists on the table"}
	}

	state.Hands[playerID] = append(append([]string(nil), state.Hands[playerID]...), snap.Cards...)
	state.Melds[owner] = append(append([][]string(nil), state.Melds[owner][:idx]...), state.Melds[owner][idx+1:]...)
	if metas := state.MeldMeta[owner]; idx < len(metas) {
		state.MeldMeta[owner] = append(append([]MeldInfo(nil), metas[:idx]...), metas[idx+1:]...)
	}
	state.RoundReqMet[playerID] = snap.PrevRoundReqMet
	state.MeldsLaidThisTurn = snap.PrevMeldsLaidThisTurn
	state.DiscardDrawnCardPendingMeld = snap.PrevDiscardDrawnCardPendingMeld
	state.DiscardTakenCard = snap.PrevDiscardTakenCard
	state.JokersReclaimedPendingMeld = append([]string(nil), snap.PrevJokersReclaimedPendingMeld...)
	state.LastMeldLaid = nil

	return state, nil
}

// snapshotTurnMeld deep-copies everything a meld phase can touch (hands,
// melds/meta of every player, the discard pile, and the current player's
// round-requirement/turn bookkeeping) so ValidateUndoTurn can restore it
// exactly, regardless of how many actions build on top of it before then.
func snapshotTurnMeld(state GameState, playerID string) *TurnMeldSnapshot {
	hands := map[string][]string{}
	for k, v := range state.Hands {
		hands[k] = append([]string(nil), v...)
	}
	melds := map[string][][]string{}
	for k, ms := range state.Melds {
		for _, m := range ms {
			melds[k] = append(melds[k], append([]string(nil), m...))
		}
	}
	meldMeta := map[string][]MeldInfo{}
	for k, metas := range state.MeldMeta {
		meldMeta[k] = append([]MeldInfo(nil), metas...)
	}
	roundReqMet := map[string]bool{}
	for k, v := range state.RoundReqMet {
		roundReqMet[k] = v
	}
	return &TurnMeldSnapshot{
		PlayerID:                    playerID,
		Hands:                       hands,
		Melds:                       melds,
		MeldMeta:                    meldMeta,
		RoundReqMet:                 state.RoundReqMet[playerID],
		AllRoundReqMet:              roundReqMet,
		MeldsLaidThisTurn:           state.MeldsLaidThisTurn,
		DiscardDrawnCardPendingMeld: state.DiscardDrawnCardPendingMeld,
		DiscardTakenCard:            state.DiscardTakenCard,
		DiscardDrawnCards:           append([]string(nil), state.DiscardDrawnCards...),
		// Not append([]string(nil), ...): appending zero elements to a nil
		// slice returns nil, not an empty slice — harmless everywhere else,
		// but DiscardPile round-trips to the client as JSON, where a nil
		// slice serializes to `null` instead of `[]`. The client indexes it
		// unconditionally (state.discardPile[state.discardPile.length - 1])
		// expecting an array, so restoring a nil discard pile via undo_turn
		// (e.g. right after the discard pile's last card was just picked
		// up) crashed the whole game screen. make() is never nil, even at
		// length 0.
		DiscardPile:                append(make([]string, 0, len(state.DiscardPile)), state.DiscardPile...),
		NextMeldSeq:                state.NextMeldSeq,
		JokersReclaimedPendingMeld: append([]string(nil), state.JokersReclaimedPendingMeld...),
	}
}

// ValidateUndoTurn reverts every meld/lay-off/joker-swap action taken since
// this player's draw, restoring hands, table melds, the discard pile, and
// round-requirement bookkeeping to exactly how they were right after the
// draw. Unlike ValidateUndoLayOff/ValidateUndoLayMeld (which only reach back
// one action), this is available any time during the meld phase, however
// many actions have piled up since — the snapshot itself only advances on
// the next draw, never on the actions being undone.
func ValidateUndoTurn(state GameState, playerID string) (GameState, error) {
	if state.Status != StatusActive {
		return state, RulesError{Code: ErrGameNotActive}
	}
	if state.Phase == PhaseSuspended || state.Status == StatusSuspended {
		return state, RulesError{Code: ErrGameSuspended}
	}
	if state.CurrentTurn != playerID {
		return state, RulesError{Code: ErrNotYourTurn}
	}
	if state.Phase != PhaseMeld {
		return state, RulesError{Code: ErrWrongPhase}
	}
	snap := state.TurnMeldSnapshot
	if snap == nil || snap.PlayerID != playerID {
		return state, RulesError{Code: ErrNothingToUndo}
	}

	state.Hands = snap.Hands
	state.Melds = snap.Melds
	state.MeldMeta = snap.MeldMeta
	// Every player's flag, not just the acting player's: a lay_off or joker
	// swap this turn may have put the meld's owner down. Falls back to the
	// acting player alone for a snapshot written before AllRoundReqMet
	// existed, which is exactly what that build would have restored.
	if len(snap.AllRoundReqMet) > 0 {
		for pid, met := range snap.AllRoundReqMet {
			state.RoundReqMet[pid] = met
		}
	} else {
		state.RoundReqMet[playerID] = snap.RoundReqMet
	}
	state.MeldsLaidThisTurn = snap.MeldsLaidThisTurn
	state.DiscardDrawnCardPendingMeld = snap.DiscardDrawnCardPendingMeld
	state.DiscardTakenCard = snap.DiscardTakenCard
	state.DiscardDrawnCards = snap.DiscardDrawnCards
	state.DiscardPile = snap.DiscardPile
	state.NextMeldSeq = snap.NextMeldSeq
	state.JokersReclaimedPendingMeld = append([]string(nil), snap.JokersReclaimedPendingMeld...)
	state.LastLayOff = nil
	state.LastMeldLaid = nil
	// Re-snapshot the just-restored state rather than reusing snap directly:
	// keeps TurnMeldSnapshot's map/slice fields independent copies so a
	// second undo_turn later this same turn can't alias state this undo just
	// wrote into the live GameState.
	state.TurnMeldSnapshot = snapshotTurnMeld(state, playerID)

	return state, nil
}

func ValidateLayOff(state GameState, playerID string, meldID string, cards []string, position string) (GameState, error) {
	if state.Status != StatusActive {
		return state, RulesError{Code: ErrGameNotActive}
	}
	if state.Phase != PhaseMeld {
		return state, RulesError{Code: ErrWrongPhase}
	}
	if state.CurrentTurn != playerID {
		return state, RulesError{Code: ErrNotYourTurn}
	}
	if len(cards) == 0 {
		return state, RulesError{Code: ErrInvalidMeld, Message: "no cards to lay off"}
	}
	// A player can only lay off (onto their own or another player's melds)
	// once they've laid their own initial meld — before that, any card they
	// draw must go toward completing their own combination, not extending
	// someone else's.
	if !state.RoundReqMet[playerID] {
		return state, notDownError(state, playerID)
	}
	if err := requireCardsInHand(state.Hands[playerID], cards); err != nil {
		return state, err
	}

	owner, idx := findMeldByID(state, meldID)
	if owner == "" {
		return state, RulesError{Code: ErrInvalidMeld, Message: "that meld no longer exists on the table"}
	}

	cfg := effectiveRules(state)
	prevCards := state.Melds[owner][idx]
	prevMeta := MeldInfo{}
	hasPrevMeta := false
	if metas := state.MeldMeta[owner]; idx < len(metas) {
		prevMeta = metas[idx]
		hasPrevMeta = true
	}

	// A natural card that takes an existing joker's exact place in this
	// meld swaps the joker out instead of piling in alongside it — the
	// same replace-and-revalidate ValidateSwapJoker does explicitly for a
	// single-card drop, generalized here so a multi-card lay-off (fully
	// supported by the client's multi-select drag) doesn't strand a joker
	// just because the matching naturals arrived together instead of one
	// at a time: once they're on the table, nothing can trigger a swap for
	// that joker again. Cards are tried in submission order against the
	// meld as it's reclaimed so far; whatever doesn't match an existing
	// joker is appended below, same as before this loop existed.
	working := append([]string(nil), prevCards...)
	var reclaimed []string
	var toAppend []string
	for _, c := range cards {
		if !IsJoker(c) {
			if swapped, joker, ok := reclaimJokerForLayOff(working, c, cfg, prevMeta, hasPrevMeta); ok {
				working = swapped
				reclaimed = append(reclaimed, joker)
				continue
			}
		}
		toAppend = append(toAppend, c)
	}
	newMeld := append(working, toAppend...)
	mv, err := ValidateMeld(newMeld, cfg)
	if err != nil {
		return state, err
	}
	// Position is a drag-and-drop UX hint: "front" or "end" names which
	// side of the run the player dropped the card(s) onto, so that side is
	// the one that actually has to grow. Without this, a card dropped on
	// the left edge of a run could silently resolve onto the right edge
	// instead (whichever placement uses fewer wilds) — deterministic to the
	// server, but not to a player who watched their card land somewhere
	// they didn't drop it. Sets have no directionality, so position is
	// ignored for them.
	if position == "front" || position == "end" {
		if mv.Type == MeldRun {
			// A card that could equally extend either end of the run (e.g.
			// a joker) resolves ambiguously by wild count alone. Re-resolve
			// preferring the end the player actually dropped on, so a
			// correct "end" drop doesn't get silently reinterpreted as
			// extending the front and then rejected below for "growing the
			// wrong end".
			minRun := cfg.MinRunSize
			if minRun == 0 {
				minRun = 4
			}
			if reMV, reErr := validateRun(newMeld, minRun, position == "end"); reErr == nil {
				mv = reMV
			}
			// Which end this submission actually grows is then resolved by
			// the shared helper LegalActions also uses to build its per-card
			// "front"/"end" drop hints — so the hint a client renders and the
			// check the server enforces can never disagree.
			if sides, known := runGrowthSides(prevCards, mv); known && !containsString(sides, position) {
				return state, RulesError{
					Code:    ErrWrongRunEnd,
					Message: "that card extends the other end of the run — try dropping it there instead",
				}
			}
		}
	}

	prevDiscardTakenCard := state.DiscardTakenCard
	prevJokersReclaimed := append([]string(nil), state.JokersReclaimedPendingMeld...)
	state.DiscardTakenCard = clearIfSpent(state.DiscardTakenCard, cards)
	state.Hands[playerID] = removeCards(state.Hands[playerID], cards)
	state.Hands[playerID] = append(state.Hands[playerID], reclaimed...)
	// One lay-off can move the play-it-this-turn joker debt both ways: a
	// joker among cards pays its debt off, and a joker swapped out of the
	// meld takes on a fresh one.
	state.JokersReclaimedPendingMeld = removeCards(state.JokersReclaimedPendingMeld, cards)
	state.JokersReclaimedPendingMeld = append(state.JokersReclaimedPendingMeld, reclaimed...)
	state.Melds[owner][idx] = OrderMeldForDisplay(newMeld, mv)
	if metas := state.MeldMeta[owner]; idx < len(metas) {
		metas[idx].Type = mv.Type
		metas[idx].WildCount = mv.WildCount
	}
	// The meld just grew. If it belongs to a player who was not down yet,
	// those extra cards can be what finally satisfies their contract or
	// lifts them over the point floor, so re-derive it rather than leaving
	// the flag as ValidateLayMeld last left it.
	prevOwnerReqMet := state.RoundReqMet[owner]
	refreshRoundReqMet(state, owner)

	// Once a lay-off has happened this turn, the discard-pile pickup (if
	// any) can no longer be cleanly undone.
	state.DiscardDrawnCards = nil
	state.LastMeldLaid = nil
	state.LastLayOff = &LayOffSnapshot{
		PlayerID:  playerID,
		MeldID:    meldID,
		PrevCards: append([]string(nil), prevCards...),
		PrevMeta:  prevMeta,
		Cards:     append([]string(nil), cards...),

		PrevDiscardTakenCard:           prevDiscardTakenCard,
		PrevOwnerReqMet:                prevOwnerReqMet,
		ReclaimedJokers:                append([]string(nil), reclaimed...),
		PrevJokersReclaimedPendingMeld: prevJokersReclaimed,
	}

	if !cfg.IsFinalDeal(state.GameNumber) && len(state.Hands[playerID]) == 0 {
		return state, RulesError{Code: ErrInvalidMeld, Message: "must discard your last card to go out"}
	}

	return state, nil
}

// ValidateUndoLayOff reverses the most recent lay_off this turn: the card(s)
// go back to the player's hand and the meld reverts to exactly what it held
// before. Only available in the window right after that lay_off, before
// anything else this turn (a fresh draw, a lay_meld, a swap_joker, or
// another lay_off) has had a chance to build on top of it.
func ValidateUndoLayOff(state GameState, playerID string) (GameState, error) {
	if state.Status != StatusActive {
		return state, RulesError{Code: ErrGameNotActive}
	}
	if state.Phase == PhaseSuspended || state.Status == StatusSuspended {
		return state, RulesError{Code: ErrGameSuspended}
	}
	if state.CurrentTurn != playerID {
		return state, RulesError{Code: ErrNotYourTurn}
	}
	if state.Phase != PhaseMeld {
		return state, RulesError{Code: ErrWrongPhase}
	}
	snap := state.LastLayOff
	if snap == nil || snap.PlayerID != playerID {
		return state, RulesError{Code: ErrNothingToUndo}
	}
	owner, idx := findMeldByID(state, snap.MeldID)
	if owner == "" {
		return state, RulesError{Code: ErrNothingToUndo, Message: "that meld no longer exists on the table"}
	}

	// Any joker the lay-off swapped out of the meld went into the hand
	// alongside the ordinary cards — remove it before adding those cards
	// back, since PrevCards below already has the joker back in the meld
	// and leaving it in hand too would duplicate it.
	state.Hands[playerID] = removeCards(state.Hands[playerID], snap.ReclaimedJokers)
	state.Hands[playerID] = append(append([]string(nil), state.Hands[playerID]...), snap.Cards...)
	state.DiscardTakenCard = snap.PrevDiscardTakenCard
	state.Melds[owner][idx] = append([]string(nil), snap.PrevCards...)
	if metas := state.MeldMeta[owner]; idx < len(metas) {
		metas[idx] = snap.PrevMeta
	}
	// The meld is back to its prior size, so the owner's down-status has to
	// go back with it — see LayOffSnapshot.PrevOwnerReqMet.
	if state.RoundReqMet != nil {
		state.RoundReqMet[owner] = snap.PrevOwnerReqMet
	}
	state.JokersReclaimedPendingMeld = append([]string(nil), snap.PrevJokersReclaimedPendingMeld...)
	state.LastLayOff = nil

	return state, nil
}

// reclaimJokerForLayOff tries to remove the first joker in meld and put card
// in its place — the same substitution ValidateSwapJoker performs for an
// explicit single-card swap, reused here so a multi-card lay-off can reclaim
// a joker per matching card instead of only ever appending. ok is false if
// meld holds no joker, or the resulting meld isn't a valid meld of the same
// type meta records (hasMeta false skips that check, matching
// ValidateSwapJoker's own behavior when the meld has no meta entry yet).
func reclaimJokerForLayOff(meld []string, card string, cfg RulesConfig, meta MeldInfo, hasMeta bool) (newMeld []string, joker string, ok bool) {
	jokerPos := -1
	for i, c := range meld {
		if IsJoker(c) {
			jokerPos = i
			break
		}
	}
	if jokerPos == -1 {
		return nil, "", false
	}
	joker = meld[jokerPos]

	replaced := append(append([]string(nil), meld[:jokerPos]...), meld[jokerPos+1:]...)
	replaced = append(replaced, card)

	mv, err := ValidateMeld(replaced, cfg)
	if err != nil {
		return nil, "", false
	}
	if hasMeta && mv.Type != meta.Type {
		return nil, "", false
	}
	return replaced, joker, true
}

// ValidateSwapJoker replaces a joker sitting in an existing meld with the
// exact natural card it stands in for, moving the joker into playerID's
// hand. Applies to both runs (the joker fills a specific rank/suit gap) and
// sets (the joker fills a specific rank, any suit not already present) —
// re-validating the meld with the joker removed and the natural card added
// is enough to confirm the natural card genuinely occupies the joker's slot,
// since any other placement would either duplicate a rank/suit already on
// the table or fail to form a contiguous run.
func ValidateSwapJoker(state GameState, playerID string, meldID string, card string) (GameState, error) {
	if state.Status != StatusActive {
		return state, RulesError{Code: ErrGameNotActive}
	}
	if state.Phase != PhaseMeld {
		return state, RulesError{Code: ErrWrongPhase}
	}
	if state.CurrentTurn != playerID {
		return state, RulesError{Code: ErrNotYourTurn}
	}
	if IsJoker(card) {
		return state, RulesError{Code: ErrJokerSwapMismatch, Message: "the replacement card must be a natural card, not a joker"}
	}
	if err := requireCardsInHand(state.Hands[playerID], []string{card}); err != nil {
		return state, err
	}

	owner, idx := findMeldByID(state, meldID)
	if owner == "" {
		return state, RulesError{Code: ErrInvalidMeld, Message: "that meld no longer exists on the table"}
	}
	meld := state.Melds[owner][idx]

	jokerPos := -1
	for i, c := range meld {
		if IsJoker(c) {
			jokerPos = i
			break
		}
	}
	if jokerPos == -1 {
		return state, RulesError{Code: ErrNoJokerInMeld, Message: "that meld has no joker to swap out"}
	}
	joker := meld[jokerPos]

	replaced := append(append([]string(nil), meld[:jokerPos]...), meld[jokerPos+1:]...)
	replaced = append(replaced, card)

	cfg := effectiveRules(state)
	mv, err := ValidateMeld(replaced, cfg)
	if err != nil {
		return state, RulesError{
			Code:    ErrJokerSwapMismatch,
			Message: "that card doesn't take the joker's exact place in this meld",
		}
	}
	if meta := state.MeldMeta[owner]; idx < len(meta) && mv.Type != meta[idx].Type {
		return state, RulesError{
			Code:    ErrJokerSwapMismatch,
			Message: "that card doesn't take the joker's exact place in this meld",
		}
	}

	// Buying a joker back is a table move, reserved — like lay_off — for a
	// player who has laid their own initial meld. The one exception is the
	// swap that IS the coming out: swapping the joker out of your own run
	// can be exactly what makes it the clean run the contract demands, or
	// what lifts your melds over the point floor, so the gate asks whether
	// the player is down *after* the swap rather than before it. Checked on
	// a throwaway clone: a validator must not touch the caller's maps on a
	// refusal.
	if !state.RoundReqMet[playerID] {
		sim := cloneState(state)
		sim.Melds[owner][idx] = replaced
		if metas := sim.MeldMeta[owner]; idx < len(metas) {
			metas[idx].Type = mv.Type
			metas[idx].WildCount = mv.WildCount
		}
		refreshRoundReqMet(sim, playerID)
		if !sim.RoundReqMet[playerID] {
			return state, notDownError(state, playerID)
		}
	}

	state.DiscardTakenCard = clearIfSpent(state.DiscardTakenCard, []string{card})
	state.Hands[playerID] = removeCards(state.Hands[playerID], []string{card})
	state.Hands[playerID] = append(state.Hands[playerID], joker)
	// The joker arrives owing a meld: under JokerReclaimMustPlay the turn
	// cannot end until it is played again (see ValidateDiscard).
	state.JokersReclaimedPendingMeld = append(state.JokersReclaimedPendingMeld, joker)
	state.Melds[owner][idx] = OrderMeldForDisplay(replaced, mv)
	if metas := state.MeldMeta[owner]; idx < len(metas) {
		metas[idx].Type = mv.Type
		metas[idx].WildCount = mv.WildCount
	}
	// Taking a joker out of a run can turn it into the joker-free run the
	// owner's contract requires, so the same re-derivation a lay-off needs
	// applies here too.
	refreshRoundReqMet(state, owner)
	// Swapping a joker never changes hand size, so it can never be the move
	// that empties a hand — unlike lay_meld/lay_off, no go-out check here.
	state.DiscardDrawnCards = nil
	state.LastLayOff = nil
	state.LastMeldLaid = nil

	return state, nil
}

// jokerDiscardIsOnlyMove reports whether playerID's hand is nothing but
// jokers with nowhere to go: no non-joker card to discard instead, and no
// lay-off possible for any of them (which, before the player has gone down,
// includes every meld — lay-off is refused outright until then).
//
// A new meld is never in question here: validateSet demands at least one
// natural to name the rank, so an all-joker hand can never lay one down.
func jokerDiscardIsOnlyMove(state GameState, playerID string) bool {
	hand := state.Hands[playerID]
	for _, c := range hand {
		if !IsJoker(c) {
			return false
		}
	}
	if !state.RoundReqMet[playerID] {
		return true
	}
	for _, m := range tableMelds(state) {
		if ok, _ := probe(state, playerID, Action{Type: ActionLayOff, MeldID: m.MeldID, Card: hand[0]}); ok {
			return false
		}
	}
	return true
}

func ValidateDiscard(state GameState, playerID string, card string, cardIndex *int) (GameState, bool, error) {
	if state.Status != StatusActive {
		return state, false, RulesError{Code: ErrGameNotActive}
	}
	if state.CurrentTurn != playerID {
		return state, false, RulesError{Code: ErrNotYourTurn}
	}
	if state.Phase != PhaseDiscard && state.Phase != PhaseMeld {
		return state, false, RulesError{Code: ErrWrongPhase, Message: "not in discard-allowed phase"}
	}
	if err := requireCardsInHand(state.Hands[playerID], []string{card}); err != nil {
		return state, false, err
	}
	cfg := effectiveRules(state)
	if cfg.JokerDiscardRestricted && IsJoker(card) {
		// A joker can never be discarded — except as the exact card that
		// empties an already-down player's hand, ending the deal for them, or
		// as the player's genuinely only legal move. An all-joker hand can
		// never form a new meld (a set needs a natural to name its rank), so
		// once every table meld is either full or out of reach, refusing this
		// discard would leave the player with nothing they can legally do for
		// the rest of the deal — the dead end dont-let-a-turn-end-in-a-dead-
		// position exists to avoid.
		goesOut := len(state.Hands[playerID]) == 1 && state.RoundReqMet[playerID]
		if !goesOut && !jokerDiscardIsOnlyMove(state, playerID) {
			return state, false, RulesError{
				Code:    ErrJokerDiscard,
				Message: "a joker can't be discarded unless it's the card that ends the hand",
			}
		}
	}
	// A player who started laying melds toward their (still unmet) initial
	// requirement this turn must finish it before ending their turn — no
	// leaving a partial lay-down on the table across turns.
	//
	// This used to be gated on `cfg.FixedDealCount > 0`, exempting Žolík
	// Classic, on the reasoning that its clean-run requirement can only ever
	// be met by a clean run and so a player who laid a set without holding
	// one would be unable to ever discard again. The exemption is worse than
	// the thing it avoided: it is exactly how a player ends up with melds on
	// the table, not down, and locked out of laying off for the rest of the
	// deal with no indication why. Reported from a real game — three sets
	// laid across three turns, every lay-off refused, and nothing on screen
	// saying a run was what was missing.
	//
	// It is not a dead end, because a lay-down that cannot be completed can
	// be taken back: `undo:lay_meld` returns the meld just laid, and
	// `undo:turn` returns the whole turn including every meld laid in it.
	// The rule is "finish it or take it back", which is the discipline a
	// partial lay-down needs in either profile.
	if state.MeldsLaidThisTurn > 0 && !state.RoundReqMet[playerID] {
		return state, false, RulesError{
			Code:    ErrIncompleteInitialMeld,
			Message: incompleteMeldMessage(state, playerID),
		}
	}
	// A joker taken off the table this turn owes a meld before the turn can
	// end — hoarding it in hand across the discard is what the take-and-
	// replay rule forbids. The one discard exempt is the one that empties an
	// already-down hand: it ends the deal, the same carve-out the joker
	// discard ban makes above. This gate is never a dead end — the take can
	// always be walked back instead, by undo:lay_off in the window right
	// after a lay-off reclaim, or by undo:turn at any point before the
	// discard.
	if cfg.JokerReclaimMustPlay && len(state.JokersReclaimedPendingMeld) > 0 {
		goesOut := len(state.Hands[playerID]) == 1 && state.RoundReqMet[playerID]
		if !goesOut {
			return state, false, RulesError{
				Code:    ErrReclaimedJokerNotMelded,
				Message: "the joker you took off the table must be played into a meld this turn — play it, or undo taking it",
			}
		}
	}
	// A discard-pile pickup made before going down obligates the player to
	// lay that card into their initial meld this turn.
	if state.DiscardDrawnCardPendingMeld != "" && !state.RoundReqMet[playerID] {
		return state, false, RulesError{
			Code:    ErrDiscardCardNotMelded,
			Message: "the card you picked up from the discard pile must go into your initial meld before you can discard",
		}
	}
	// The card taken off the discard pile this turn can't simply go back on
	// it. For a player who isn't down the gate above already covers this
	// (that card owes the initial meld), but a down player's pickup carries
	// no obligation, and returning it unused undoes the entire turn: the
	// pile, the hand and the deck all end up exactly as they started, so a
	// table can pass one card round in a circle indefinitely while the deck
	// still holds most of its cards. Taking a card commits you to it for
	// the turn — play it or keep it.
	if state.DiscardTakenCard != "" && card == state.DiscardTakenCard &&
		hasOtherDiscardableCard(state.Hands[playerID], state.DiscardTakenCard, cfg) {
		return state, false, RulesError{
			Code:    ErrDiscardTakenCard,
			Message: "you can't discard the card you just took from the discard pile — play it or keep it this turn",
		}
	}

	state.Hands[playerID] = removeCardAt(state.Hands[playerID], card, cardIndex)
	state.DiscardPile = append(state.DiscardPile, card)
	// The marker is scoped to the turn that took the card, and this discard
	// ends that turn.
	state.DiscardTakenCard = ""
	// So is the joker debt: either the gate above found it paid, or it was
	// never owed (rule off, or the going-out carve-out) — a new turn starts
	// with a clean slate either way.
	state.JokersReclaimedPendingMeld = nil
	// The meld phase is over now that the discard's landed — the whole-turn
	// undo only makes sense before that point.
	state.TurnMeldSnapshot = nil

	// Go-out check: if hand empty after discard, must have met the game requirement.
	if len(state.Hands[playerID]) == 0 {
		if !state.RoundReqMet[playerID] {
			return state, false, RulesError{
				Code:    ErrRoundReqNotMet,
				Message: "that would empty your hand, but you still haven't met this round's meld requirement — lay it down first",
			}
		}
		return state, true, nil
	}

	// Pass the turn to the next player, who draws from the deck or the
	// discard pile like any other turn. If play comes back around to
	// whoever started this deal, a full lap of the table has completed.
	next := nextPlayer(state.TurnOrder, playerID)
	if next == state.DealStarterID {
		state.Round++
	}
	state.Phase = PhaseDraw
	state.CurrentTurn = next

	return state, false, nil
}

func ensureDrawPile(state GameState) (GameState, error) {
	if len(state.DrawPile) > 0 {
		return state, nil
	}
	if len(state.DiscardPile) == 0 {
		return state, RulesError{Code: ErrNoCardsLeft}
	}
	// Reshuffle: entire discard pile becomes new draw pile (shuffled), discard emptied.
	pile := append([]string(nil), state.DiscardPile...)
	state.DiscardPile = nil
	state.ReshuffleCount++
	seed := state.DeckSeed + int64(state.ReshuffleCount)*7919
	if seed == 0 {
		seed = int64(state.ReshuffleCount) + 1
	}
	state.DrawPile = Shuffle(pile, seed)
	return state, nil
}

func requireCardsInHand(hand []string, want []string) error {
	counts := map[string]int{}
	for _, c := range hand {
		counts[c]++
	}
	for _, w := range want {
		if counts[w] <= 0 {
			return RulesError{Code: ErrCardNotInHand}
		}
		counts[w]--
	}
	return nil
}

// removeCardAt removes a single named card from hand. When idx points at a
// slot that actually holds that value, that exact slot is removed — needed
// because two decks are in play, so a hand can hold two physical cards with
// the same value and only the caller (who knows which one the player picked)
// can tell them apart. A nil/out-of-range/mismatched idx falls back to
// removing the first matching value, same as the old value-only behavior.
func removeCardAt(hand []string, card string, idx *int) []string {
	if idx != nil && *idx >= 0 && *idx < len(hand) && hand[*idx] == card {
		out := make([]string, 0, len(hand)-1)
		out = append(out, hand[:*idx]...)
		out = append(out, hand[*idx+1:]...)
		return out
	}
	return removeCards(hand, []string{card})
}

// hasOtherDiscardableCard reports whether the hand holds some card, other
// than the one just taken off the discard pile, that the rules would let its
// owner discard. It guards the taken-card ban so the ban can only ever
// remove a *choice*, never the last legal move: a player who laid off down
// to nothing but the card they took (or to that card plus undiscardable
// jokers) is allowed to shed it and end their turn rather than sit wedged
// with the whole table stuck behind them. Handing back the only card you
// hold is not the null move the ban is aimed at, either — the turn still
// shrinks the hand that reached it.
func hasOtherDiscardableCard(hand []string, taken string, cfg RulesConfig) bool {
	for _, c := range hand {
		if c == taken {
			continue
		}
		if cfg.JokerDiscardRestricted && IsJoker(c) {
			continue
		}
		return true
	}
	return false
}

// clearIfSpent blanks a single-card marker once that card turns up among
// cards that have just left the hand for the table. Value comparison is
// enough even though two decks put duplicates in play: if the marked card
// and its twin are interchangeable in hand, then whichever one went to the
// table, the hand that remains is identical either way.
func clearIfSpent(marker string, spent []string) string {
	if marker == "" {
		return ""
	}
	for _, c := range spent {
		if c == marker {
			return ""
		}
	}
	return marker
}

func removeCards(hand []string, remove []string) []string {
	counts := map[string]int{}
	for _, c := range remove {
		counts[c]++
	}
	out := make([]string, 0, len(hand))
	for _, c := range hand {
		if counts[c] > 0 {
			counts[c]--
			continue
		}
		out = append(out, c)
	}
	return out
}

func nextPlayer(order []string, current string) string {
	if len(order) == 0 {
		return ""
	}
	for i, pid := range order {
		if pid == current {
			return order[(i+1)%len(order)]
		}
	}
	return order[0]
}

// notDownError says why playerID is not down yet, distinguishing the two
// quite different reasons a player can be short.
//
// Telling a player who has two sets and a clean run on the table to "lay your
// own initial meld first" is simply false, and it is the message that used to
// come back when what they were actually short of was the initial-meld point
// floor — leaving them re-reading a table that already showed everything the
// message asked for. MELD_BELOW_MINIMUM names the real obstacle and carries
// how far short they are.
func notDownError(state GameState, playerID string) RulesError {
	cfg := effectiveRules(state)
	// Only once something is actually on the table: a player who has laid
	// nothing is short of the initial meld itself, whatever the arithmetic
	// says, and "lay it down first" is the honest instruction there.
	if len(state.Melds[playerID]) > 0 &&
		cfg.InitialMeldMinimum > 0 && PlayerMeetsRoundRequirement(state, playerID) {
		have := PlayerInitialMeldNaturalValue(state, playerID)
		if short := cfg.InitialMeldMinimum - have; short > 0 {
			return RulesError{
				Code: ErrMeldBelowMinimum,
				Message: fmt.Sprintf(
					"your melds are worth %d points, %d short of the %d needed to go down",
					have, short, cfg.InitialMeldMinimum),
			}
		}
	}
	// Melds on the table and still not down: "lay your initial meld first" is
	// then plainly false — the player is looking at the melds they laid — and
	// reads as a broken feature rather than an unmet rule. Say which part is
	// missing instead. The clean run is called out on its own because it is
	// the one requirement no amount of further melding can satisfy: only a
	// joker-free run will do, so a player told merely to "lay more" can go on
	// laying sets forever without ever getting there.
	if len(state.Melds[playerID]) > 0 {
		req := cfg.ContractFor(state.GameNumber)
		sets, runs, hasCleanRun := PlayerMeldCounts(state, playerID)
		if req.RequireCleanRun && !hasCleanRun && sets >= req.Sets && runs >= req.Runs {
			return RulesError{
				Code:    ErrNeedCleanRun,
				Message: "you need a joker-free run on the table before you count as down",
			}
		}
	}
	return RulesError{
		Code:    ErrRoundReqNotMet,
		Message: "lay your own initial meld before laying off on any meld",
	}
}

// incompleteMeldMessage explains what an unfinished lay-down is still short
// of, so the refusal to discard names the way out rather than restating that
// there is a rule.
func incompleteMeldMessage(state GameState, playerID string) string {
	cfg := effectiveRules(state)
	req := cfg.ContractFor(state.GameNumber)
	sets, runs, hasCleanRun := PlayerMeldCounts(state, playerID)

	var missing []string
	if short := req.Sets - sets; short > 0 {
		missing = append(missing, fmt.Sprintf("%d more set(s)", short))
	}
	if short := req.Runs - runs; short > 0 {
		missing = append(missing, fmt.Sprintf("%d more run(s)", short))
	}
	if req.RequireCleanRun && !hasCleanRun {
		missing = append(missing, "a joker-free run")
	}
	if cfg.InitialMeldMinimum > 0 {
		if short := cfg.InitialMeldMinimum - PlayerInitialMeldNaturalValue(state, playerID); short > 0 {
			missing = append(missing, fmt.Sprintf("%d more points", short))
		}
	}
	if len(missing) == 0 {
		return "finish your initial meld before you can discard"
	}
	return "your lay-down still needs " + strings.Join(missing, " and ") +
		" — complete it, or take it back with undo, before you can discard"
}

func findMeldByID(state GameState, meldID string) (owner string, idx int) {
	for oid, metas := range state.MeldMeta {
		for i, mi := range metas {
			if mi.MeldID == meldID {
				return oid, i
			}
		}
	}
	return "", -1
}
