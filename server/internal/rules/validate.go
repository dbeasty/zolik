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
		return state, card, nil, nil

	case DrawFromDiscard:
		if state.DiscardDrawMinRound > 1 && state.Round < state.DiscardDrawMinRound {
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

	// Dry-run to validate minimum meld / round requirement before mutating.
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

	if PlayerMeetsRoundRequirement(sim, playerID) {
		if state.InitialMeldMinimum > 0 && !state.RoundReqMet[playerID] {
			if got := PlayerInitialMeldNaturalValue(sim, playerID); got < state.InitialMeldMinimum {
				return state, "", "", RulesError{
					Code: ErrMeldBelowMinimum,
					Message: fmt.Sprintf(
						"your melds are worth %d points so far, but this table requires %d+ to go down",
						got, state.InitialMeldMinimum,
					),
				}
			}
		}
	}

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

	if PlayerMeetsRoundRequirement(state, playerID) {
		state.RoundReqMet[playerID] = true
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

	return state, meldID, mv.Type, nil
}

func ValidateLayOff(state GameState, playerID string, meldID string, card string) (GameState, error) {
	if state.Status != StatusActive {
		return state, RulesError{Code: ErrGameNotActive}
	}
	if state.Phase != PhaseMeld {
		return state, RulesError{Code: ErrWrongPhase}
	}
	if state.CurrentTurn != playerID {
		return state, RulesError{Code: ErrNotYourTurn}
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
	if err := requireCardsInHand(state.Hands[playerID], []string{card}); err != nil {
		return state, err
	}

	owner, idx := findMeldByID(state, meldID)
	if owner == "" {
		return state, RulesError{Code: ErrInvalidMeld, Message: "that meld no longer exists on the table"}
	}

	cfg := effectiveRules(state)
	newMeld := append(append([]string(nil), state.Melds[owner][idx]...), card)
	mv, err := ValidateMeld(newMeld, cfg)
	if err != nil {
		return state, err
	}
	if LayOffBreaksCleanRun(cfg, state.GameNumber, state.Melds[owner], idx, card) {
		return state, RulesError{
			Code:    ErrBreaksCleanRun,
			Message: "that run has to stay joker-free — start a separate meld instead",
		}
	}

	state.Hands[playerID] = removeCards(state.Hands[playerID], []string{card})
	state.Melds[owner][idx] = OrderMeldForDisplay(newMeld, mv)
	if metas := state.MeldMeta[owner]; idx < len(metas) {
		metas[idx].Type = mv.Type
		metas[idx].WildCount = mv.WildCount
	}
	// Once a lay-off has happened this turn, the discard-pile pickup (if
	// any) can no longer be cleanly undone.
	state.DiscardDrawnCards = nil

	if !cfg.IsFinalDeal(state.GameNumber) && len(state.Hands[playerID]) == 0 {
		return state, RulesError{Code: ErrInvalidMeld, Message: "must discard your last card to go out"}
	}

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
