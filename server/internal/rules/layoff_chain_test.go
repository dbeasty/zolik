package rules

import "testing"

// chainState is the reported bug, as a board: a run of 7-8-9-10 belonging to
// someone else, and a hand holding the 5 and the 6. The 6 extends the run on
// its own; the 5 only does so with the 6 alongside it.
func chainState(hand []string) GameState {
	return GameState{
		Status:      StatusActive,
		Rules:       continentalNoFloor(),
		GameNumber:  1,
		Round:       1,
		Phase:       PhaseMeld,
		CurrentTurn: "me",
		TurnOrder:   []string{"me", "karel"},
		Hands:       map[string][]string{"me": hand, "karel": {"2D"}},
		Melds:       map[string][][]string{"karel": {{"7C", "8C", "9C", "TC"}}},
		MeldMeta: map[string][]MeldInfo{"karel": {
			{MeldID: "m1", Type: MeldRun, OwnerID: "karel"},
		}},
		RoundReqMet: map[string]bool{"me": true, "karel": true},
		GameScores:  map[string][]int{},
		TotalScores: map[string]int{},
	}
}

func layOffPlacementFor(t *testing.T, st GameState, meldID, card string) *Placement {
	t.Helper()
	o := FindOffer(LegalActions(st, "me"), LayOffOfferID(meldID))
	if o == nil || o.Source == nil {
		t.Fatalf("no lay-off offer for %s", meldID)
	}
	for i := range o.Source.Placements {
		if o.Source.Placements[i].Card == card {
			return &o.Source.Placements[i]
		}
	}
	return nil
}

// The bug this file exists for. Dropping the 5 and the 6 together onto a run
// of 7-8-9-10 is a move ValidateLayOff has always accepted, but the offer
// listed only the cards that extend the run *on their own* — so the client,
// which trusts that list, refused the pair and made the player lay one card
// at a time.
func TestLayOffPlacements_BridgesAGapWhenBothCardsAreHeld(t *testing.T) {
	st := chainState([]string{"5C", "6C", "2H"})

	six := layOffPlacementFor(t, st, "m1", "6C")
	if six == nil {
		t.Fatal("the 6 extends the run on its own and must be offered")
	}
	if len(six.Requires) != 0 {
		t.Fatalf("the 6 needs no company, got requires=%v", six.Requires)
	}

	five := layOffPlacementFor(t, st, "m1", "5C")
	if five == nil {
		t.Fatal("the 5 is playable alongside the 6 and must be offered")
	}
	if len(five.Requires) != 1 || five.Requires[0] != "6C" {
		t.Fatalf("the 5 needs exactly the 6, got requires=%v", five.Requires)
	}

	// And the engine agrees, which is the whole point of listing it.
	if _, err := ValidateLayOff(cloneState(st), "me", "m1", []string{"6C", "5C"}, ""); err != nil {
		t.Fatalf("the pair the offer now lists was refused by the engine: %v", err)
	}
}

// Source.Cards stays the cards that may be sent *alone*, because two things
// read it that way and neither can be handed a card needing company: the RN
// client's isOneTap turns a single-card list into a pressable button, and the
// terminal client submits Source.Cards[:MinCards] sight unseen.
func TestLayOffPlacements_SourceCardsHoldOnlyTheStandaloneOnes(t *testing.T) {
	st := chainState([]string{"5C", "6C", "2H"})
	o := FindOffer(LegalActions(st, "me"), LayOffOfferID("m1"))
	if o == nil || o.Source == nil {
		t.Fatal("no lay-off offer")
	}
	if len(o.Source.Cards) != 1 || o.Source.Cards[0] != "6C" {
		t.Fatalf("Source.Cards must be the 6 alone, got %v", o.Source.Cards)
	}
	if len(o.Source.Placements) != 2 {
		t.Fatalf("both cards must still be listed as placements, got %v", o.Source.Placements)
	}
	// The engine's own answer for the card that needs company, so the two
	// lists cannot drift apart on what "alone" means.
	if _, err := ValidateLayOff(cloneState(st), "me", "m1", []string{"5C"}, ""); err == nil {
		t.Fatal("the 5 alone leaves a gap at the 6 and must be refused")
	}
}

// Each end of a run chains independently. This is the test that fails if the
// prerequisite set is shrunk in acceptance order instead of reverse: dropping
// the jack while the queen is still in the trial set leaves a dangling queen,
// the meld fails for the queen's sake, and the 5 is wrongly reported as
// needing the jack.
func TestLayOffPlacements_ChainsEachEndOfARunSeparately(t *testing.T) {
	st := chainState([]string{"5C", "6C", "JC", "QC", "2H"})

	for _, tc := range []struct{ card, want string }{
		{"5C", "6C"},
		{"QC", "JC"},
	} {
		p := layOffPlacementFor(t, st, "m1", tc.card)
		if p == nil {
			t.Fatalf("%s is playable in company and must be offered", tc.card)
		}
		if len(p.Requires) != 1 || p.Requires[0] != tc.want {
			t.Fatalf("%s must need exactly %s, got requires=%v", tc.card, tc.want, p.Requires)
		}
	}
}

// A placement's Requires is a promise, not a hint: whatever it names, plus
// the card itself, has to be a submission the engine takes.
func TestLayOffPlacements_RequiresIsASubmissionTheEngineAccepts(t *testing.T) {
	st := chainState([]string{"5C", "6C", "JC", "QC", "2H"})
	o := FindOffer(LegalActions(st, "me"), LayOffOfferID("m1"))
	if o == nil || o.Source == nil {
		t.Fatal("no lay-off offer")
	}
	// cloneState per placement: GameState carries maps, so a successful
	// ValidateLayOff writes the shortened hand straight back through the
	// caller's own state and the next placement would be judged against a
	// hand the player still holds.
	for _, p := range o.Source.Placements {
		cards := append(append([]string(nil), p.Requires...), p.Card)
		if _, err := ValidateLayOff(cloneState(st), "me", "m1", cards, ""); err != nil {
			t.Errorf("offer lists %s requiring %v, but the engine refuses %v: %v",
				p.Card, p.Requires, cards, err)
		}
	}
}

// A card that cannot be sent on its own carries a run-end hint describing the
// submission it belongs to, not the card alone — there is no such thing as
// dropping it alone. The 5 grows the front, alongside the 6.
func TestLayOffPlacements_ChainedCardHintsTheEndItsSubmissionGrows(t *testing.T) {
	st := chainState([]string{"5C", "6C", "2H"})
	five := layOffPlacementFor(t, st, "m1", "5C")
	if five == nil {
		t.Fatal("the 5 must be offered")
	}
	if len(five.Positions) != 1 || five.Positions[0] != "front" {
		t.Fatalf("the 5 and 6 grow the front of the run, got positions=%v", five.Positions)
	}
}

// A set cannot chain: one card per suit, four in total, so a set on the table
// has room for at most one more card and that card always fits on its own.
// The scan is skipped for sets entirely, and this pins the behaviour that
// justifies skipping it.
func TestLayOffPlacements_SetsNeverNeedCompany(t *testing.T) {
	st := chainState([]string{"7S", "7H", "2H"})
	st.Melds["karel"] = [][]string{{"7C", "7D", "7H"}}
	st.MeldMeta["karel"] = []MeldInfo{{MeldID: "m1", Type: MeldSet, OwnerID: "karel"}}

	o := FindOffer(LegalActions(st, "me"), LayOffOfferID("m1"))
	if o == nil || o.Source == nil {
		t.Fatal("no lay-off offer")
	}
	for _, p := range o.Source.Placements {
		if len(p.Requires) != 0 {
			t.Errorf("a set placement must never need company, %s wants %v", p.Card, p.Requires)
		}
	}
	if len(o.Source.Cards) != len(o.Source.Placements) {
		t.Fatalf("for a set every placement is standalone: cards=%v placements=%v",
			o.Source.Cards, o.Source.Placements)
	}
}

// Two jokers each extend a four-card run on their own, but not together —
// they would land adjacent. The chain scan must not swallow the second one:
// it runs against the meld as it lies, separately from the pass that grows a
// working copy. Merge the two passes and the second joker vanishes from the
// list while the engine still accepts it, which is exactly the drift
// TestLegalActions_AgreesWithApplyAction exists to catch.
func TestLayOffPlacements_TwoJokersAtOneEndAreBothStillOffered(t *testing.T) {
	st := chainState([]string{"JOKER1", "JOKER2", "2H"})
	o := FindOffer(LegalActions(st, "me"), LayOffOfferID("m1"))
	if o == nil || o.Source == nil {
		t.Fatal("no lay-off offer")
	}
	for _, j := range []string{"JOKER1", "JOKER2"} {
		if !containsString(o.Source.Cards, j) {
			t.Errorf("%s extends the run on its own and must be sendable alone; cards=%v",
				j, o.Source.Cards)
		}
	}
}

// The chain may not swallow the whole hand on a non-final deal: the engine
// refuses the lay-off that leaves nothing to discard, so offering it would be
// a promise the offer cannot keep.
func TestLayOffPlacements_ChainLeavesACardToDiscard(t *testing.T) {
	st := chainState([]string{"5C", "6C"})
	o := FindOffer(LegalActions(st, "me"), LayOffOfferID("m1"))
	if o == nil || o.Source == nil {
		t.Fatal("no lay-off offer")
	}
	for _, p := range o.Source.Placements {
		cards := append(append([]string(nil), p.Requires...), p.Card)
		if _, err := ValidateLayOff(cloneState(st), "me", "m1", cards, ""); err != nil {
			t.Errorf("offer lists %s requiring %v, but %v empties the hand: %v",
				p.Card, p.Requires, cards, err)
		}
	}
}

// The chain stops where the validator stops. A run has a ceiling — the ace at
// the bottom, twelve ranks above it, the ace at the top (MaxRunLength, #43) —
// and the closure discovers that by asking ValidateMeld rather than by knowing
// it. This pins that the ceiling is respected through the offer path too: the
// cards that would push the run past it are simply not listed.
func TestLayOffPlacements_ChainStopsAtTheRunCeiling(t *testing.T) {
	// Twelve of the thirteen slots already on the table, ace-low through queen.
	run := []string{"AC", "2C", "3C", "4C", "5C", "6C", "7C", "8C", "9C", "TC", "JC", "QC"}
	st := chainState([]string{"KC", "2H", "3H"})
	st.Melds["karel"] = [][]string{run}

	o := FindOffer(LegalActions(st, "me"), LayOffOfferID("m1"))
	if o == nil || o.Source == nil {
		t.Fatal("no lay-off offer")
	}
	// Whatever it offers, the engine has to take — ceiling included.
	for _, p := range o.Source.Placements {
		cards := append(append([]string(nil), p.Requires...), p.Card)
		if _, err := ValidateLayOff(cloneState(st), "me", "m1", cards, ""); err != nil {
			t.Errorf("offer lists %s requiring %v, but the engine refuses %v: %v",
				p.Card, p.Requires, cards, err)
		}
	}
	if len(o.Source.Placements) > 1 {
		t.Fatalf("only the king fits below the ceiling, got %v", o.Source.Placements)
	}
}
