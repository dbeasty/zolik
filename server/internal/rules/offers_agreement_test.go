package rules

import (
	"fmt"
	"testing"
)

// This is the load-bearing test for the whole offer mechanism.
//
// The problem Phase 1 exists to solve is *drift*: a second implementation of
// a rule that agrees with the first today and disagrees after the next edit.
// Shipping legal actions only helps if the offer list and the engine can
// never disagree — so rather than asserting particular offers in particular
// positions (which would pin today's behaviour without preventing drift),
// this test cross-checks the two implementations against each other over a
// corpus of states.
//
// For every state × player × concrete action it can construct, it asks:
//   - what does LegalActions say about that action's offer?
//   - what does ApplyAction actually do with it?
// and fails on any disagreement. Add a rule to one side without the other
// and this test goes red.

// concreteActions enumerates every action a client could plausibly send in a
// state, paired with the offer ID that is supposed to govern it.
func concreteActions(state GameState, playerID string) []struct {
	OfferID string
	Action  Action
} {
	type pair struct {
		OfferID string
		Action  Action
	}
	var out []pair

	out = append(out,
		pair{OfferDrawDeck, Action{Type: ActionDrawCard, DrawFrom: DrawFromDeck}},
		pair{OfferDrawDiscard, Action{Type: ActionDrawCard, DrawFrom: DrawFromDiscard}},
		pair{OfferUndoDrawDiscard, Action{Type: ActionUndoDrawDiscard}},
		pair{OfferUndoLayOff, Action{Type: ActionUndoLayOff}},
		pair{OfferUndoLayMeld, Action{Type: ActionUndoLayMeld}},
		pair{OfferUndoTurn, Action{Type: ActionUndoTurn}},
	)

	seen := map[string]bool{}
	for _, c := range state.Hands[playerID] {
		if seen[c] {
			continue
		}
		seen[c] = true
		out = append(out, pair{OfferDiscard, Action{Type: ActionDiscard, Card: c}})
		for _, m := range tableMelds(state) {
			out = append(out,
				pair{LayOffOfferID(m.MeldID), Action{Type: ActionLayOff, MeldID: m.MeldID, Card: c}},
				pair{SwapJokerOfferID(m.MeldID), Action{Type: ActionSwapJoker, MeldID: m.MeldID, Card: c}},
			)
		}
	}
	res := make([]struct {
		OfferID string
		Action  Action
	}, len(out))
	for i, p := range out {
		res[i] = struct {
			OfferID string
			Action  Action
		}{p.OfferID, p.Action}
	}
	return res
}

// TestLegalActions_AgreesWithApplyAction is the anti-drift guarantee.
//
// The contract being checked, in both directions:
//
//	engine accepts an action  =>  its offer is enabled, and (where the offer
//	                              lists eligible cards) that card is listed
//	offer lists a card        =>  the engine accepts that exact action
//
// A disabled offer is *not* required to mean every concrete action under it
// fails for the offer's stated reason — a lay-off offer stays enabled while
// individual cards do not fit — which is why the card lists, not just the
// booleans, are what the strict direction checks.
func TestLegalActions_AgreesWithApplyAction(t *testing.T) {
	for _, sc := range agreementCorpus() {
		for _, pid := range sc.state.TurnOrder {
			offers := LegalActions(sc.state, pid)

			for _, ca := range concreteActions(sc.state, pid) {
				_, applyErr := ApplyAction(cloneState(sc.state), pid, ca.Action)
				accepted := applyErr == nil

				offer := FindOffer(offers, ca.OfferID)
				if offer == nil {
					t.Fatalf("%s/%s: no offer %q for action %+v", sc.name, pid, ca.OfferID, ca.Action)
				}

				// Direction 1: anything the engine accepts must be offered.
				if accepted && !offer.Enabled {
					t.Errorf("%s/%s: engine ACCEPTS %+v but offer %q is disabled (whyNot=%s)",
						sc.name, pid, ca.Action, offer.ID, offer.WhyNot)
				}

				// Direction 2: any card the offer lists must be accepted.
				card := ca.Action.Card
				if card != "" && offer.Source != nil && offerListsCard(offer, card) && !accepted {
					t.Errorf("%s/%s: offer %q lists card %s but engine REJECTS %+v: %v",
						sc.name, pid, offer.ID, card, ca.Action, applyErr)
				}

				// Direction 3: for the card-listing offers, an accepted
				// action's card must appear in the list — otherwise a client
				// that trusts the list would grey out a legal move.
				if accepted && card != "" && listsCards(offer) && !offerListsCard(offer, card) {
					t.Errorf("%s/%s: engine ACCEPTS %+v but offer %q does not list card %s (cards=%v)",
						sc.name, pid, ca.Action, offer.ID, card, offer.Source.Cards)
				}
			}
		}
	}
}

// TestLegalActions_RunEndHintsMatchTheValidator checks the third direction
// for lay-off placements specifically: a "front"/"end" hint the client
// renders must be exactly what ValidateLayOff will accept, and an end the
// hint omits must be what it rejects with WRONG_RUN_END. This is where the
// shared runGrowthSides helper earns its keep.
func TestLegalActions_RunEndHintsMatchTheValidator(t *testing.T) {
	for _, sc := range agreementCorpus() {
		for _, pid := range sc.state.TurnOrder {
			for _, o := range LegalActions(sc.state, pid) {
				if o.Verb != VerbLayOff || o.Source == nil {
					continue
				}
				for _, p := range o.Source.Placements {
					for _, pos := range []string{"front", "end"} {
						_, err := ApplyAction(cloneState(sc.state), pid, Action{
							Type: ActionLayOff, MeldID: o.Target.MeldID,
							Card: p.Card, Position: pos,
						})
						hinted := containsString(p.Positions, pos)
						wrongEnd := codeOf(err) == ErrWrongRunEnd && err != nil

						if hinted && wrongEnd {
							t.Errorf("%s/%s: offer %q hints %s=%s but validator says WRONG_RUN_END",
								sc.name, pid, o.ID, p.Card, pos)
						}
						// The converse only holds when the placement named
						// any end at all — an empty hint list means "send no
						// position", not "both ends are rejected".
						if len(p.Positions) > 0 && !hinted && !wrongEnd && err == nil {
							t.Errorf("%s/%s: offer %q omits %s=%s but validator accepts it",
								sc.name, pid, o.ID, p.Card, pos)
						}
					}
				}
			}
		}
	}
}

func listsCards(o *ActionOffer) bool {
	return o.Source != nil && (len(o.Source.Cards) > 0 || len(o.Source.Placements) > 0)
}

func offerListsCard(o *ActionOffer, card string) bool {
	if o.Source == nil {
		return false
	}
	return containsString(o.Source.Cards, card)
}

type agreementCase struct {
	name  string
	state GameState
}

// agreementCorpus is deliberately broad rather than deep: the point is to
// hit every gate in the engine (wrong phase, not your turn, discard lock,
// clean-run protection, the joker discard rule, both initial-meld
// obligations, each undo window) at least once, under more than one profile,
// so the cross-check above has something to disagree about.
func agreementCorpus() []agreementCase {
	base := func(mut func(*GameState)) GameState {
		s := GameState{
			Status:        StatusActive,
			Rules:         continentalNoFloor(),
			GameNumber:    1,
			Round:         1,
			Phase:         PhaseDraw,
			CurrentTurn:   "p1",
			TurnOrder:     []string{"p1", "p2"},
			DealStarterID: "p1",
			DrawPile:      []string{"2C", "3C", "4C", "5C", "6C"},
			DiscardPile:   []string{"9H", "TH"},
			Hands: map[string][]string{
				"p1": {"7H", "7D", "7S", "8H", "9H", "TH", "JOKER1", "AS"},
				"p2": {"2H", "3H", "4H", "5D"},
			},
			Melds:       map[string][][]string{},
			MeldMeta:    map[string][]MeldInfo{},
			RoundReqMet: map[string]bool{},
			GameScores:  map[string][]int{},
			TotalScores: map[string]int{},
		}
		mut(&s)
		return s
	}

	withTable := func(s *GameState) {
		s.Melds["p2"] = [][]string{
			{"5H", "6H", "7H", "8H"}, // clean run, extendable at both ends
			{"QH", "QD", "QC"},       // set
			{"2S", "3S", "JOKER2"},   // run holding a joker to swap
		}
		s.MeldMeta["p2"] = []MeldInfo{
			{MeldID: "meld_1", Type: MeldRun, OwnerID: "p2", WildCount: 0},
			{MeldID: "meld_2", Type: MeldSet, OwnerID: "p2", WildCount: 0},
			{MeldID: "meld_3", Type: MeldRun, OwnerID: "p2", WildCount: 1},
		}
		s.NextMeldSeq = 3
	}

	cases := []agreementCase{
		{"draw phase, nothing on table", base(func(s *GameState) {})},
		{"draw phase, table populated", base(withTable)},
		{"meld phase, not down", base(func(s *GameState) {
			withTable(s)
			s.Phase = PhaseMeld
		})},
		{"meld phase, down", base(func(s *GameState) {
			withTable(s)
			s.Phase = PhaseMeld
			s.RoundReqMet["p1"] = true
		})},
		{"discard phase, down", base(func(s *GameState) {
			withTable(s)
			s.Phase = PhaseDiscard
			s.RoundReqMet["p1"] = true
		})},
		{"other player's turn", base(func(s *GameState) {
			withTable(s)
			s.Phase = PhaseMeld
			s.CurrentTurn = "p2"
			s.RoundReqMet["p2"] = true
		})},
		{"suspended", base(func(s *GameState) {
			withTable(s)
			s.Phase = PhaseSuspended
			s.Status = StatusSuspended
		})},
		// zolik_classic deliberately, not continental: continental locks the
		// discard pile until round 3, so under it an empty pile is reported
		// as DISCARD_LOCKED and this case would never reach the emptiness
		// check it exists to cover.
		{"discard pile empty", base(func(s *GameState) {
			s.Rules = ProfileZolikClassic
			s.DiscardPile = nil
		})},
		{"discard pile locked until round 3", base(func(s *GameState) {
			cfg := continentalNoFloor()
			cfg.DiscardDrawMinRound = 3
			s.Rules = cfg
			s.Round = 1
		})},
		{"discard pile unlocked at round 3", base(func(s *GameState) {
			cfg := continentalNoFloor()
			cfg.DiscardDrawMinRound = 3
			s.Rules = cfg
			s.Round = 3
		})},
		{"zolik_classic, any-from-pile pickup", base(func(s *GameState) {
			withTable(s)
			s.Rules = ProfileZolikClassic
			s.DiscardPile = []string{"9H", "TH", "JH", "QH"}
		})},
		{"zolik_classic, meld phase down, joker discard restricted", base(func(s *GameState) {
			withTable(s)
			s.Rules = ProfileZolikClassic
			s.Phase = PhaseMeld
			s.RoundReqMet["p1"] = true
		})},
		{"joker is the only card left, already down", base(func(s *GameState) {
			s.Rules = ProfileZolikClassic
			s.Phase = PhaseMeld
			s.RoundReqMet["p1"] = true
			s.Hands["p1"] = []string{"JOKER1"}
		})},
		{"pending discard-pickup obligation", base(func(s *GameState) {
			withTable(s)
			s.Phase = PhaseMeld
			s.DiscardDrawnCardPendingMeld = "TH"
			s.DiscardDrawnCards = []string{"TH"}
		})},
		{"started but unfinished initial meld", base(func(s *GameState) {
			withTable(s)
			s.Phase = PhaseMeld
			s.MeldsLaidThisTurn = 1
		})},
		{"undo lay-off window open", base(func(s *GameState) {
			withTable(s)
			s.Phase = PhaseMeld
			s.RoundReqMet["p1"] = true
			s.LastLayOff = &LayOffSnapshot{
				PlayerID: "p1", MeldID: "meld_2",
				PrevCards: []string{"QH", "QD", "QC"},
				PrevMeta:  MeldInfo{MeldID: "meld_2", Type: MeldSet, OwnerID: "p2"},
				Cards:     []string{"QS"},
			}
		})},
		{"undo lay-meld window open", base(func(s *GameState) {
			withTable(s)
			s.Phase = PhaseMeld
			s.LastMeldLaid = &MeldLaidSnapshot{
				PlayerID: "p1", MeldID: "meld_2",
				Cards: []string{"QH", "QD", "QC"},
			}
		})},
		{"undo turn window open", base(func(s *GameState) {
			withTable(s)
			s.Phase = PhaseMeld
			s.TurnMeldSnapshot = &TurnMeldSnapshot{
				PlayerID:    "p1",
				Hands:       map[string][]string{"p1": {"7H"}},
				Melds:       map[string][][]string{},
				MeldMeta:    map[string][]MeldInfo{},
				DiscardPile: []string{},
			}
		})},
		{"clean-run contract, extending the only clean run", base(func(s *GameState) {
			cfg := ProfileZolikClassic
			s.Rules = cfg
			s.Phase = PhaseMeld
			s.RoundReqMet["p1"] = true
			s.Melds["p1"] = [][]string{{"5H", "6H", "7H"}}
			s.MeldMeta["p1"] = []MeldInfo{{MeldID: "meld_1", Type: MeldRun, OwnerID: "p1"}}
			s.NextMeldSeq = 1
			s.Hands["p1"] = []string{"JOKER1", "8H", "4H", "2D"}
		})},
		{"final deal of a five-deal profile", base(func(s *GameState) {
			cfg := continentalNoFloor()
			cfg.FixedDealCount = 5
			s.Rules = cfg
			s.GameNumber = 5
			s.Phase = PhaseMeld
			s.RoundReqMet["p1"] = true
			s.Melds["p1"] = [][]string{{"5H", "6H", "7H", "8H"}}
			s.MeldMeta["p1"] = []MeldInfo{{MeldID: "meld_1", Type: MeldRun, OwnerID: "p1"}}
			s.NextMeldSeq = 1
			s.Hands["p1"] = []string{"9H"} // laying this off empties the hand
		})},
		{"non-final deal, one card left (must keep a discard)", base(func(s *GameState) {
			s.Phase = PhaseMeld
			s.RoundReqMet["p1"] = true
			s.Melds["p1"] = [][]string{{"5H", "6H", "7H", "8H"}}
			s.MeldMeta["p1"] = []MeldInfo{{MeldID: "meld_1", Type: MeldRun, OwnerID: "p1"}}
			s.NextMeldSeq = 1
			s.Hands["p1"] = []string{"9H"}
		})},
		{"game not active (lobby)", base(func(s *GameState) {
			s.Status = StatusLobby
		})},
		{"game completed", base(func(s *GameState) {
			s.Status = StatusCompleted
		})},
	}
	return cases
}

// TestAgreementCorpus_IsActuallyExercisingTheEngine guards the guard: a
// corpus where nothing is ever legal would pass the agreement test
// vacuously. Assert that the corpus produces both enabled and disabled
// offers for every verb the module defines.
func TestAgreementCorpus_IsActuallyExercisingTheEngine(t *testing.T) {
	sawEnabled := map[OfferVerb]bool{}
	sawDisabled := map[OfferVerb]bool{}
	sawWhyNot := map[RulesErrorCode]bool{}

	for _, sc := range agreementCorpus() {
		for _, pid := range sc.state.TurnOrder {
			for _, o := range LegalActions(sc.state, pid) {
				if o.Enabled {
					sawEnabled[o.Verb] = true
				} else {
					sawDisabled[o.Verb] = true
					sawWhyNot[o.WhyNot] = true
				}
			}
		}
	}

	for _, v := range []OfferVerb{VerbDraw, VerbLayMeld, VerbLayOff, VerbSwapJoker, VerbDiscard, VerbUndo} {
		if !sawEnabled[v] {
			t.Errorf("corpus never produces an ENABLED %s offer — the agreement test is vacuous for it", v)
		}
		if !sawDisabled[v] {
			t.Errorf("corpus never produces a DISABLED %s offer — the agreement test is vacuous for it", v)
		}
	}

	// The reasons a client is expected to render must all be reachable.
	for _, code := range []RulesErrorCode{
		ErrNotYourTurn, ErrWrongPhase, ErrDiscardLocked, ErrDiscardPileEmpty,
		ErrRoundReqNotMet, ErrNothingToUndo, ErrGameNotActive,
	} {
		if !sawWhyNot[code] {
			t.Errorf("corpus never produces whyNot=%s", code)
		}
	}

	if testing.Verbose() {
		fmt.Printf("agreement corpus: %d states\n", len(agreementCorpus()))
	}
}
