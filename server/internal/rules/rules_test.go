package rules

import (
	"reflect"
	"testing"
)

func TestValidateSet_WildRatio(t *testing.T) {
	_, err := validateSet([]string{"7H", "JOKER1", "JOKER2"}, 3)
	if err == nil {
		t.Fatalf("expected too many wilds error")
	}
}

func TestValidateSet_DuplicateSuitRejected(t *testing.T) {
	// Even with two physical decks in play, a single set may not reuse a suit.
	_, err := validateSet([]string{"5D", "5C", "5C"}, 3)
	if err == nil {
		t.Fatalf("expected error for duplicate suit within a set")
	}
}

func TestValidateSet_DistinctSuitsAllowed(t *testing.T) {
	mv, err := validateSet([]string{"5D", "5C", "5H"}, 3)
	if err != nil {
		t.Fatalf("expected valid set, got %v", err)
	}
	if mv.Type != MeldSet {
		t.Fatalf("expected set")
	}
}

func TestValidateSet_AceIsNotWild(t *testing.T) {
	// Two natural queens plus an ace is not "three queens" — an ace is a
	// real rank, not a stand-in for a third queen. Only jokers are wild
	// in a set.
	_, err := validateSet([]string{"QH", "QD", "AC"}, 3)
	if err == nil {
		t.Fatalf("expected error: an ace cannot substitute for a third queen in a set")
	}
}

func TestValidateSet_JokerStillWild(t *testing.T) {
	mv, err := validateSet([]string{"QH", "QD", "JOKER1"}, 3)
	if err != nil {
		t.Fatalf("expected joker to complete the set, got %v", err)
	}
	if mv.WildCount != 1 {
		t.Fatalf("expected WildCount 1, got %d", mv.WildCount)
	}
}

func TestValidateSet_AllFourSuitsIsValid(t *testing.T) {
	mv, err := validateSet([]string{"9C", "9D", "9H", "9S"}, 3)
	if err != nil {
		t.Fatalf("expected a complete four-suit set to be valid, got %v", err)
	}
	if mv.NaturalCount != 4 || mv.WildCount != 0 {
		t.Fatalf("expected 4 naturals and 0 wilds, got naturals=%d wilds=%d", mv.NaturalCount, mv.WildCount)
	}
}

func TestValidateSet_WildCannotPadAFullSet(t *testing.T) {
	// Two decks put a second 9 of every suit in play, but a set still only
	// wants one of each — once all four suits are down, a fifth card (wild or
	// not) can't join. Without this cap a set could grow past four using the
	// second deck's jokers, which is a canasta rule this game doesn't have.
	_, err := validateSet([]string{"9C", "9D", "9H", "9S", "JOKER1"}, 3)
	if err == nil {
		t.Fatalf("expected a wild joining a complete four-suit set to be rejected")
	}
	re, ok := err.(RulesError)
	// SET_TOO_LARGE, not TOO_MANY_WILDS: a set of four naturals with a fifth
	// card is refused for its size, and telling a player holding one joker
	// that they have too many jokers is a different rule than the one that
	// stopped them.
	if !ok || re.Code != ErrSetTooLarge {
		t.Fatalf("expected SET_TOO_LARGE, got %v", err)
	}
}

func TestOrderMeldForDisplay_RunSortsAscending(t *testing.T) {
	cards := []string{"6H", "8H", "7H"}
	mv, err := validateRun(cards, 3)
	if err != nil {
		t.Fatalf("expected valid run, got %v", err)
	}
	got := OrderMeldForDisplay(cards, mv)
	want := []string{"6H", "7H", "8H"}
	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected %v, got %v", want, got)
		}
	}
}

func TestValidateMeldAction_StoresRunInSortedOrder(t *testing.T) {
	p := "p1"
	st := baseActiveState(2, p) // Game 2: one set, one run — a run contributes.
	st.Hands[p] = []string{"6H", "9H", "8H", "7H", "2S"}

	st, meldID, _, err := ValidateMeldAction(st, p, []string{"6H", "9H", "8H", "7H"})
	if err != nil {
		t.Fatalf("expected valid meld, got %v", err)
	}
	_ = meldID
	got := st.Melds[p][0]
	want := []string{"6H", "7H", "8H", "9H"}
	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected run stored in sorted order %v, got %v", want, got)
		}
	}
}

func TestOrderMeldForDisplay_SetSortsBySuit(t *testing.T) {
	cards := []string{"QS", "QD", "QH"}
	mv, err := validateSet(cards, 3)
	if err != nil {
		t.Fatalf("expected valid set, got %v", err)
	}
	got := OrderMeldForDisplay(cards, mv)
	want := []string{"QD", "QH", "QS"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected %v, got %v", want, got)
		}
	}
}

func TestValidateRun_AdjacentWildsRejected(t *testing.T) {
	// Ranks 4 and 7 with two consecutive gap fillers (both wild) => invalid.
	_, err := validateRun([]string{"4H", "7H", "JOKER1", "JOKER2"}, 4)
	if err == nil {
		t.Fatalf("expected error for invalid run")
	}
}

func TestValidateRun_AceHighAllowed(t *testing.T) {
	// Q-K-A in suit + wildcard filling J => J-Q-K-A (len 4)
	mv, err := validateRun([]string{"QH", "KH", "AH", "JOKER1"}, 4)
	if err != nil {
		t.Fatalf("expected valid run, got %v", err)
	}
	if mv.Type != MeldRun {
		t.Fatalf("expected run")
	}
}

// A joker may stand in for the ace at either end of a run. It used to be the
// one rank a joker could not be: the endpoint slots demanded a real ace of
// the run's suit, so any window reaching rank 1 or rank 14 was discarded
// before the wild accounting ever saw it.
func TestValidateRun_JokerFillsAceSlot(t *testing.T) {
	cases := []struct {
		name  string
		cards []string
		want  []int // resolved rank window
	}{
		// J-Q-K-A, with jokers as the jack and the ace. The only window that
		// fits: sliding down to 10-J-Q-K puts both jokers side by side at
		// ranks 10 and 11, which the adjacent-wild rule forbids.
		{"joker as the high ace", []string{"QS", "KS", "JOKER1", "JOKER2"}, []int{11, 12, 13, 14}},
		// The mirror image at the bottom: A-2-3-4, jokers as the ace and the
		// four, since 2-3-4-5 would sit both jokers at ranks 4 and 5.
		{"joker as the low ace", []string{"2S", "3S", "JOKER1", "JOKER2"}, []int{1, 2, 3, 4}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mv, err := validateRun(tc.cards, 3)
			if err != nil {
				t.Fatalf("expected valid run, got %v", err)
			}
			if !reflect.DeepEqual(mv.ResolvedRun, tc.want) {
				t.Fatalf("resolved %v, want %v", mv.ResolvedRun, tc.want)
			}
			if mv.WildCount != 2 {
				t.Fatalf("expected 2 wilds, got %d", mv.WildCount)
			}
			// A joker in an ace slot is worth the ace it stands in for.
			if mv.NaturalValue != runValue(tc.want) {
				t.Fatalf("naturalValue = %d, want %d", mv.NaturalValue, runValue(tc.want))
			}
		})
	}
}

// Between two windows that fit the same cards for the same wilds, the more
// valuable one wins — which is what puts a lone joker behind the king rather
// than in front of the queen, since it is worth the card it replaces.
//
// This is the bug as a player meets it: an AI laid Q♠-K♠-JOKER, a 35-point
// ace-high run that clears the 35 floor, and the engine rewrote it as the
// 30-point J♠-Q♠-K♠ and left the player short.
func TestValidateRun_PrefersTheMoreValuableWindow(t *testing.T) {
	cases := []struct {
		name      string
		cards     []string
		want      []int
		wantValue int
	}{
		{"joker goes behind the king, as the ace", []string{"QS", "KS", "JOKER1"}, []int{12, 13, 14}, 10 + 10 + AceMeldValue},
		// The low ace is the one place the ace is cheap (AceRunLowValue),
		// so at the bottom of a run the same preference points the other
		// way: 2-3-4 is worth 9, A-2-3 only 6.
		{"joker takes the four, not the ace below the two", []string{"2S", "3S", "JOKER1"}, []int{2, 3, 4}, 9},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mv, err := validateRun(tc.cards, 3)
			if err != nil {
				t.Fatalf("expected valid run, got %v", err)
			}
			if !reflect.DeepEqual(mv.ResolvedRun, tc.want) {
				t.Fatalf("resolved %v, want %v", mv.ResolvedRun, tc.want)
			}
			if mv.NaturalValue != tc.wantValue {
				t.Fatalf("naturalValue = %d, want %d", mv.NaturalValue, tc.wantValue)
			}
		})
	}
}

// A real ace still beats a joker for the slot it is natural in: the ace is
// worth AceMeldValue there, the joker 0.
func TestValidateRun_RealAcePreferredOverJokerInAceSlot(t *testing.T) {
	mv, err := validateRun([]string{"QH", "KH", "AH", "JOKER1"}, 4)
	if err != nil {
		t.Fatalf("expected valid run, got %v", err)
	}
	if mv.AceAsNatural["AH"] != 1 {
		t.Fatalf("A♥ should hold rank 14 naturally, got %v", mv.AceAsNatural)
	}
	want := 3*10 + AceMeldValue // J-Q-K-A: the joker is the jack, worth 10
	if mv.NaturalValue != want {
		t.Fatalf("natural value %d, want %d", mv.NaturalValue, want)
	}
}

func TestValidateRun_AceBridgeRejected(t *testing.T) {
	// K-A-2 bridge is invalid (cannot wrap).
	_, err := validateRun([]string{"KH", "AH", "2H", "3H"}, 4)
	if err == nil {
		t.Fatalf("expected ace bridge rejection")
	}
}

func TestValidateRun_MismatchedSuitAceNotWild(t *testing.T) {
	// Reproduces a live-game bug: a 4C-5C-JOKER1 run (clubs) let A♦ fill the
	// wild slot as though it were an interchangeable filler, even though an
	// ace can only ever be a natural rank-1/rank-14 endpoint of its own
	// suit — never a stand-in for some other rank, and never in a suit that
	// doesn't match the rest of the run.
	_, err := validateRun([]string{"4C", "5C", "JOKER1", "AD"}, 3)
	if err == nil {
		t.Fatalf("expected mismatched-suit ace to be rejected, not treated as a wild filler")
	}
}

func TestValidateLayOff_MismatchedSuitAceRejected(t *testing.T) {
	// Same bug, exercised through the full lay-off path: an AI laid A♦ off
	// onto its own 4C-5C-JOKER1 run in a live game, which should never have
	// been accepted.
	st := GameState{
		Status:      StatusActive,
		Phase:       PhaseMeld,
		Round:       1,
		CurrentTurn: "p1",
		Hands:       map[string][]string{"p1": {"AD"}},
		Melds:       map[string][][]string{"p1": {{"4C", "5C", "JOKER1"}}},
		MeldMeta:    map[string][]MeldInfo{"p1": {{MeldID: "meld_1", Type: MeldRun, OwnerID: "p1"}}},
		RoundReqMet: map[string]bool{"p1": true},
	}
	if _, err := ValidateLayOff(st, "p1", "meld_1", []string{"AD"}, ""); err == nil {
		t.Fatalf("expected A♦ lay-off onto a clubs run to be rejected")
	}
}

func TestEnsureDrawPile_ReshuffleAndCount(t *testing.T) {
	st := GameState{
		Status:         StatusActive,
		Phase:          PhaseDraw,
		DrawPile:       nil,
		DiscardPile:    []string{"7H", "8H"},
		ReshuffleCount: 0,
		Hands:          map[string][]string{"p1": {}},
		CurrentTurn:    "p1",
	}
	ns, _, _, err := ValidateDraw(st, "p1", DrawFromDeck, "")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if ns.ReshuffleCount != 1 {
		t.Fatalf("expected reshuffleCount=1 got %d", ns.ReshuffleCount)
	}
	if len(ns.DiscardPile) != 0 {
		t.Fatalf("expected discard emptied")
	}
}

func TestEnsureDrawPile_EmptyBothErrors(t *testing.T) {
	st := GameState{
		Status:      StatusActive,
		Phase:       PhaseDraw,
		DrawPile:    nil,
		DiscardPile: nil,
		Hands:       map[string][]string{"p1": {}},
		CurrentTurn: "p1",
	}
	_, _, _, err := ValidateDraw(st, "p1", DrawFromDeck, "")
	if err == nil {
		t.Fatalf("expected error")
	}
	re, ok := err.(RulesError)
	if !ok || re.Code != ErrNoCardsLeft {
		t.Fatalf("expected ErrNoCardsLeft got %#v", err)
	}
}

func TestEndGame_ScoringAndAdvance(t *testing.T) {
	st := GameState{
		Status:      StatusActive,
		GameNumber:  1,
		Phase:       PhaseDiscard,
		TurnOrder:   []string{"p1", "p2"},
		Hands:       map[string][]string{"p1": {"KH"}, "p2": {}},
		RoundReqMet: map[string]bool{"p1": true, "p2": false},
		DeckSeed:    123,
		GameScores:  map[string][]int{"p1": {}, "p2": {}},
		TotalScores: map[string]int{"p1": 0, "p2": 0},
	}

	next, err := EndGame(st, "p2")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if next.GameNumber != 2 {
		t.Fatalf("expected game=2 got %d", next.GameNumber)
	}
	if next.Status != StatusActive {
		t.Fatalf("expected active status")
	}
	// p2 went out, so p2 score = 0; p1 has KH remaining => 10 points.
	if len(next.GameScores["p1"]) != 1 || next.GameScores["p1"][0] != 10 {
		t.Fatalf("expected p1 game score 10 got %#v", next.GameScores["p1"])
	}
	if len(next.GameScores["p2"]) != 1 || next.GameScores["p2"][0] != 0 {
		t.Fatalf("expected p2 game score 0 got %#v", next.GameScores["p2"])
	}
	if next.TotalScores["p1"] != 10 {
		t.Fatalf("expected total score 10 got %d", next.TotalScores["p1"])
	}
}

func TestValidateDraw_DiscardLockedBeforeMinRound(t *testing.T) {
	st := GameState{
		Status:      StatusActive,
		Phase:       PhaseDraw,
		Round:       2,
		DrawPile:    []string{"2H"},
		DiscardPile: []string{"7H"},
		Hands:       map[string][]string{"p1": {}},
		CurrentTurn: "p1",
		Rules:       ProfileContinental, // locks pickup until lap round 3
	}
	_, _, _, err := ValidateDraw(st, "p1", DrawFromDiscard, "")
	if err == nil {
		t.Fatalf("expected discard draw to be locked before round 3")
	}
	re, ok := err.(RulesError)
	if !ok || re.Code != ErrDiscardLocked {
		t.Fatalf("expected ErrDiscardLocked got %#v", err)
	}
	// Drawing from the deck must still work while discard is locked.
	if _, _, _, err := ValidateDraw(st, "p1", DrawFromDeck, ""); err != nil {
		t.Fatalf("expected deck draw to succeed, got %v", err)
	}
}

func TestValidateDraw_DiscardAllowedAtMinRound(t *testing.T) {
	st := GameState{
		Status:      StatusActive,
		Phase:       PhaseDraw,
		Round:       3,
		DrawPile:    []string{"2H"},
		DiscardPile: []string{"7H"},
		Hands:       map[string][]string{"p1": {}},
		CurrentTurn: "p1",
		Rules:       ProfileContinental, // locks pickup until lap round 3
	}
	if _, _, _, err := ValidateDraw(st, "p1", DrawFromDiscard, ""); err != nil {
		t.Fatalf("expected discard draw to be allowed at round 3, got %v", err)
	}
}

func TestValidateDraw_DiscardUnrestrictedByDefault(t *testing.T) {
	st := GameState{
		Status:      StatusActive,
		Phase:       PhaseDraw,
		Round:       1,
		DrawPile:    []string{"2H"},
		DiscardPile: []string{"7H"},
		Hands:       map[string][]string{"p1": {}},
		CurrentTurn: "p1",
		// zolik_classic places no round gate on discard pickup.
		Rules: ProfileZolikClassic,
	}
	if _, _, _, err := ValidateDraw(st, "p1", DrawFromDiscard, ""); err != nil {
		t.Fatalf("expected discard draw to be unrestricted by default, got %v", err)
	}
}

func TestValidateDiscard_BlocksIncompleteInitialMeld(t *testing.T) {
	st := GameState{
		Status:            StatusActive,
		Phase:             PhaseMeld,
		Round:             1,
		CurrentTurn:       "p1",
		Hands:             map[string][]string{"p1": {"9S"}},
		DiscardPile:       []string{},
		RoundReqMet:       map[string]bool{"p1": false},
		MeldsLaidThisTurn: 1,
	}
	_, _, err := ValidateDiscard(st, "p1", "9S", nil)
	if err == nil {
		t.Fatalf("expected discard to be blocked with an incomplete initial meld")
	}
	re, ok := err.(RulesError)
	if !ok || re.Code != ErrIncompleteInitialMeld {
		t.Fatalf("expected ErrIncompleteInitialMeld, got %#v", err)
	}
}

func TestValidateDiscard_DuplicateValueUsesCardIndex(t *testing.T) {
	st := GameState{
		Status:            StatusActive,
		Phase:             PhaseDiscard,
		Round:             1,
		CurrentTurn:       "p1",
		TurnOrder:         []string{"p1", "p2"},
		Hands:             map[string][]string{"p1": {"9S", "4D", "9S"}, "p2": {}},
		DiscardPile:       []string{},
		RoundReqMet:       map[string]bool{"p1": false, "p2": false},
		MeldsLaidThisTurn: 0,
	}
	idx := 2
	ns, _, err := ValidateDiscard(st, "p1", "9S", &idx)
	if err != nil {
		t.Fatalf("expected discard to succeed, got %v", err)
	}
	if got := ns.Hands["p1"]; len(got) != 2 || got[0] != "9S" || got[1] != "4D" {
		t.Fatalf("expected the 9S at index 2 to be removed (leaving the one at index 0), got %v", got)
	}
}

func TestValidateDiscard_AllowedWithNoMeldsLaidThisTurn(t *testing.T) {
	st := GameState{
		Status:            StatusActive,
		Phase:             PhaseMeld,
		Round:             1,
		CurrentTurn:       "p1",
		TurnOrder:         []string{"p1", "p2"},
		Hands:             map[string][]string{"p1": {"9S", "4D"}, "p2": {}},
		DiscardPile:       []string{},
		RoundReqMet:       map[string]bool{"p1": false, "p2": false},
		MeldsLaidThisTurn: 0,
	}
	if _, _, err := ValidateDiscard(st, "p1", "9S", nil); err != nil {
		t.Fatalf("expected discard with no melds laid this turn to succeed, got %v", err)
	}
}

func TestValidateDiscard_AllowedOnceRoundReqMet(t *testing.T) {
	st := GameState{
		Status:            StatusActive,
		Phase:             PhaseMeld,
		Round:             1,
		CurrentTurn:       "p1",
		TurnOrder:         []string{"p1", "p2"},
		Hands:             map[string][]string{"p1": {"9S"}, "p2": {}},
		DiscardPile:       []string{},
		RoundReqMet:       map[string]bool{"p1": true, "p2": false},
		MeldsLaidThisTurn: 2,
	}
	if _, _, err := ValidateDiscard(st, "p1", "9S", nil); err != nil {
		t.Fatalf("expected discard to succeed once round requirement is met, got %v", err)
	}
}

func TestValidateDraw_DiscardPickupSetsPendingMeldBeforeGoingDown(t *testing.T) {
	st := GameState{
		Status:      StatusActive,
		Phase:       PhaseDraw,
		Round:       3,
		DrawPile:    []string{"2H"},
		DiscardPile: []string{"9S"},
		Hands:       map[string][]string{"p1": {}},
		CurrentTurn: "p1",
		RoundReqMet: map[string]bool{"p1": false},
	}
	ns, card, _, err := ValidateDraw(st, "p1", DrawFromDiscard, "")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if card != "9S" || ns.DiscardDrawnCardPendingMeld != "9S" {
		t.Fatalf("expected pending-meld obligation on 9S, got card=%q pending=%q", card, ns.DiscardDrawnCardPendingMeld)
	}
}

func TestValidateDraw_DiscardPickupNoObligationOnceDown(t *testing.T) {
	st := GameState{
		Status:      StatusActive,
		Phase:       PhaseDraw,
		Round:       3,
		DrawPile:    []string{"2H"},
		DiscardPile: []string{"9S"},
		Hands:       map[string][]string{"p1": {}},
		CurrentTurn: "p1",
		RoundReqMet: map[string]bool{"p1": true},
	}
	ns, _, _, err := ValidateDraw(st, "p1", DrawFromDiscard, "")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if ns.DiscardDrawnCardPendingMeld != "" {
		t.Fatalf("expected no pending obligation once already down, got %q", ns.DiscardDrawnCardPendingMeld)
	}
}

func TestValidateDiscard_BlocksWhenPickedUpDiscardCardStillUnmelded(t *testing.T) {
	st := GameState{
		Status:                      StatusActive,
		Phase:                       PhaseMeld,
		Round:                       3,
		CurrentTurn:                 "p1",
		Hands:                       map[string][]string{"p1": {"9S", "4D"}},
		DiscardPile:                 []string{},
		RoundReqMet:                 map[string]bool{"p1": false},
		DiscardDrawnCardPendingMeld: "9S",
	}
	// Trying to discard a *different* card while 9S (the discard pickup)
	// remains unmelded must be rejected.
	_, _, err := ValidateDiscard(st, "p1", "4D", nil)
	if err == nil {
		t.Fatalf("expected discard to be blocked while the picked-up card is unmelded")
	}
	re, ok := err.(RulesError)
	if !ok || re.Code != ErrDiscardCardNotMelded {
		t.Fatalf("expected ErrDiscardCardNotMelded, got %#v", err)
	}
}

func TestValidateMeldAction_UsingPendingDiscardCardClearsObligation(t *testing.T) {
	st := GameState{
		Status:                      StatusActive,
		Phase:                       PhaseMeld,
		GameNumber:                  1,
		CurrentTurn:                 "p1",
		TurnOrder:                   []string{"p1", "p2"},
		Hands:                       map[string][]string{"p1": {"9S", "9D", "9C", "KH", "KD", "KC"}, "p2": {}},
		Melds:                       map[string][][]string{},
		MeldMeta:                    map[string][]MeldInfo{},
		RoundReqMet:                 map[string]bool{"p1": false, "p2": false},
		DiscardDrawnCardPendingMeld: "9S",
	}
	ns, _, _, err := ValidateMeldAction(st, "p1", []string{"9S", "9D", "9C"})
	if err != nil {
		t.Fatalf("unexpected err laying the meld containing the pending card: %v", err)
	}
	if ns.DiscardDrawnCardPendingMeld != "" {
		t.Fatalf("expected pending obligation cleared after melding 9S, got %q", ns.DiscardDrawnCardPendingMeld)
	}
}

func TestValidateLayOff_BlockedBeforeOwnRoundReqMet(t *testing.T) {
	st := GameState{
		Status:      StatusActive,
		Phase:       PhaseMeld,
		Round:       1,
		CurrentTurn: "p1",
		Hands:       map[string][]string{"p1": {"7H"}},
		Melds:       map[string][][]string{"p2": {{"7D", "7C", "7S"}}},
		MeldMeta:    map[string][]MeldInfo{"p2": {{MeldID: "meld_1", Type: MeldSet, OwnerID: "p2"}}},
		RoundReqMet: map[string]bool{"p1": false, "p2": true},
	}
	_, err := ValidateLayOff(st, "p1", "meld_1", []string{"7H"}, "")
	if err == nil {
		t.Fatalf("expected lay-off to be blocked before p1 has met their own round requirement")
	}
	re, ok := err.(RulesError)
	if !ok || re.Code != ErrRoundReqNotMet {
		t.Fatalf("expected ErrRoundReqNotMet, got %#v", err)
	}
}

func TestValidateLayOff_AllowedOnceOwnRoundReqMet(t *testing.T) {
	st := GameState{
		Status:      StatusActive,
		Phase:       PhaseMeld,
		Round:       1,
		CurrentTurn: "p1",
		Hands:       map[string][]string{"p1": {"7H", "4D"}},
		Melds:       map[string][][]string{"p2": {{"7D", "7C", "7S"}}},
		MeldMeta:    map[string][]MeldInfo{"p2": {{MeldID: "meld_1", Type: MeldSet, OwnerID: "p2"}}},
		RoundReqMet: map[string]bool{"p1": true, "p2": true},
	}
	if _, err := ValidateLayOff(st, "p1", "meld_1", []string{"7H"}, ""); err != nil {
		t.Fatalf("expected lay-off to succeed once p1 has met their own round requirement, got %v", err)
	}
}

func TestValidateLayOff_MultiCardInOneAction(t *testing.T) {
	st := GameState{
		Status:      StatusActive,
		Phase:       PhaseMeld,
		Round:       1,
		CurrentTurn: "p1",
		Hands:       map[string][]string{"p1": {"8C", "9C", "2S"}},
		Melds:       map[string][][]string{"p2": {{"5C", "6C", "7C"}}},
		MeldMeta:    map[string][]MeldInfo{"p2": {{MeldID: "meld_1", Type: MeldRun, OwnerID: "p2"}}},
		RoundReqMet: map[string]bool{"p1": true, "p2": true},
	}
	ns, err := ValidateLayOff(st, "p1", "meld_1", []string{"8C", "9C"}, "")
	if err != nil {
		t.Fatalf("expected multi-card lay-off to succeed, got %v", err)
	}
	if got := len(ns.Melds["p2"][0]); got != 5 {
		t.Fatalf("expected the run to grow by both cards, got %d cards: %v", got, ns.Melds["p2"][0])
	}
	if len(ns.Hands["p1"]) != 1 || ns.Hands["p1"][0] != "2S" {
		t.Fatalf("expected only the laid-off cards removed from hand, got %v", ns.Hands["p1"])
	}
}

// Regression test: a multi-card lay-off used to skip the joker-reclaim logic
// entirely (only a single-card drop tried it), so laying off both remaining
// naturals of an already-padded set at once left the jokers stranded in the
// meld — "5C 5D 5H 5S JOKER1 JOKER2" — instead of being swapped back to the
// player's hand the way a single matching card already was.
func TestValidateLayOff_MultiCardReclaimsRedundantJokers(t *testing.T) {
	st := GameState{
		Status:      StatusActive,
		Phase:       PhaseMeld,
		Round:       1,
		CurrentTurn: "p1",
		Hands:       map[string][]string{"p1": {"5H", "5S", "2C"}},
		Melds:       map[string][][]string{"p2": {{"5C", "5D", "JOKER1", "JOKER2"}}},
		MeldMeta:    map[string][]MeldInfo{"p2": {{MeldID: "meld_1", Type: MeldSet, OwnerID: "p2", WildCount: 2}}},
		RoundReqMet: map[string]bool{"p1": true, "p2": true},
	}
	ns, err := ValidateLayOff(st, "p1", "meld_1", []string{"5H", "5S"}, "")
	if err != nil {
		t.Fatalf("expected the multi-card lay-off to succeed, got %v", err)
	}
	got := ns.Melds["p2"][0]
	if len(got) != 4 {
		t.Fatalf("expected 4 natural fives with both jokers reclaimed, got %v", got)
	}
	for _, c := range got {
		if IsJoker(c) {
			t.Fatalf("expected no jokers left padding the set, got %v", got)
		}
	}
	if !containsCard(ns.Hands["p1"], "JOKER1") || !containsCard(ns.Hands["p1"], "JOKER2") {
		t.Fatalf("expected both jokers reclaimed into p1's hand, got %v", ns.Hands["p1"])
	}
	if containsCard(ns.Hands["p1"], "5H") || containsCard(ns.Hands["p1"], "5S") {
		t.Fatalf("expected the laid-off cards to leave p1's hand, got %v", ns.Hands["p1"])
	}
}

// Same reclaim behavior for a run, mixed with an ordinary extension in the
// same action: TD takes JOKER1's exact slot (rank 10) while 7D genuinely
// grows the run's front — both in one multi-card lay-off.
func TestValidateLayOff_MultiCardReclaimsJokerInRun(t *testing.T) {
	st := GameState{
		Status:      StatusActive,
		Phase:       PhaseMeld,
		Round:       1,
		CurrentTurn: "p1",
		Hands:       map[string][]string{"p1": {"TD", "7D"}},
		Melds:       map[string][][]string{"p2": {{"8D", "9D", "JOKER1", "JD"}}},
		MeldMeta:    map[string][]MeldInfo{"p2": {{MeldID: "meld_1", Type: MeldRun, OwnerID: "p2", WildCount: 1}}},
		RoundReqMet: map[string]bool{"p1": true, "p2": true},
	}
	ns, err := ValidateLayOff(st, "p1", "meld_1", []string{"TD", "7D"}, "")
	if err != nil {
		t.Fatalf("expected the multi-card lay-off to succeed, got %v", err)
	}
	got := ns.Melds["p2"][0]
	if len(got) != 5 {
		t.Fatalf("expected a 5-card run (7-8-9-10-J) with the joker reclaimed, got %v", got)
	}
	for _, c := range got {
		if IsJoker(c) {
			t.Fatalf("expected the joker reclaimed out of the run, got %v", got)
		}
	}
	if !containsCard(ns.Hands["p1"], "JOKER1") {
		t.Fatalf("expected the reclaimed joker in p1's hand, got %v", ns.Hands["p1"])
	}
}

// A multi-card lay-off that reclaims a joker must be fully undoable: the
// meld reverts to holding the joker again, and the joker that was moved into
// the player's hand comes back out of it (rather than being left duplicated
// in both the restored meld and the hand).
func TestValidateUndoLayOff_UndoesReclaimedJokersToo(t *testing.T) {
	st := GameState{
		Status:      StatusActive,
		Phase:       PhaseMeld,
		Round:       1,
		CurrentTurn: "p1",
		Hands:       map[string][]string{"p1": {"5H", "5S"}},
		Melds:       map[string][][]string{"p2": {{"5C", "5D", "JOKER1", "JOKER2"}}},
		MeldMeta:    map[string][]MeldInfo{"p2": {{MeldID: "meld_1", Type: MeldSet, OwnerID: "p2", WildCount: 2}}},
		RoundReqMet: map[string]bool{"p1": true, "p2": true},
	}
	ns, err := ValidateLayOff(st, "p1", "meld_1", []string{"5H", "5S"}, "")
	if err != nil {
		t.Fatalf("lay-off failed: %v", err)
	}
	undone, err := ValidateUndoLayOff(ns, "p1")
	if err != nil {
		t.Fatalf("expected undo to succeed, got %v", err)
	}
	got := undone.Melds["p2"][0]
	if len(got) != 4 || !containsCard(got, "JOKER1") || !containsCard(got, "JOKER2") {
		t.Fatalf("expected the meld to revert with both jokers back in place, got %v", got)
	}
	if containsCard(undone.Hands["p1"], "JOKER1") || containsCard(undone.Hands["p1"], "JOKER2") {
		t.Fatalf("expected the reclaimed jokers removed from hand on undo, got %v", undone.Hands["p1"])
	}
	if !containsCard(undone.Hands["p1"], "5H") || !containsCard(undone.Hands["p1"], "5S") {
		t.Fatalf("expected the laid-off cards back in hand, got %v", undone.Hands["p1"])
	}
}

func TestValidateLayOff_PositionMustMatchTheRequestedRunEnd(t *testing.T) {
	base := func() GameState {
		return GameState{
			Status:      StatusActive,
			Phase:       PhaseMeld,
			Round:       1,
			CurrentTurn: "p1",
			Hands:       map[string][]string{"p1": {"4C", "8C"}},
			Melds:       map[string][][]string{"p2": {{"5C", "6C", "7C"}}},
			MeldMeta:    map[string][]MeldInfo{"p2": {{MeldID: "meld_1", Type: MeldRun, OwnerID: "p2"}}},
			RoundReqMet: map[string]bool{"p1": true, "p2": true},
		}
	}

	// 4C only fits at the front (before 5C) — asking for "front" succeeds.
	if _, err := ValidateLayOff(base(), "p1", "meld_1", []string{"4C"}, "front"); err != nil {
		t.Fatalf("expected front lay-off of 4C to succeed, got %v", err)
	}
	// Asking for "end" when the card only fits at the front should be rejected.
	if _, err := ValidateLayOff(base(), "p1", "meld_1", []string{"4C"}, "end"); err == nil {
		t.Fatalf("expected end lay-off of 4C to be rejected")
	} else if re, ok := err.(RulesError); !ok || re.Code != ErrWrongRunEnd {
		t.Fatalf("expected ErrWrongRunEnd, got %v", err)
	}
	// 8C only fits at the end — the mirror case.
	if _, err := ValidateLayOff(base(), "p1", "meld_1", []string{"8C"}, "end"); err != nil {
		t.Fatalf("expected end lay-off of 8C to succeed, got %v", err)
	}
	if _, err := ValidateLayOff(base(), "p1", "meld_1", []string{"8C"}, "front"); err == nil {
		t.Fatalf("expected front lay-off of 8C to be rejected")
	}
}

func TestValidateLayOff_AmbiguousJokerRespectsDroppedEnd(t *testing.T) {
	// A joker laid off onto a 5-6-7 run could equally fill rank 4 (front) or
	// rank 8 (end) — same wild count either way. Regression test: this used
	// to always resolve to the front window, so dropping on "end" was
	// rejected with ErrWrongRunEnd even though the drop was valid.
	base := func() GameState {
		return GameState{
			Status:      StatusActive,
			Phase:       PhaseMeld,
			Round:       1,
			CurrentTurn: "p1",
			Hands:       map[string][]string{"p1": {"JOKER1", "2D"}},
			Melds:       map[string][][]string{"p2": {{"5C", "6C", "7C"}}},
			MeldMeta:    map[string][]MeldInfo{"p2": {{MeldID: "meld_1", Type: MeldRun, OwnerID: "p2"}}},
			RoundReqMet: map[string]bool{"p1": true, "p2": true},
		}
	}

	st, err := ValidateLayOff(base(), "p1", "meld_1", []string{"JOKER1"}, "end")
	if err != nil {
		t.Fatalf("expected end lay-off of joker to succeed, got %v", err)
	}
	got := st.Melds["p2"][0]
	want := []string{"5C", "6C", "7C", "JOKER1"}
	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected joker appended at the end %v, got %v", want, got)
		}
	}

	st2, err := ValidateLayOff(base(), "p1", "meld_1", []string{"JOKER1"}, "front")
	if err != nil {
		t.Fatalf("expected front lay-off of joker to succeed, got %v", err)
	}
	got2 := st2.Melds["p2"][0]
	want2 := []string{"JOKER1", "5C", "6C", "7C"}
	if len(got2) != len(want2) {
		t.Fatalf("expected %v, got %v", want2, got2)
	}
	for i := range want2 {
		if got2[i] != want2[i] {
			t.Fatalf("expected joker prepended at the front %v, got %v", want2, got2)
		}
	}
}

func TestValidateUndoLayOff_RevertsMeldAndReturnsCardToHand(t *testing.T) {
	st := GameState{
		Status:      StatusActive,
		Phase:       PhaseMeld,
		Round:       1,
		CurrentTurn: "p1",
		Hands:       map[string][]string{"p1": {"8C", "2S"}},
		Melds:       map[string][][]string{"p2": {{"5C", "6C", "7C"}}},
		MeldMeta:    map[string][]MeldInfo{"p2": {{MeldID: "meld_1", Type: MeldRun, OwnerID: "p2"}}},
		RoundReqMet: map[string]bool{"p1": true, "p2": true},
	}

	if _, err := ValidateUndoLayOff(st, "p1"); err == nil {
		t.Fatalf("expected nothing to undo before any lay-off happened")
	}

	ns, err := ValidateLayOff(st, "p1", "meld_1", []string{"8C"}, "")
	if err != nil {
		t.Fatalf("lay-off failed: %v", err)
	}

	undone, err := ValidateUndoLayOff(ns, "p1")
	if err != nil {
		t.Fatalf("expected undo to succeed, got %v", err)
	}
	if got := undone.Melds["p2"][0]; len(got) != 3 {
		t.Fatalf("expected the run to revert to 3 cards, got %v", got)
	}
	if got := undone.Hands["p1"]; len(got) != 2 || !containsCard(got, "8C") {
		t.Fatalf("expected 8C back in hand, got %v", got)
	}
	if undone.LastLayOff != nil {
		t.Fatalf("expected the undo window to close after undoing")
	}

	if _, err := ValidateUndoLayOff(undone, "p1"); err == nil {
		t.Fatalf("expected a second undo to fail, nothing left to undo")
	}
}

func TestValidateUndoLayMeld_RevertsMeldAndReturnsCardsToHand(t *testing.T) {
	st := GameState{
		Status:      StatusActive,
		Rules:       ProfileZolikClassic,
		Phase:       PhaseMeld,
		GameNumber:  1,
		Round:       1,
		CurrentTurn: "p1",
		TurnOrder:   []string{"p1", "p2"},
		Hands:       map[string][]string{"p1": {"5H", "6H", "7H", "2S"}, "p2": {}},
		Melds:       map[string][][]string{},
		MeldMeta:    map[string][]MeldInfo{},
		RoundReqMet: map[string]bool{"p1": false, "p2": false},
	}

	if _, err := ValidateUndoLayMeld(st, "p1"); err == nil {
		t.Fatalf("expected nothing to undo before any meld was laid")
	}

	ns, meldID, _, err := ValidateMeldAction(st, "p1", []string{"5H", "6H", "7H"})
	if err != nil {
		t.Fatalf("lay_meld failed: %v", err)
	}
	if !ns.RoundReqMet["p1"] {
		t.Fatalf("expected p1 to be down after laying the required clean run")
	}

	undone, err := ValidateUndoLayMeld(ns, "p1")
	if err != nil {
		t.Fatalf("expected undo to succeed, got %v", err)
	}
	if got := undone.Melds["p1"]; len(got) != 0 {
		t.Fatalf("expected the meld removed from the table, got %v", got)
	}
	if got := undone.Hands["p1"]; len(got) != 4 || !containsCard(got, "5H") || !containsCard(got, "6H") || !containsCard(got, "7H") {
		t.Fatalf("expected all three cards back in hand, got %v", got)
	}
	if undone.RoundReqMet["p1"] {
		t.Fatalf("expected RoundReqMet to revert to false, since it was false before this meld")
	}
	if undone.LastMeldLaid != nil {
		t.Fatalf("expected the undo window to close after undoing")
	}
	if _, idx := findMeldByID(undone, meldID); idx != -1 {
		t.Fatalf("expected the meld to no longer be findable by id after undo")
	}

	if _, err := ValidateUndoLayMeld(undone, "p1"); err == nil {
		t.Fatalf("expected a second undo to fail, nothing left to undo")
	}
}

func TestValidateUndoLayMeld_OnlyMostRecentMeldIsUndoable(t *testing.T) {
	st := GameState{
		Status:      StatusActive,
		Rules:       ProfileZolikClassic,
		Phase:       PhaseMeld,
		GameNumber:  1,
		Round:       1,
		CurrentTurn: "p1",
		TurnOrder:   []string{"p1", "p2"},
		Hands:       map[string][]string{"p1": {"5H", "6H", "7H", "2S", "2D", "2C", "9S"}, "p2": {}},
		Melds:       map[string][][]string{},
		MeldMeta:    map[string][]MeldInfo{},
		RoundReqMet: map[string]bool{"p1": false, "p2": false},
	}

	// First meld doesn't complete shape by itself for a set (only a clean
	// run counts toward zolik_classic's requirement), so it lays freely.
	ns, firstMeldID, _, err := ValidateMeldAction(st, "p1", []string{"2S", "2D", "2C"})
	if err != nil {
		t.Fatalf("first lay_meld failed: %v", err)
	}
	// Second meld (the clean run) is the one that puts p1 down — only it
	// should be in the undo window now, mirroring ValidateUndoLayOff.
	ns, _, _, err = ValidateMeldAction(ns, "p1", []string{"5H", "6H", "7H"})
	if err != nil {
		t.Fatalf("second lay_meld failed: %v", err)
	}

	if _, err := ValidateUndoLayMeld(ns, "p1"); err != nil {
		t.Fatalf("expected the second (most recent) meld to be undoable, got %v", err)
	}
	// findMeldByID confirms the first meld is untouched by the (would-be)
	// undo target — it's still on the table under its original id.
	if owner, idx := findMeldByID(ns, firstMeldID); owner == "" || idx == -1 {
		t.Fatalf("expected the first meld to remain on the table, unaffected by the undo window")
	}
}

func TestValidateUndoTurn_RevertsEverySinceDrawEvenAfterMultipleActions(t *testing.T) {
	st := GameState{
		Status:      StatusActive,
		Rules:       ProfileZolikClassic,
		Phase:       PhaseDraw,
		GameNumber:  1,
		Round:       1,
		CurrentTurn: "p1",
		TurnOrder:   []string{"p1", "p2"},
		DrawPile:    []string{"8H"},
		Hands:       map[string][]string{"p1": {"5H", "6H", "7H", "2S", "2D", "2C", "3S"}, "p2": {}},
		Melds:       map[string][][]string{},
		MeldMeta:    map[string][]MeldInfo{},
		RoundReqMet: map[string]bool{"p1": false, "p2": false},
	}

	if _, err := ValidateUndoTurn(st, "p1"); err == nil {
		t.Fatalf("expected nothing to undo before a draw started the meld phase")
	}

	afterDraw, _, _, err := ValidateDraw(st, "p1", DrawFromDeck, "")
	if err != nil {
		t.Fatalf("draw failed: %v", err)
	}
	postDrawHand := append([]string(nil), afterDraw.Hands["p1"]...)

	// Two melds and a lay-off, so undoing the first meld alone (the
	// last-action-only undo) is no longer possible — only undo_turn can
	// still get back to the start of the meld phase.
	ns, _, _, err := ValidateMeldAction(afterDraw, "p1", []string{"2S", "2D", "2C"})
	if err != nil {
		t.Fatalf("first lay_meld failed: %v", err)
	}
	ns, _, _, err = ValidateMeldAction(ns, "p1", []string{"5H", "6H", "7H"})
	if err != nil {
		t.Fatalf("second lay_meld failed: %v", err)
	}
	ns, err = ValidateLayOff(ns, "p1", findMeldIDByOwnerIndex(ns, "p1", 1), []string{"8H"}, "end")
	if err != nil {
		t.Fatalf("lay_off failed: %v", err)
	}
	if ns.LastMeldLaid != nil {
		t.Fatalf("expected the single-step meld undo window to already be closed by the lay_off")
	}

	undone, err := ValidateUndoTurn(ns, "p1")
	if err != nil {
		t.Fatalf("expected undo_turn to succeed, got %v", err)
	}
	if got := undone.Hands["p1"]; len(got) != len(postDrawHand) {
		t.Fatalf("expected hand restored to its post-draw contents, got %v want %v", got, postDrawHand)
	}
	for _, c := range postDrawHand {
		if !containsCard(undone.Hands["p1"], c) {
			t.Fatalf("expected %s back in hand after undo_turn, got %v", c, undone.Hands["p1"])
		}
	}
	if len(undone.Melds["p1"]) != 0 {
		t.Fatalf("expected both melds removed from the table, got %v", undone.Melds["p1"])
	}
	if undone.RoundReqMet["p1"] {
		t.Fatalf("expected RoundReqMet to revert to false")
	}
	if undone.MeldsLaidThisTurn != 0 {
		t.Fatalf("expected MeldsLaidThisTurn to revert to 0, got %d", undone.MeldsLaidThisTurn)
	}

	// The undo window itself stays open (it's not a one-shot like the
	// single-action undos) — the player can keep melding and undo_turn
	// again, any number of times, up until they discard.
	if _, err := ValidateUndoTurn(undone, "p1"); err != nil {
		t.Fatalf("expected undo_turn to remain available, got %v", err)
	}
}

func findMeldIDByOwnerIndex(state GameState, owner string, idx int) string {
	if metas := state.MeldMeta[owner]; idx < len(metas) {
		return metas[idx].MeldID
	}
	return ""
}

func containsCard(hand []string, card string) bool {
	for _, c := range hand {
		if c == card {
			return true
		}
	}
	return false
}

func TestValidateSwapJoker_RunReplacesJokerAndReturnsItToHand(t *testing.T) {
	st := GameState{
		Status:      StatusActive,
		Phase:       PhaseMeld,
		Round:       1,
		CurrentTurn: "p1",
		Hands:       map[string][]string{"p1": {"7C"}},
		Melds:       map[string][][]string{"p2": {{"5C", "6C", "JOKER1", "8C"}}},
		MeldMeta:    map[string][]MeldInfo{"p2": {{MeldID: "meld_1", Type: MeldRun, OwnerID: "p2", WildCount: 1}}},
		RoundReqMet: map[string]bool{"p1": false, "p2": true},
	}
	ns, err := ValidateSwapJoker(st, "p1", "meld_1", "7C")
	if err != nil {
		t.Fatalf("expected swap to succeed, got %v", err)
	}
	if got := ns.Melds["p2"][0]; len(got) != 4 || got[2] != "7C" {
		t.Fatalf("expected 7C to fill the joker's slot, got %v", got)
	}
	found := false
	for _, c := range ns.Hands["p1"] {
		if c == "JOKER1" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected the joker to land in p1's hand, got %v", ns.Hands["p1"])
	}
	if len(ns.Hands["p1"]) != 1 {
		t.Fatalf("expected hand size unchanged (one card out, one in), got %v", ns.Hands["p1"])
	}
	if meta := ns.MeldMeta["p2"][0]; meta.WildCount != 0 {
		t.Fatalf("expected wild count to drop to 0, got %d", meta.WildCount)
	}
}

func TestValidateSwapJoker_SetReplacesJokerWithMatchingRank(t *testing.T) {
	st := GameState{
		Status:      StatusActive,
		Phase:       PhaseMeld,
		Round:       1,
		CurrentTurn: "p1",
		Hands:       map[string][]string{"p1": {"9H"}},
		Melds:       map[string][][]string{"p2": {{"9C", "9D", "JOKER2"}}},
		MeldMeta:    map[string][]MeldInfo{"p2": {{MeldID: "meld_1", Type: MeldSet, OwnerID: "p2", WildCount: 1}}},
		RoundReqMet: map[string]bool{"p1": false, "p2": true},
	}
	ns, err := ValidateSwapJoker(st, "p1", "meld_1", "9H")
	if err != nil {
		t.Fatalf("expected swap to succeed, got %v", err)
	}
	if got := ns.Melds["p2"][0]; len(got) != 3 {
		t.Fatalf("expected set to keep 3 cards, got %v", got)
	}
	if meta := ns.MeldMeta["p2"][0]; meta.WildCount != 0 {
		t.Fatalf("expected wild count to drop to 0, got %d", meta.WildCount)
	}
}

func TestValidateSwapJoker_WrongCardRejected(t *testing.T) {
	st := GameState{
		Status:      StatusActive,
		Phase:       PhaseMeld,
		Round:       1,
		CurrentTurn: "p1",
		Hands:       map[string][]string{"p1": {"2H"}},
		Melds:       map[string][][]string{"p2": {{"5C", "6C", "JOKER1", "8C"}}},
		MeldMeta:    map[string][]MeldInfo{"p2": {{MeldID: "meld_1", Type: MeldRun, OwnerID: "p2", WildCount: 1}}},
		RoundReqMet: map[string]bool{"p1": false, "p2": true},
	}
	_, err := ValidateSwapJoker(st, "p1", "meld_1", "2H")
	if err == nil {
		t.Fatalf("expected an unrelated card to be rejected")
	}
	re, ok := err.(RulesError)
	if !ok || re.Code != ErrJokerSwapMismatch {
		t.Fatalf("expected JOKER_SWAP_MISMATCH, got %#v", err)
	}
}

func TestValidateSwapJoker_NoJokerInMeldRejected(t *testing.T) {
	st := GameState{
		Status:      StatusActive,
		Phase:       PhaseMeld,
		Round:       1,
		CurrentTurn: "p1",
		Hands:       map[string][]string{"p1": {"9C"}},
		Melds:       map[string][][]string{"p2": {{"5C", "6C", "7C"}}},
		MeldMeta:    map[string][]MeldInfo{"p2": {{MeldID: "meld_1", Type: MeldRun, OwnerID: "p2"}}},
		RoundReqMet: map[string]bool{"p1": false, "p2": true},
	}
	_, err := ValidateSwapJoker(st, "p1", "meld_1", "9C")
	if err == nil {
		t.Fatalf("expected error: meld has no joker to swap")
	}
	re, ok := err.(RulesError)
	if !ok || re.Code != ErrNoJokerInMeld {
		t.Fatalf("expected NO_JOKER_IN_MELD, got %#v", err)
	}
}

// Dragging the exact natural card a joker in a meld stands in for should
// reclaim the joker (a swap), not just pad the meld with a redundant card
// and leave the joker stuck — which is what plain lay-off would otherwise do
// silently (no error, since a set happily accepts naturals >= wilds), even
// though the player's evident intent was to take the joker back.
func TestApplyAction_LayOffPrefersJokerSwapOverRedundantSetAdd(t *testing.T) {
	st := GameState{
		Status:      StatusActive,
		Phase:       PhaseMeld,
		Round:       1,
		CurrentTurn: "p1",
		Hands:       map[string][]string{"p1": {"9H", "2D"}},
		Melds:       map[string][][]string{"p2": {{"9C", "9D", "JOKER2"}}},
		MeldMeta:    map[string][]MeldInfo{"p2": {{MeldID: "meld_1", Type: MeldSet, OwnerID: "p2", WildCount: 1}}},
		RoundReqMet: map[string]bool{"p1": true, "p2": true},
	}
	outcome, err := ApplyAction(st, "p1", Action{Type: ActionLayOff, MeldID: "meld_1", Card: "9H"})
	if err != nil {
		t.Fatalf("expected the drop to resolve as a joker swap, got error: %v", err)
	}
	if got := outcome.State.Melds["p2"][0]; len(got) != 3 || !containsCard(got, "9H") {
		t.Fatalf("expected 9H to take the joker's slot without padding the set, got %v", got)
	}
	if !containsCard(outcome.State.Hands["p1"], "JOKER2") {
		t.Fatalf("expected the joker to land in p1's hand, got %v", outcome.State.Hands["p1"])
	}
	foundSwapEvent := false
	for _, e := range outcome.Events {
		if e.Type == "joker_swapped" {
			foundSwapEvent = true
		}
	}
	if !foundSwapEvent {
		t.Fatalf("expected a joker_swapped event, got %v", outcome.Events)
	}
}

// When the card doesn't fit the joker's own slot but is a perfectly ordinary
// extension of the meld, lay-off still goes through as before — the swap
// attempt is tried first but fails harmlessly and falls through.
func TestApplyAction_LayOffStillWorksWhenNoJokerSwapApplies(t *testing.T) {
	st := GameState{
		Status:      StatusActive,
		Phase:       PhaseMeld,
		Round:       1,
		CurrentTurn: "p1",
		Hands:       map[string][]string{"p1": {"9C", "2D"}},
		Melds:       map[string][][]string{"p2": {{"9H", "9D"}}},
		MeldMeta:    map[string][]MeldInfo{"p2": {{MeldID: "meld_1", Type: MeldSet, OwnerID: "p2"}}},
		RoundReqMet: map[string]bool{"p1": true, "p2": true},
	}
	outcome, err := ApplyAction(st, "p1", Action{Type: ActionLayOff, MeldID: "meld_1", Card: "9C"})
	if err != nil {
		t.Fatalf("expected a normal lay-off to succeed, got error: %v", err)
	}
	if got := outcome.State.Melds["p2"][0]; len(got) != 3 || !containsCard(got, "9C") {
		t.Fatalf("expected 9C added to the set, got %v", got)
	}
}

func TestValidateUndoDrawDiscard_ReturnsCardAndReopensDraw(t *testing.T) {
	st := GameState{
		Status: StatusActive,
		// zolik_classic: pickup is open from lap round 1, so these tests
		// exercise the undo window itself rather than the round gate.
		Rules:       ProfileZolikClassic,
		Phase:       PhaseDraw,
		Round:       1,
		DrawPile:    []string{"2H"},
		DiscardPile: []string{"9S"},
		Hands:       map[string][]string{"p1": {}},
		CurrentTurn: "p1",
		RoundReqMet: map[string]bool{"p1": false},
	}
	ns, card, _, err := ValidateDraw(st, "p1", DrawFromDiscard, "")
	if err != nil {
		t.Fatalf("unexpected err drawing: %v", err)
	}
	if card != "9S" {
		t.Fatalf("expected to draw 9S, got %q", card)
	}

	ns, err = ValidateUndoDrawDiscard(ns, "p1")
	if err != nil {
		t.Fatalf("unexpected err undoing draw: %v", err)
	}
	if ns.Phase != PhaseDraw {
		t.Fatalf("expected phase back to draw, got %v", ns.Phase)
	}
	if len(ns.Hands["p1"]) != 0 {
		t.Fatalf("expected 9S removed from hand, got %v", ns.Hands["p1"])
	}
	if len(ns.DiscardPile) != 1 || ns.DiscardPile[0] != "9S" {
		t.Fatalf("expected 9S back on the discard pile, got %v", ns.DiscardPile)
	}
	if ns.DiscardDrawnCardPendingMeld != "" {
		t.Fatalf("expected pending-meld obligation cleared, got %q", ns.DiscardDrawnCardPendingMeld)
	}
	if len(ns.DiscardDrawnCards) != 0 {
		t.Fatalf("expected DiscardDrawnCards cleared, got %v", ns.DiscardDrawnCards)
	}

	// The player should now be able to draw again, e.g. from the deck.
	if _, _, _, err := ValidateDraw(ns, "p1", DrawFromDeck, ""); err != nil {
		t.Fatalf("expected to be able to draw again after undo, got %v", err)
	}
}

func TestValidateUndoDrawDiscard_NothingToUndoAfterDeckDraw(t *testing.T) {
	st := GameState{
		Status:      StatusActive,
		Phase:       PhaseDraw,
		Round:       1,
		DrawPile:    []string{"2H"},
		DiscardPile: []string{"9S"},
		Hands:       map[string][]string{"p1": {}},
		CurrentTurn: "p1",
	}
	ns, _, _, err := ValidateDraw(st, "p1", DrawFromDeck, "")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	_, err = ValidateUndoDrawDiscard(ns, "p1")
	if err == nil {
		t.Fatalf("expected error undoing a deck draw")
	}
	re, ok := err.(RulesError)
	if !ok || re.Code != ErrNothingToUndo {
		t.Fatalf("expected ErrNothingToUndo, got %#v", err)
	}
}

func TestValidateUndoDrawDiscard_UnavailableAfterLayingMeld(t *testing.T) {
	st := GameState{
		Status: StatusActive,
		// zolik_classic: pickup is open from lap round 1.
		Rules:       ProfileZolikClassic,
		Phase:       PhaseDraw,
		Round:       1,
		DrawPile:    []string{"2H"},
		DiscardPile: []string{"9S"},
		Hands:       map[string][]string{"p1": {"9H", "9D", "2C"}},
		CurrentTurn: "p1",
		RoundReqMet: map[string]bool{"p1": true},
	}
	ns, _, _, err := ValidateDraw(st, "p1", DrawFromDiscard, "")
	if err != nil {
		t.Fatalf("unexpected err drawing: %v", err)
	}
	ns, _, _, err = ValidateMeldAction(ns, "p1", []string{"9H", "9D", "9S"})
	if err != nil {
		t.Fatalf("unexpected err melding: %v", err)
	}
	if _, err := ValidateUndoDrawDiscard(ns, "p1"); err == nil {
		t.Fatalf("expected undo to be unavailable once the drawn card has been melded")
	}
}

func TestValidateUndoDrawDiscard_WrongPlayerOrPhaseRejected(t *testing.T) {
	st := GameState{
		Status: StatusActive,
		// zolik_classic: pickup is open from lap round 1, so these tests
		// exercise the undo window itself rather than the round gate.
		Rules:       ProfileZolikClassic,
		Phase:       PhaseDraw,
		Round:       1,
		DrawPile:    []string{"2H"},
		DiscardPile: []string{"9S"},
		Hands:       map[string][]string{"p1": {}},
		CurrentTurn: "p1",
		RoundReqMet: map[string]bool{"p1": false},
	}
	ns, _, _, err := ValidateDraw(st, "p1", DrawFromDiscard, "")
	if err != nil {
		t.Fatalf("unexpected err drawing: %v", err)
	}
	if _, err := ValidateUndoDrawDiscard(ns, "p2"); err == nil {
		t.Fatalf("expected error when a different player tries to undo")
	}
}
