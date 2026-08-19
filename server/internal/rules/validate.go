package rules

import "fmt"

func ValidateDraw(state GameState, playerID string, from DrawFrom) (GameState, string, *string, error) {
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
	if state.Offer != nil {
		return state, "", nil, RulesError{Code: ErrWrongPhase, Message: "offer active"}
	}

	switch from {
	case DrawFromDeck:
		if state.DeckDrawMinRound > 1 && state.Round < state.DeckDrawMinRound {
			// Deadlock safety: if discard draw isn't actually available right
			// now either (empty pile or itself locked), let the deck through
			// regardless — a turn must always have a legal way to draw.
			if discardDrawAvailable(state) {
				return state, "", nil, RulesError{Code: ErrDeckLocked}
			}
		}
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
		return state, card, nil, nil

	case DrawFromDiscard:
		if state.DiscardDrawMinRound > 1 && state.Round < state.DiscardDrawMinRound {
			// Same deadlock safety as the deck lock, mirrored: if the deck
			// truly can't be drawn from either, let discard through.
			if state.DeckDrawMinRound <= 1 || state.Round >= state.DeckDrawMinRound {
				return state, "", nil, RulesError{Code: ErrDiscardLocked}
			}
		}
		if len(state.DiscardPile) == 0 {
			return state, "", nil, RulesError{Code: ErrDiscardPileEmpty}
		}
		card := state.DiscardPile[len(state.DiscardPile)-1]
		state.DiscardPile = state.DiscardPile[:len(state.DiscardPile)-1]
		state.Hands[playerID] = append(state.Hands[playerID], card)
		state.Phase = PhaseMeld
		state.MeldsLaidThisTurn = 0
		return state, card, nil, nil
	default:
		return state, "", nil, fmt.Errorf("unknown draw source")
	}
}

func ValidateAcceptOffer(state GameState, playerID string) (GameState, string, string, error) {
	if state.Status != StatusActive {
		return state, "", "", RulesError{Code: ErrGameNotActive}
	}
	if state.Phase != PhaseOffer {
		return state, "", "", RulesError{Code: ErrWrongPhase}
	}
	if state.Offer == nil {
		return state, "", "", RulesError{Code: ErrNoActiveOffer}
	}
	if state.Offer.OfferedTo != playerID {
		return state, "", "", RulesError{Code: ErrNotOfferRecipient}
	}

	offeredCard := state.Offer.Card
	state.Hands[playerID] = append(state.Hands[playerID], offeredCard)

	// penalty draw from deck
	state, err := ensureDrawPile(state)
	if err != nil {
		return state, "", "", err
	}
	if len(state.DrawPile) == 0 {
		return state, "", "", RulesError{Code: ErrNoCardsLeft}
	}
	penalty := state.DrawPile[len(state.DrawPile)-1]
	state.DrawPile = state.DrawPile[:len(state.DrawPile)-1]
	state.Hands[playerID] = append(state.Hands[playerID], penalty)

	state.Offer = nil
	state.CurrentTurn = playerID
	state.Phase = PhaseMeld
	state.MeldsLaidThisTurn = 0

	return state, offeredCard, penalty, nil
}

func ValidateDeclineOffer(state GameState, playerID string) (GameState, error) {
	if state.Status != StatusActive {
		return state, RulesError{Code: ErrGameNotActive}
	}
	if state.Phase != PhaseOffer {
		return state, RulesError{Code: ErrWrongPhase}
	}
	if state.Offer == nil {
		return state, RulesError{Code: ErrNoActiveOffer}
	}
	if state.Offer.OfferedTo != playerID {
		return state, RulesError{Code: ErrNotOfferRecipient}
	}

	state.Offer = nil
	state.CurrentTurn = playerID
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

	mv, err := ValidateMeld(cards)
	if err != nil {
		return state, "", "", err
	}

	if !MeldContributesTowardRequirement(state, playerID, mv.Type, len(cards)) {
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
	sim.Melds[playerID] = append(sim.Melds[playerID], append([]string(nil), cards...))
	nextID := sim.NextMeldSeq + 1
	meldID := fmt.Sprintf("meld_%d", nextID)
	sim.MeldMeta[playerID] = append(sim.MeldMeta[playerID], MeldInfo{
		MeldID: meldID, Type: mv.Type, OwnerID: playerID,
	})

	if PlayerMeetsRoundRequirement(sim, playerID) {
		if state.InitialMeldMinimum > 0 && !state.RoundReqMet[playerID] {
			if PlayerInitialMeldNaturalValue(sim, playerID) < state.InitialMeldMinimum {
				return state, "", "", RulesError{Code: ErrMeldBelowMinimum}
			}
		}
	}

	// Rounds 1–6 require a final discard to go out; cannot meld away every card.
	if state.Round < 7 && len(sim.Hands[playerID]) == 0 {
		return state, "", "", RulesError{Code: ErrInvalidMeld, Message: "must discard last card to go out before round 7"}
	}

	// Apply for real.
	state.Hands[playerID] = removeCards(state.Hands[playerID], cards)
	if state.Melds == nil {
		state.Melds = map[string][][]string{}
	}
	if state.MeldMeta == nil {
		state.MeldMeta = map[string][]MeldInfo{}
	}
	state.Melds[playerID] = append(state.Melds[playerID], append([]string(nil), cards...))
	state.NextMeldSeq = nextID
	state.MeldMeta[playerID] = append(state.MeldMeta[playerID], MeldInfo{
		MeldID: meldID, Type: mv.Type, OwnerID: playerID,
	})

	if PlayerMeetsRoundRequirement(state, playerID) {
		state.RoundReqMet[playerID] = true
	}
	if !wasReqMet {
		state.MeldsLaidThisTurn++
	}

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
	if err := requireCardsInHand(state.Hands[playerID], []string{card}); err != nil {
		return state, err
	}

	owner, idx := findMeldByID(state, meldID)
	if owner == "" {
		return state, RulesError{Code: ErrInvalidMeld}
	}

	newMeld := append(append([]string(nil), state.Melds[owner][idx]...), card)
	if _, err := ValidateMeld(newMeld); err != nil {
		return state, err
	}

	state.Hands[playerID] = removeCards(state.Hands[playerID], []string{card})
	state.Melds[owner][idx] = newMeld

	if state.Round < 7 && len(state.Hands[playerID]) == 0 {
		return state, RulesError{Code: ErrInvalidMeld, Message: "must discard last card to go out before round 7"}
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
	// A player who started laying melds toward their (still unmet) initial
	// round requirement this turn must finish it before ending their turn —
	// no leaving a lone partial meld on the table across turns.
	if state.MeldsLaidThisTurn > 0 && !state.RoundReqMet[playerID] {
		return state, false, RulesError{
			Code:    ErrIncompleteInitialMeld,
			Message: "finish laying your full initial meld combination (all required sets/runs, total value met) before you can discard",
		}
	}

	state.Hands[playerID] = removeCards(state.Hands[playerID], []string{card})
	state.DiscardPile = append(state.DiscardPile, card)

	// Go-out check: if hand empty after discard, must have met round requirement.
	if len(state.Hands[playerID]) == 0 {
		if !state.RoundReqMet[playerID] {
			return state, false, RulesError{Code: ErrRoundReqNotMet}
		}
		// Round ends immediately; offer is not created.
		state.Offer = nil
		return state, true, nil
	}

	// Create offer for next player; only the offeree may respond.
	next := nextPlayer(state.TurnOrder, playerID)
	state.Offer = &DiscardOffer{Card: card, OfferedTo: next}
	state.Phase = PhaseOffer
	state.CurrentTurn = next

	return state, false, nil
}

func discardDrawAvailable(state GameState) bool {
	locked := state.DiscardDrawMinRound > 1 && state.Round < state.DiscardDrawMinRound
	return len(state.DiscardPile) > 0 && !locked
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

