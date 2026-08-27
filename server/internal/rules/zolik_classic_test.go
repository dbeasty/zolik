package rules

import (
	"strings"
	"testing"
)

func classicState(playerID string) GameState {
	return GameState{
		Status:      StatusActive,
		Rules:       ProfileZolikClassic,
		GameNumber:  1,
		Phase:       PhaseMeld,
		CurrentTurn: playerID,
		TurnOrder:   []string{playerID, "p2"},
		Hands:       map[string][]string{playerID: {}, "p2": {}},
		Melds:       map[string][][]string{},
		MeldMeta:    map[string][]MeldInfo{},
		RoundReqMet: map[string]bool{playerID: false, "p2": false},
		DrawPile:    []string{"2C"},
		DiscardPile: []string{},
		DeckSeed:    42,
		GameScores:  map[string][]int{playerID: {}, "p2": {}},
		TotalScores: map[string]int{playerID: 0, "p2": 0},
	}
}

func TestZolikClassic_SetsAloneNeverSatisfyRequirement(t *testing.T) {
	p := "p1"
	st := classicState(p)
	st.Hands[p] = []string{"7H", "7D", "7C", "8H", "8D", "8C", "2S"}

	st, _, _, err := ValidateMeldAction(st, p, []string{"7H", "7D", "7C"})
	if err != nil {
		t.Fatalf("a set should always be a valid meld: %v", err)
	}
	if st.RoundReqMet[p] {
		t.Fatal("sets alone must never satisfy Žolík Classic's clean-run requirement")
	}

	st, _, _, err = ValidateMeldAction(st, p, []string{"8H", "8D", "8C"})
	if err != nil {
		t.Fatalf("a second set should also be laid freely: %v", err)
	}
	if st.RoundReqMet[p] {
		t.Fatal("still not down: two sets and no clean run")
	}
}

func TestZolikClassic_RunWithJokerDoesNotSatisfyRequirement(t *testing.T) {
	p := "p1"
	st := classicState(p)
	st.Hands[p] = []string{"5H", "6H", "JOKER1", "8H", "2S"}

	st, _, _, err := ValidateMeldAction(st, p, []string{"5H", "6H", "JOKER1", "8H"})
	if err != nil {
		t.Fatalf("a run using a joker should still be a valid meld: %v", err)
	}
	if st.RoundReqMet[p] {
		t.Fatal("a run containing a joker must not satisfy the clean-run requirement")
	}
}

func TestZolikClassic_CleanRunSatisfiesRequirement(t *testing.T) {
	p := "p1"
	st := classicState(p)
	st.Hands[p] = []string{"5H", "6H", "7H", "8H", "2S"}

	st, _, _, err := ValidateMeldAction(st, p, []string{"5H", "6H", "7H", "8H"})
	if err != nil {
		t.Fatalf("clean run should be a valid meld: %v", err)
	}
	if !st.RoundReqMet[p] {
		t.Fatal("a joker-free run should satisfy Žolík Classic's requirement and mark the player down")
	}
}

func TestZolikClassic_NoContractCountGate_UnlikeContinental(t *testing.T) {
	// Regression test for the exact bug report: under Continental's rotation,
	// laying an unrelated meld type before the deal's required combo is met
	// is rejected with MELD_NO_CONTRIBUTION. Žolík Classic has no such
	// per-deal contract — any valid meld should be layable at any time (the
	// only remaining gate is the clean-run requirement itself, tested
	// separately above).
	p := "p1"
	st := classicState(p)
	st.Hands[p] = []string{"7H", "7D", "7C", "2S"}

	_, _, _, err := ValidateMeldAction(st, p, []string{"7H", "7D", "7C"})
	if err != nil {
		t.Fatalf("expected the set to be accepted without a contract-count gate, got: %v", err)
	}
}

func TestZolikClassic_JokerCannotBeDiscarded(t *testing.T) {
	p := "p1"
	st := classicState(p)
	st.Phase = PhaseDiscard
	st.Hands[p] = []string{"JOKER1", "5H"}

	_, _, err := ValidateDiscard(st, p, "JOKER1", nil)
	if err == nil {
		t.Fatal("expected joker discard to be rejected")
	}
	re, ok := err.(RulesError)
	if !ok || re.Code != ErrJokerDiscard {
		t.Fatalf("expected JOKER_DISCARD_FORBIDDEN, got %v", err)
	}
}

func TestZolikClassic_JokerCanEndTheHand(t *testing.T) {
	p := "p1"
	st := classicState(p)
	st.Phase = PhaseDiscard
	st.RoundReqMet[p] = true
	st.Hands[p] = []string{"JOKER1"}

	_, goOut, err := ValidateDiscard(st, p, "JOKER1", nil)
	if err != nil {
		t.Fatalf("expected the winning joker discard to be accepted: %v", err)
	}
	if !goOut {
		t.Fatal("expected discarding the last card while down to end the deal")
	}
}

func TestZolikClassic_JokerCannotSneakOutWithoutBeingDown(t *testing.T) {
	// Even as the literal last card in hand, a joker discard is still
	// forbidden if the player never met the clean-run requirement — the
	// "ends the game" exception only covers a legitimate go-out.
	//
	// The error code is the general go-out gate (ROUND_REQ_NOT_MET) rather
	// than the joker-specific one: a lone joker with nowhere to lay off is
	// this player's only conceivable move, which is exactly the case
	// jokerDiscardIsOnlyMove lets through the joker gate for — but the empty-
	// hand-without-being-down check underneath still catches it, same as it
	// would for any other last card.
	p := "p1"
	st := classicState(p)
	st.Phase = PhaseDiscard
	st.Hands[p] = []string{"JOKER1"}

	_, _, err := ValidateDiscard(st, p, "JOKER1", nil)
	if err == nil {
		t.Fatal("expected joker discard to still be rejected when not down")
	}
	re, ok := err.(RulesError)
	if !ok || re.Code != ErrRoundReqNotMet {
		t.Fatalf("expected ROUND_REQ_NOT_MET, got %v", err)
	}
}

func TestZolikClassic_JokerDiscardAllowedAsOnlyMove(t *testing.T) {
	// A down player holding nothing but jokers, with every table meld already
	// full, has no other legal action at all: no natural to start a new meld
	// with, and no room to lay either joker off anywhere. Refusing the
	// discard here would strand them for the rest of the deal.
	p := "p1"
	st := classicState(p)
	st.Phase = PhaseMeld
	st.RoundReqMet[p] = true
	st.Hands[p] = []string{"JOKER1", "JOKER2"}
	st.Melds[p] = [][]string{{"9C", "9D", "9H", "9S"}}
	st.MeldMeta[p] = []MeldInfo{{MeldID: "m1", OwnerID: p, Type: MeldSet}}

	_, _, err := ValidateDiscard(st, p, "JOKER1", nil)
	if err != nil {
		t.Fatalf("expected the joker discard to be allowed as the only legal move, got %v", err)
	}
}

func TestZolikClassic_JokerDiscardStillBlockedWhenLayOffPossible(t *testing.T) {
	// Same shape as above, but the table meld has room for one more joker —
	// laying it off is a real alternative, so the discard restriction stands.
	p := "p1"
	st := classicState(p)
	st.Phase = PhaseMeld
	st.RoundReqMet[p] = true
	st.Hands[p] = []string{"JOKER1", "JOKER2"}
	st.Melds[p] = [][]string{{"9C", "9D", "9H"}}
	st.MeldMeta[p] = []MeldInfo{{MeldID: "m1", OwnerID: p, Type: MeldSet}}

	_, _, err := ValidateDiscard(st, p, "JOKER1", nil)
	if err == nil {
		t.Fatal("expected joker discard to still be rejected while a lay-off is available")
	}
	re, ok := err.(RulesError)
	if !ok || re.Code != ErrJokerDiscard {
		t.Fatalf("expected JOKER_DISCARD_FORBIDDEN, got %v", err)
	}
}

func TestZolikClassic_MinRunSizeThree(t *testing.T) {
	mv, err := ValidateMeld([]string{"5H", "6H", "7H"}, ProfileZolikClassic)
	if err != nil {
		t.Fatalf("a 3-card run should be valid under Žolík Classic: %v", err)
	}
	if mv.Type != MeldRun {
		t.Fatalf("expected a run, got %v", mv.Type)
	}
}

func TestContinental_MinRunSizeStillFour(t *testing.T) {
	if _, err := ValidateMeld([]string{"5H", "6H", "7H"}, ProfileContinental); err == nil {
		t.Fatal("a 3-card run must still be rejected under Continental")
	}
}

func TestResolveProfile_UnknownFallsBackToZolikClassic(t *testing.T) {
	if ResolveProfile("something-made-up") != ProfileZolikClassic {
		t.Fatal("unknown profile name should resolve to ProfileZolikClassic")
	}
}

// A clean run is the ticket down, not a permanent fixture. Žolík HD (the
// ruleset ProfileZolikClassic follows) lets any meld already on the table be
// extended with a joker during the lay-off phase, a run that went down
// joker-free included — the only limits on the finished meld are the ordinary
// wild ones (no more wilds than naturals, none adjacent).
//
// This used to be refused with BREAKS_CLEAN_RUN, on the reasoning that the
// clean run which let a player go down had to stay clean for the deal. It
// does not, and nothing downstream needs it to: going down is monotonic (see
// refreshRoundReqMet), so the flag survives the run being dirtied, and going
// out reads that flag rather than re-deriving the contract.
func TestZolikClassic_LayOffMayDirtyACleanRun(t *testing.T) {
	p := "p1"
	st := classicState(p)
	st.Hands[p] = []string{"5C", "6C", "7C", "8C", "JOKER1", "TC", "2S"}

	st, meldID, _, err := ValidateMeldAction(st, p, []string{"5C", "6C", "7C"})
	if err != nil {
		t.Fatalf("clean run should be a valid meld: %v", err)
	}
	if !st.RoundReqMet[p] {
		t.Fatal("clean run should mark the player down")
	}

	// Extending it with a natural is fine — the run stays clean.
	st, err = ValidateLayOff(st, p, meldID, []string{"8C"}, "")
	if err != nil {
		t.Fatalf("laying off a natural onto a clean run should be allowed: %v", err)
	}

	// And so is extending it with a joker.
	st, err = ValidateLayOff(st, p, meldID, []string{"JOKER1"}, "")
	if err != nil {
		t.Fatalf("laying off a joker onto a clean run should be allowed: %v", err)
	}
	if _, _, hasCleanRun := PlayerMeldCounts(st, p); hasCleanRun {
		t.Fatal("the run now carries a joker, so no clean run should be left on the table")
	}
	// Down is down: dirtying the run that got the player there does not
	// take it back.
	if !st.RoundReqMet[p] {
		t.Fatal("the player must stay down after their clean run is dirtied")
	}
}

// The clean run is still required to go *down*: a player whose only run
// carries a joker has not met the contract, however many cards are on the
// table. Removing the lay-off lock must not weaken this gate.
func TestZolikClassic_CleanRunStillRequiredToGoDown(t *testing.T) {
	p := "p1"
	st := classicState(p)
	st.Hands[p] = []string{"5C", "6C", "JOKER1", "TC", "2S"}

	st, _, _, err := ValidateMeldAction(st, p, []string{"5C", "6C", "JOKER1"})
	if err != nil {
		t.Fatalf("a run with one joker is a valid meld: %v", err)
	}
	if st.RoundReqMet[p] {
		t.Fatal("a joker-carrying run must not satisfy the clean-run contract")
	}
}

// Reproduces a reported bug: laying a natural ace onto J-Q-K to extend the
// run ace-high (J-Q-K-A) was being mis-resolved as a 10-J-Q-K window with
// the ace standing in as a wild for the missing 10 — a valid but needlessly
// wild resolution that wrongly dirtied an otherwise clean run. The natural,
// wild-free ace-high resolution must win whenever both are possible.
func TestZolikClassic_LayOffNaturalAceExtendsRunAceHighWithoutGoingWild(t *testing.T) {
	p := "p1"
	st := classicState(p)
	st.Hands[p] = []string{"JD", "QD", "KD", "AD", "2S"}

	st, meldID, _, err := ValidateMeldAction(st, p, []string{"JD", "QD", "KD"})
	if err != nil {
		t.Fatalf("clean run should be a valid meld: %v", err)
	}

	st, err = ValidateLayOff(st, p, meldID, []string{"AD"}, "")
	if err != nil {
		t.Fatalf("laying a natural ace onto J-Q-K to make it ace-high should be allowed: %v", err)
	}

	got := st.Melds[p][0]
	want := []string{"JD", "QD", "KD", "AD"}
	if len(got) != len(want) {
		t.Fatalf("expected the run to grow to %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected the run to grow to %v, got %v", want, got)
		}
	}
	if meta := st.MeldMeta[p][0]; meta.WildCount != 0 {
		t.Fatalf("expected the ace to resolve naturally (WildCount 0), got %d", meta.WildCount)
	}
}

func TestZolikClassic_LayOffJokerAllowedOnceAnotherCleanRunExists(t *testing.T) {
	p := "p1"
	st := classicState(p)
	st.Hands[p] = []string{"5C", "6C", "7C", "5H", "6H", "7H", "JOKER1", "2S"}

	st, meldID, _, err := ValidateMeldAction(st, p, []string{"5C", "6C", "7C"})
	if err != nil {
		t.Fatalf("first clean run: %v", err)
	}
	st, _, _, err = ValidateMeldAction(st, p, []string{"5H", "6H", "7H"})
	if err != nil {
		t.Fatalf("second clean run: %v", err)
	}

	st, err = ValidateLayOff(st, p, meldID, []string{"JOKER1"}, "")
	if err != nil {
		t.Fatalf("a joker may dirty one run while another clean run remains: %v", err)
	}
	metas := st.MeldMeta[p]
	if metas[0].WildCount != 1 {
		t.Fatalf("lay-off must refresh the meld's wild count, got %d", metas[0].WildCount)
	}
}

// A lay-down that does not satisfy the contract cannot simply be walked away
// from at the end of the turn — the player must finish it or take it back.
//
// This reverses an earlier decision. Laying a set is freely allowed under
// Classic's no-count contract, and the "finish your initial meld before
// discarding" block used to be skipped here on the grounds that no number of
// sets can ever satisfy the clean-run requirement, so a player without a
// clean run in hand would be stuck. The exemption is what produced the
// reported bug: melds on the table, not down, every lay-off refused for the
// rest of the deal, and nothing saying a run was what was missing. Undo is
// the way out, not a discard that abandons a half-finished lay-down.
func TestZolikClassic_CannotDiscardAfterLayingASetWithoutCleanRun(t *testing.T) {
	p := "p1"
	st := classicState(p)
	st.Hands[p] = []string{"7H", "7D", "7C", "2S", "3D"}

	st, _, _, err := ValidateMeldAction(st, p, []string{"7H", "7D", "7C"})
	if err != nil {
		t.Fatalf("set should be a valid meld: %v", err)
	}
	if st.RoundReqMet[p] {
		t.Fatal("a set must not satisfy the clean-run requirement")
	}

	_, _, err = ValidateDiscard(st, p, "2S", nil)
	re, ok := err.(RulesError)
	if !ok || re.Code != ErrIncompleteInitialMeld {
		t.Fatalf("expected INCOMPLETE_INITIAL_MELD, got %#v", err)
	}
	// And it says what is missing, rather than that a rule exists.
	if !strings.Contains(re.Message, "joker-free run") {
		t.Fatalf("the refusal should name the clean run, got %q", re.Message)
	}
}

// The way out of the position above: take the meld back, then discard. This
// is what makes the gate a rule rather than a dead end, so it is pinned.
func TestZolikClassic_UndoingTheLayDownFreesTheDiscard(t *testing.T) {
	p := "p1"
	st := classicState(p)
	st.Hands[p] = []string{"7H", "7D", "7C", "2S", "3D"}

	st, _, _, err := ValidateMeldAction(st, p, []string{"7H", "7D", "7C"})
	if err != nil {
		t.Fatalf("set should be a valid meld: %v", err)
	}

	st, err = ValidateUndoLayMeld(st, p)
	if err != nil {
		t.Fatalf("the meld just laid must be undoable: %v", err)
	}
	if _, _, err := ValidateDiscard(st, p, "2S", nil); err != nil {
		t.Fatalf("discard should be allowed once the lay-down is taken back: %v", err)
	}
}

// Undoing the whole turn is the escape when more than one meld was laid —
// undo:lay_meld only returns the most recent, so it alone is not enough.
func TestZolikClassic_UndoTurnFreesTheDiscardAfterTwoMelds(t *testing.T) {
	p := "p1"
	st := classicState(p)
	st.Hands[p] = []string{"7H", "7D", "7C", "8H", "8D", "8C", "2S"}
	st.Phase = PhaseDraw
	st, _, _, err := ValidateDraw(st, p, DrawFromDeck, "")
	if err != nil {
		t.Fatalf("draw: %v", err)
	}
	for _, m := range [][]string{{"7H", "7D", "7C"}, {"8H", "8D", "8C"}} {
		if st, _, _, err = ValidateMeldAction(st, p, m); err != nil {
			t.Fatalf("laying %v: %v", m, err)
		}
	}
	if _, _, err := ValidateDiscard(st, p, "2S", nil); err == nil {
		t.Fatal("two sets and no clean run must not be discardable")
	}

	st, err = ValidateUndoTurn(st, p)
	if err != nil {
		t.Fatalf("undo turn must be available: %v", err)
	}
	if st.MeldsLaidThisTurn != 0 {
		t.Fatalf("undo turn should clear the turn's melds, got %d", st.MeldsLaidThisTurn)
	}
}

// The clean-run rule is a house rule, and a table may turn it off. With it
// off, sets alone bring a player down and the lay-down gate is satisfied.
func TestZolikClassic_CleanRunRequirementCanBeTurnedOff(t *testing.T) {
	p := "p1"
	st := classicState(p)
	st.Rules.StaticContract.RequireCleanRun = false
	st.Hands[p] = []string{"7H", "7D", "7C", "2S", "3D"}

	st, _, _, err := ValidateMeldAction(st, p, []string{"7H", "7D", "7C"})
	if err != nil {
		t.Fatalf("set should be a valid meld: %v", err)
	}
	if !st.RoundReqMet[p] {
		t.Fatal("with the clean-run rule off, a set should bring the player down")
	}
	if _, _, err := ValidateDiscard(st, p, "2S", nil); err != nil {
		t.Fatalf("a down player may discard: %v", err)
	}
}

// The engine allowing a move means nothing if the client never draws the drop
// target for it. `layOffPlacements` used to filter the joker out of the
// placements it offers on a clean run, so removing the refusal in
// `ValidateLayOff` without removing that filter would have left the move legal
// and unreachable — exactly the offer/engine drift `offers_agreement_test`
// exists to catch, but on a state that corpus does not build.
func TestZolikClassic_TheJokerIsOfferedOntoACleanRun(t *testing.T) {
	p := "p1"
	st := classicState(p)
	st.Hands[p] = []string{"5C", "6C", "7C", "JOKER1", "2S"}

	st, meldID, _, err := ValidateMeldAction(st, p, []string{"5C", "6C", "7C"})
	if err != nil {
		t.Fatalf("clean run should be a valid meld: %v", err)
	}

	o := FindOffer(LegalActions(st, p), LayOffOfferID(meldID))
	if o == nil || !o.Enabled {
		t.Fatalf("expected an enabled lay-off offer on the player's own clean run, got %+v", o)
	}

	var jokerPositions []string
	found := false
	for _, pl := range o.Source.Placements {
		if pl.Card == "JOKER1" {
			jokerPositions, found = pl.Positions, true
		}
	}
	if !found {
		t.Fatalf("the joker must be offered onto the clean run, got placements %+v", o.Source.Placements)
	}
	// Which ends are open is the run's business, not this rule's — what is
	// pinned here is that at least one of them is.
	if len(jokerPositions) == 0 {
		t.Error("the joker is offered with no position to drop it on")
	}
	if !equalStrings(o.Source.Cards, cardsOf(o.Source.Placements)) {
		t.Errorf("Cards %v != cardsOf(Placements) %v", o.Source.Cards, cardsOf(o.Source.Placements))
	}
}

// The load-bearing downstream claim, pinned end-to-end rather than argued in a
// comment: a player who dirties the very run that got them down can still go
// out. `ValidateDiscard` decides that from `RoundReqMet`, and
// `refreshRoundReqMet` only ever sets that flag — but "only ever sets it" is a
// property of code that can be edited, so this asserts the outcome instead.
func TestZolikClassic_GoesOutAfterDirtyingTheRunThatGotThemDown(t *testing.T) {
	p := "p1"
	st := classicState(p)
	st.Hands[p] = []string{"5C", "6C", "7C", "8C", "JOKER1", "2S"}

	st, meldID, _, err := ValidateMeldAction(st, p, []string{"5C", "6C", "7C"})
	if err != nil {
		t.Fatalf("clean run should be a valid meld: %v", err)
	}
	if st, err = ValidateLayOff(st, p, meldID, []string{"8C"}, ""); err != nil {
		t.Fatalf("laying off a natural: %v", err)
	}
	if st, err = ValidateLayOff(st, p, meldID, []string{"JOKER1"}, ""); err != nil {
		t.Fatalf("laying off the joker: %v", err)
	}
	if _, _, hasCleanRun := PlayerMeldCounts(st, p); hasCleanRun {
		t.Fatal("precondition: the run should now carry a joker")
	}

	_, goesOut, err := ValidateDiscard(st, p, "2S", nil)
	if err != nil {
		t.Fatalf("discarding the last card should be allowed: %v", err)
	}
	if !goesOut {
		t.Fatal("the player emptied their hand while down and must go out")
	}
}
