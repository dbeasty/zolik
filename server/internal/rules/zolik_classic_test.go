package rules

import "testing"

func classicState(playerID string) GameState {
	return GameState{
		Status:             StatusActive,
		Rules:              ProfileZolikClassic,
		GameNumber:         1,
		Phase:              PhaseMeld,
		CurrentTurn:        playerID,
		TurnOrder:          []string{playerID, "p2"},
		Hands:              map[string][]string{playerID: {}, "p2": {}},
		Melds:              map[string][][]string{},
		MeldMeta:           map[string][]MeldInfo{},
		RoundReqMet:        map[string]bool{playerID: false, "p2": false},
		InitialMeldMinimum: ProfileZolikClassic.InitialMeldMinimum,
		DrawPile:           []string{"2C"},
		DiscardPile:        []string{},
		DeckSeed:           42,
		GameScores:         map[string][]int{playerID: {}, "p2": {}},
		TotalScores:        map[string]int{playerID: 0, "p2": 0},
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

	_, _, err := ValidateDiscard(st, p, "JOKER1")
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

	_, goOut, err := ValidateDiscard(st, p, "JOKER1")
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
	p := "p1"
	st := classicState(p)
	st.Phase = PhaseDiscard
	st.Hands[p] = []string{"JOKER1"}

	_, _, err := ValidateDiscard(st, p, "JOKER1")
	if err == nil {
		t.Fatal("expected joker discard to still be rejected when not down")
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

// Reproduces the reported table state: an AI went down on a clean club run,
// then kept laying off onto it until the run held two jokers — leaving the
// player "down" with no joker-free run on the table at all. The jokers have
// to go into a separate meld instead.
func TestZolikClassic_LayOffCannotDirtyTheOnlyCleanRun(t *testing.T) {
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

	// Extending it with a joker is not: it would leave no clean run behind.
	_, err = ValidateLayOff(st, p, meldID, []string{"JOKER1"}, "")
	re, ok := err.(RulesError)
	if !ok || re.Code != ErrBreaksCleanRun {
		t.Fatalf("expected BREAKS_CLEAN_RUN when a joker would dirty the only clean run, got %#v", err)
	}

	// The requirement bookkeeping must reflect the table, not the counts
	// recorded when each meld was first laid.
	if _, _, hasCleanRun := PlayerMeldCounts(st, p); !hasCleanRun {
		t.Fatal("5C-6C-7C-8C is still on the table and still clean")
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

// Reproduces the reported bug: laying a set (freely allowed under Classic's
// no-count contract) used to trip Continental's "finish your initial meld
// before discarding" block — but a set can never satisfy the clean-run
// requirement no matter how many more you lay, so a player without a clean
// run in hand this turn could never discard again.
func TestZolikClassic_CanDiscardAfterLayingASetWithoutCleanRun(t *testing.T) {
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

	if _, _, err := ValidateDiscard(st, p, "2S"); err != nil {
		t.Fatalf("expected discard to be allowed after laying a set with no clean run available, got: %v", err)
	}
}
