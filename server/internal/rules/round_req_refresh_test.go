package rules

import "testing"

// wedgedGameState is the shape game 6a8aa17f… was found in: a player whose
// table plainly satisfies everything needed to be down, carrying a
// RoundReqMet flag that says otherwise because nothing re-derived it when an
// opponent's lay-off grew their meld past the point floor.
//
// Deliberately ace-free. The real game's melds included a set of aces, whose
// value is its own rule question (see TestAceInASetScoresAsARealAce) — this
// fixture exists to pin the *flag* behaviour, so its arithmetic is kept
// independent of that: a clean run worth 14 plus a set of 6s worth 18 or 24
// straddles the 35-point floor either side of one lay-off.
func wedgedGameState() GameState {
	cfg := ProfileZolikClassic
	cfg.InitialMeldMinimum = 35
	return GameState{
		Status: StatusActive, Rules: cfg, GameNumber: 3, Phase: PhaseMeld,
		CurrentTurn: "human", TurnOrder: []string{"human", "ai"}, Round: 15,
		Hands: map[string][]string{"human": {"JC", "7D", "JOKER1"}, "ai": {"6S", "2C"}},
		Melds: map[string][][]string{
			"ai":    {{"7C", "8C", "9C", "TC"}},
			"human": {{"2D", "3D", "4D", "5D"}, {"6C", "6D", "6H", "6S"}},
		},
		MeldMeta: map[string][]MeldInfo{
			"ai": {{MeldID: "meld_2", Type: MeldRun, OwnerID: "ai"}},
			"human": {
				{MeldID: "meld_3", Type: MeldRun, OwnerID: "human"},
				{MeldID: "meld_4", Type: MeldSet, OwnerID: "human"},
			},
		},
		RoundReqMet: map[string]bool{"human": false, "ai": true},
		NextMeldSeq: 4,
		DrawPile:    []string{"KD", "QS"},
		DiscardPile: []string{"9H"},
	}
}

// beforeTheAILaidOff rewinds the human's set of 6s to the three cards it held
// before the AI extended it: 32 points, short of the floor, correctly not down.
func beforeTheAILaidOff() GameState {
	st := wedgedGameState()
	st.Melds["human"][1] = []string{"6C", "6D", "6H"}
	st.CurrentTurn = "ai"
	return st
}

// A lay-off onto a not-yet-down player's meld must re-derive that player's
// down-status: the cards it adds can be what carries them over the contract
// or the point floor.
func TestLayOffRefreshesOwnerRoundReqMet(t *testing.T) {
	st := beforeTheAILaidOff()
	if got := PlayerInitialMeldNaturalValue(st, "human"); got != 32 {
		t.Fatalf("setup: human table value = %d, want 32", got)
	}
	if st.RoundReqMet["human"] {
		t.Fatal("setup: human should not be down at 32 points")
	}

	next, err := ValidateLayOff(st, "ai", "meld_4", []string{"6S"}, "")
	if err != nil {
		t.Fatalf("AI lay-off onto the human's set: %v", err)
	}
	if got := PlayerInitialMeldNaturalValue(next, "human"); got != 38 {
		t.Fatalf("human table value after the lay-off = %d, want 38", got)
	}
	if !next.RoundReqMet["human"] {
		t.Error("human clears the 35-point floor with the contract met, but RoundReqMet is still false")
	}
}

// The wedge itself: the human qualifies on every count the engine can check,
// so the flag being false is the only thing refusing their lay-offs.
func TestWedgedPlayerQualifiesAndCanLayOff(t *testing.T) {
	st := wedgedGameState()
	if !PlayerMeetsRoundRequirement(st, "human") {
		t.Fatal("human should meet the contract")
	}
	if !PlayerIsDown(st, "human") {
		t.Fatalf("human should qualify as down: value %d, floor %d",
			PlayerInitialMeldNaturalValue(st, "human"), effectiveRules(st).InitialMeldMinimum)
	}

	st.RoundReqMet["human"] = true
	if _, err := ValidateLayOff(st, "human", "meld_2", []string{"JC"}, "end"); err != nil {
		t.Fatalf("lay-off onto the opponent's run: %v", err)
	}
}

// Undoing a lay-off that put the meld's owner down has to take that back too,
// or a lay-off and its undo hand an opponent a free trip down.
func TestUndoLayOffRestoresOwnerRoundReqMet(t *testing.T) {
	next, err := ValidateLayOff(beforeTheAILaidOff(), "ai", "meld_4", []string{"6S"}, "")
	if err != nil {
		t.Fatalf("lay-off: %v", err)
	}
	if !next.RoundReqMet["human"] {
		t.Fatal("precondition: the lay-off should have put the human down")
	}

	back, err := ValidateUndoLayOff(next, "ai")
	if err != nil {
		t.Fatalf("undo lay-off: %v", err)
	}
	if back.RoundReqMet["human"] {
		t.Error("the lay-off was undone, but the human stayed down for free")
	}
}

// undo_turn rolls back the whole meld phase, including a lay-off that put
// someone other than the acting player down.
func TestUndoTurnRestoresEveryPlayersRoundReqMet(t *testing.T) {
	st := beforeTheAILaidOff()
	st.Phase = PhaseDraw

	drawn, _, _, err := ValidateDraw(st, "ai", DrawFromDeck, "")
	if err != nil {
		t.Fatalf("draw: %v", err)
	}
	afterLayOff, err := ValidateLayOff(drawn, "ai", "meld_4", []string{"6S"}, "")
	if err != nil {
		t.Fatalf("lay-off: %v", err)
	}
	if !afterLayOff.RoundReqMet["human"] {
		t.Fatal("precondition: the lay-off should have put the human down")
	}

	back, err := ValidateUndoTurn(afterLayOff, "ai")
	if err != nil {
		t.Fatalf("undo turn: %v", err)
	}
	if back.RoundReqMet["human"] {
		t.Error("undo_turn rolled the lay-off back but left the human down")
	}
}

// A game persisted by a build that never re-derived the flag carries it
// frozen. ApplyAction heals it before any gate reads it, so such a game
// recovers on the wedged player's very next action.
//
// The heal has to sit at the entry point, not in ValidateDraw: game
// 6a8aa17f… was suspended mid-turn with the player already drawn and in the
// meld phase, where a draw-time heal never runs. Their next action is the
// lay_off itself, and it has to work.
func TestApplyActionHealsAStaleRoundReqMetMidTurn(t *testing.T) {
	st := wedgedGameState() // PhaseMeld: the player has already drawn.
	if st.RoundReqMet["human"] {
		t.Fatal("setup: the flag should start stale-false")
	}

	out, err := ApplyAction(st, "human", Action{
		Type: ActionLayOff, MeldID: "meld_2", Cards: []string{"JC"}, Position: "end",
	})
	if err != nil {
		t.Fatalf("lay-off mid-turn with a stale flag: %v", err)
	}
	if !out.State.RoundReqMet["human"] {
		t.Error("the stale flag should have been healed")
	}
}

// The offer list has to agree, or the client renders every meld as an inert
// drop target however willing the engine is underneath.
func TestOffersEnabledForAStaleRoundReqMet(t *testing.T) {
	st := wedgedGameState()
	for _, o := range LegalActions(st, "human") {
		if o.ID != LayOffOfferID("meld_2") {
			continue
		}
		if !o.Enabled {
			t.Fatalf("lay_off:meld_2 offer disabled with whyNot=%q", o.WhyNot)
		}
		return
	}
	t.Fatal("no lay_off offer for meld_2")
}

// Drawing heals too — the same entry point serves the player who reconnects
// at the top of their turn rather than mid-way through it.
func TestDrawHealsAStaleRoundReqMet(t *testing.T) {
	st := wedgedGameState()
	st.Phase = PhaseDraw

	out, err := ApplyAction(st, "human", Action{Type: ActionDrawCard, DrawFrom: DrawFromDeck})
	if err != nil {
		t.Fatalf("draw: %v", err)
	}
	if !out.State.RoundReqMet["human"] {
		t.Fatal("drawing should have healed the stale flag")
	}
	if _, err := ValidateLayOff(out.State, "human", "meld_2", []string{"JC"}, "end"); err != nil {
		t.Errorf("lay-off onto the opponent's run still refused: %v", err)
	}
}

// A player who is genuinely short of the point floor is told so, rather than
// being told to lay an initial meld that is already on the table.
func TestNotDownErrorNamesThePointFloor(t *testing.T) {
	st := beforeTheAILaidOff() // 32 points against a 35 floor.
	st.CurrentTurn = "human"

	_, err := ValidateLayOff(st, "human", "meld_2", []string{"JC"}, "end")
	re, ok := err.(RulesError)
	if !ok {
		t.Fatalf("want RulesError, got %#v", err)
	}
	if re.Code != ErrMeldBelowMinimum {
		t.Errorf("code = %q, want %q (message: %s)", re.Code, ErrMeldBelowMinimum, re.Message)
	}

	// With nothing laid at all, the original instruction is the honest one.
	bare := wedgedGameState()
	bare.Melds["human"] = nil
	bare.MeldMeta["human"] = nil
	_, err = ValidateLayOff(bare, "human", "meld_2", []string{"JC"}, "end")
	if re, ok := err.(RulesError); !ok || re.Code != ErrRoundReqNotMet {
		t.Errorf("with no melds laid, want %q, got %#v", ErrRoundReqNotMet, err)
	}
}

// Going down is permanent for the deal — a refresh must never take a player
// back up, whatever the table looks like afterwards.
func TestRefreshRoundReqMetNeverUndoesBeingDown(t *testing.T) {
	st := wedgedGameState()
	st.RoundReqMet["human"] = true
	// A table that would not qualify on its own.
	st.Melds["human"] = [][]string{{"2D", "3D", "4D", "5D"}}
	st.MeldMeta["human"] = []MeldInfo{{MeldID: "meld_3", Type: MeldRun, OwnerID: "human"}}

	refreshRoundReqMet(st, "human")
	if !st.RoundReqMet["human"] {
		t.Error("a player who was already down was taken back up")
	}
}

// A joker swap can turn a wild-carrying run into the joker-free one the
// contract demands, which is the other route to going down that used to be
// missed.
func TestSwapJokerRefreshesOwnerRoundReqMet(t *testing.T) {
	cfg := ProfileZolikClassic // StaticContract requires a clean run.
	st := GameState{
		Status: StatusActive, Rules: cfg, GameNumber: 1, Phase: PhaseMeld,
		CurrentTurn: "human", TurnOrder: []string{"human", "ai"}, Round: 2,
		Hands: map[string][]string{"human": {"4H", "QS", "KD"}, "ai": {"2C", "9S"}},
		Melds: map[string][][]string{
			"human": {{"3H", "JOKER1", "5H"}},
		},
		MeldMeta: map[string][]MeldInfo{
			"human": {{MeldID: "meld_1", Type: MeldRun, OwnerID: "human", WildCount: 1}},
		},
		RoundReqMet: map[string]bool{"human": false, "ai": false},
		NextMeldSeq: 1,
		DrawPile:    []string{"KD", "QS"},
		DiscardPile: []string{"9H"},
	}
	if PlayerMeetsRoundRequirement(st, "human") {
		t.Fatal("setup: a run carrying a joker should not satisfy the clean-run contract")
	}

	next, err := ValidateSwapJoker(st, "human", "meld_1", "4H")
	if err != nil {
		t.Fatalf("swap joker: %v", err)
	}
	if !IsCleanRun(next.Melds["human"][0], ResolveConfig(cfg)) {
		t.Fatalf("run should be clean after the swap, got %v", next.Melds["human"][0])
	}
	if !next.RoundReqMet["human"] {
		t.Error("the swap made the run joker-free, but RoundReqMet is still false")
	}
}
