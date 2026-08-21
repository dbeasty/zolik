package rules

import "fmt"

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
		state.DiscardDrawnCards = nil
		state.LastLayOff = nil
		state.LastMeldLaid = nil
		state.TurnMeldSnapshot = snapshotTurnMeld(state, playerID)
		return state, card, nil, nil

	case DrawFromDiscard:
		if cfg.DiscardDrawMinRound > 1 && state.Round < cfg.DiscardDrawMinRound {
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
	state.MeldsLaidThisTurn = 0
	state.Phase = PhaseDraw

	return state, nil
}

func ValidateMeldAction(state GameState, playerID string, cards []string) (GameState, string, MeldType, error) {
	if state.Status != StatusActive {
		return state, "", "", RulesError{Code: ErrGameNotActive}
	}
	if state.Phase != PhaseMeld {
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
	if PlayerMeetsRoundRequirement(state, playerID) {
		if cfg.InitialMeldMinimum <= 0 || PlayerInitialMeldNaturalValue(state, playerID) >= cfg.InitialMeldMinimum {
			state.RoundReqMet[playerID] = true
		}
	}
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
	return &TurnMeldSnapshot{
		PlayerID:                    playerID,
		Hands:                       hands,
		Melds:                       melds,
		MeldMeta:                    meldMeta,
		RoundReqMet:                 state.RoundReqMet[playerID],
		MeldsLaidThisTurn:           state.MeldsLaidThisTurn,
		DiscardDrawnCardPendingMeld: state.DiscardDrawnCardPendingMeld,
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
		DiscardPile: append(make([]string, 0, len(state.DiscardPile)), state.DiscardPile...),
		NextMeldSeq: state.NextMeldSeq,
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
	state.RoundReqMet[playerID] = snap.RoundReqMet
	state.MeldsLaidThisTurn = snap.MeldsLaidThisTurn
	state.DiscardDrawnCardPendingMeld = snap.DiscardDrawnCardPendingMeld
	state.DiscardDrawnCards = snap.DiscardDrawnCards
	state.DiscardPile = snap.DiscardPile
	state.NextMeldSeq = snap.NextMeldSeq
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
		return state, RulesError{
			Code:    ErrRoundReqNotMet,
			Message: "lay your own initial meld before laying off on any meld",
		}
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
	newMeld := append(append([]string(nil), prevCards...), cards...)
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
			// Which end this submission actually grows is resolved by the
			// shared helper LegalActions also uses to build its per-card
			// "front"/"end" drop hints — so the hint a client renders and
			// the check the server enforces can never disagree.
			if sides, known := runGrowthSides(prevCards, mv); known && !containsString(sides, position) {
				return state, RulesError{
					Code:    ErrWrongRunEnd,
					Message: "that card extends the other end of the run — try dropping it there instead",
				}
			}
		}
	}
	if LayOffBreaksCleanRun(cfg, state.GameNumber, state.Melds[owner], idx, cards) {
		return state, RulesError{
			Code:    ErrBreaksCleanRun,
			Message: "that run has to stay joker-free — start a separate meld instead",
		}
	}

	prevMeta := MeldInfo{}
	if metas := state.MeldMeta[owner]; idx < len(metas) {
		prevMeta = metas[idx]
	}

	state.Hands[playerID] = removeCards(state.Hands[playerID], cards)
	state.Melds[owner][idx] = OrderMeldForDisplay(newMeld, mv)
	if metas := state.MeldMeta[owner]; idx < len(metas) {
		metas[idx].Type = mv.Type
		metas[idx].WildCount = mv.WildCount
	}
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

	state.Hands[playerID] = append(append([]string(nil), state.Hands[playerID]...), snap.Cards...)
	state.Melds[owner][idx] = append([]string(nil), snap.PrevCards...)
	if metas := state.MeldMeta[owner]; idx < len(metas) {
		metas[idx] = snap.PrevMeta
	}
	state.LastLayOff = nil

	return state, nil
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

	state.Hands[playerID] = removeCards(state.Hands[playerID], []string{card})
	state.Hands[playerID] = append(state.Hands[playerID], joker)
	state.Melds[owner][idx] = OrderMeldForDisplay(replaced, mv)
	if metas := state.MeldMeta[owner]; idx < len(metas) {
		metas[idx].Type = mv.Type
		metas[idx].WildCount = mv.WildCount
	}
	// Swapping a joker never changes hand size, so it can never be the move
	// that empties a hand — unlike lay_meld/lay_off, no go-out check here.
	state.DiscardDrawnCards = nil
	state.LastLayOff = nil
	state.LastMeldLaid = nil

	return state, nil
}

func ValidateDiscard(state GameState, playerID string, card string) (GameState, bool, error) {
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
		// empties an already-down player's hand, ending the deal for them.
		goesOut := len(state.Hands[playerID]) == 1 && state.RoundReqMet[playerID]
		if !goesOut {
			return state, false, RulesError{
				Code:    ErrJokerDiscard,
				Message: "a joker can't be discarded unless it's the card that ends the hand",
			}
		}
	}
	// A player who started laying melds toward their (still unmet) initial
	// game requirement this turn must finish it before ending their turn —
	// no leaving a lone partial meld on the table across turns. Only
	// meaningful under a rotating/quota contract (Continental: e.g. "lay
	// both required sets, not just one"), where a meld you've already laid
	// is inherently partial progress toward a fixed count. A non-rotating
	// profile (Žolík Classic) has no such count — every lay_meld is already
	// a complete, standalone meld, and its clean-run requirement can only
	// ever be met by a clean run specifically, never by laying more sets —
	// so this block would leave a player who laid a set (but holds no
	// clean run this turn) unable to ever discard again.
	if cfg.FixedDealCount > 0 && state.MeldsLaidThisTurn > 0 && !state.RoundReqMet[playerID] {
		return state, false, RulesError{
			Code:    ErrIncompleteInitialMeld,
			Message: "finish laying your full initial meld combination (all required sets/runs, total value met) before you can discard",
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

	state.Hands[playerID] = removeCards(state.Hands[playerID], []string{card})
	state.DiscardPile = append(state.DiscardPile, card)
	// The meld phase is over now that the discard's landed — the whole-turn
	// undo only makes sense before that point.
	state.TurnMeldSnapshot = nil

	// Go-out check: if hand empty after discard, must have met the game requirement.
	if len(state.Hands[playerID]) == 0 {
		if !state.RoundReqMet[playerID] {
			return state, false, RulesError{Code: ErrRoundReqNotMet}
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
