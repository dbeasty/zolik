package rules

import "testing"

// A preview is only worth having if the number a player sees while choosing
// is the number the server will use when judging. These tests hold it to that
// — the important one being TestPreviewMeld_AgreesWithApplyAction, which is
// the same anti-drift cross-check the offer list gets.

func previewFixture(mut func(*GameState)) GameState {
	s := GameState{
		Status:        StatusActive,
		Rules:         continentalNoFloor(),
		GameNumber:    1,
		Round:         3,
		Phase:         PhaseMeld,
		CurrentTurn:   "p1",
		TurnOrder:     []string{"p1", "p2"},
		DealStarterID: "p1",
		DrawPile:      []string{"2C", "3C", "4C"},
		DiscardPile:   []string{"9C"},
		Hands: map[string][]string{
			"p1": {"QH", "QD", "QC", "5H", "6H", "7H", "8H", "JOKER1", "2D"},
			"p2": {"2H", "3H"},
		},
		Melds:       map[string][][]string{},
		MeldMeta:    map[string][]MeldInfo{},
		RoundReqMet: map[string]bool{},
		GameScores:  map[string][]int{},
		TotalScores: map[string]int{},
	}
	if mut != nil {
		mut(&s)
	}
	return s
}

func TestPreviewMeld_DescribesAValidSetAndRun(t *testing.T) {
	s := previewFixture(nil)

	set := PreviewMeld(s, "p1", []string{"QH", "QD", "QC"})
	if !set.Valid || set.Type != MeldSet {
		t.Fatalf("expected a valid set, got %+v", set)
	}
	if set.NaturalValue != 30 { // three queens at 10
		t.Errorf("set natural value = %d, want 30", set.NaturalValue)
	}
	if set.WildCount != 0 {
		t.Errorf("set wild count = %d, want 0", set.WildCount)
	}

	run := PreviewMeld(s, "p1", []string{"5H", "6H", "7H", "8H"})
	if !run.Valid || run.Type != MeldRun {
		t.Fatalf("expected a valid run, got %+v", run)
	}
	if run.NaturalValue != 26 { // 5+6+7+8
		t.Errorf("run natural value = %d, want 26", run.NaturalValue)
	}
}

func TestPreviewMeld_ExplainsAnInvalidSelectionAndStillPricesIt(t *testing.T) {
	// The readout exists to be watched while a selection is being assembled,
	// so a half-built meld must still report a running total rather than 0 —
	// otherwise the number only appears at the moment it stops being needed.
	p := PreviewMeld(previewFixture(nil), "p1", []string{"QH", "5H"})
	if p.Valid {
		t.Fatal("two unrelated cards are not a meld")
	}
	if p.WhyNot == "" {
		t.Error("an invalid selection should carry the engine's reason")
	}
	if p.NaturalValue != 15 { // Q(10) + 5
		t.Errorf("running value = %d, want 15", p.NaturalValue)
	}
}

func TestPreviewMeld_SeparatesValidFromPlayable(t *testing.T) {
	// "Is this a meld?" and "may I play it?" are different questions, and a
	// UI needs both: a perfectly valid set is unplayable on someone else's
	// turn, and saying so is more useful than greying out with no reason.
	s := previewFixture(func(s *GameState) { s.CurrentTurn = "p2" })
	p := PreviewMeld(s, "p1", []string{"QH", "QD", "QC"})
	if !p.Valid {
		t.Fatal("three queens are a valid set regardless of whose turn it is")
	}
	if p.Playable {
		t.Error("it is not this player's turn, so it is not playable")
	}
	if p.WhyNotPlayable != ErrNotYourTurn {
		t.Errorf("whyNotPlayable = %s, want %s", p.WhyNotPlayable, ErrNotYourTurn)
	}
}

func TestPreviewMeld_MeasuresTheFloorAcrossEverythingLaid(t *testing.T) {
	// The initial-meld floor is a total, not a per-meld test. A candidate
	// that falls short alone can still clear it once what is already on the
	// table counts — the readout has to say so, or a player abandons a legal
	// play.
	s := previewFixture(func(s *GameState) {
		cfg := ProfileContinental
		cfg.InitialMeldMinimum = 50
		s.Rules = cfg
	})

	alone := PreviewMeld(s, "p1", []string{"QH", "QD", "QC"}) // 30, short of 50
	if alone.InitialMeldMinimum != 50 {
		t.Fatalf("floor = %d, want 50", alone.InitialMeldMinimum)
	}
	if alone.MeetsMinimum {
		t.Error("30 points alone should not clear a 50-point floor")
	}

	// Now with a 26-point run already laid: 26 + 30 clears it.
	withTable := previewFixture(func(s *GameState) {
		cfg := ProfileContinental
		cfg.InitialMeldMinimum = 50
		s.Rules = cfg
		s.Melds["p1"] = [][]string{{"5H", "6H", "7H", "8H"}}
		s.MeldMeta["p1"] = []MeldInfo{{MeldID: "meld_1", Type: MeldRun, OwnerID: "p1"}}
		s.NextMeldSeq = 1
		s.Hands["p1"] = []string{"QH", "QD", "QC", "2D"}
	})
	together := PreviewMeld(withTable, "p1", []string{"QH", "QD", "QC"})
	if !together.MeetsMinimum {
		t.Errorf("26 already laid plus 30 should clear a 50-point floor, got %+v", together)
	}
}

func TestPreviewMeld_NoFloorAlwaysMeetsIt(t *testing.T) {
	p := PreviewMeld(previewFixture(nil), "p1", []string{"QH", "QD", "QC"})
	if p.InitialMeldMinimum != 0 || !p.MeetsMinimum {
		t.Errorf("with no floor configured every meld clears it, got %+v", p)
	}
}

func TestPreviewMeld_EmptySelection(t *testing.T) {
	p := PreviewMeld(previewFixture(nil), "p1", nil)
	if p.Valid || p.WhyNot == "" {
		t.Errorf("an empty selection is not a meld and should say so, got %+v", p)
	}
	// The invariant a UI relies on: one field to read, always populated when
	// the answer is no.
	if p.Playable || p.WhyNotPlayable == "" {
		t.Errorf("an empty selection is not playable and should say why, got %+v", p)
	}
}

// TestPreviewMeld_UnplayableAlwaysSaysWhy pins the invariant across the whole
// corpus rather than at one input, since it is what lets a client render a
// reason without branching on validity first.
func TestPreviewMeld_UnplayableAlwaysSaysWhy(t *testing.T) {
	for _, cards := range [][]string{
		nil, {}, {"QH"}, {"QH", "5H"}, {"QH", "QD", "QC"}, {"5H", "6H", "7H", "8H"},
		{"NOTACARD"},
	} {
		for _, mut := range []func(*GameState){
			nil,
			func(s *GameState) { s.CurrentTurn = "p2" },
			func(s *GameState) { s.Phase = PhaseDraw },
			func(s *GameState) { s.Status = StatusCompleted },
		} {
			p := PreviewMeld(previewFixture(mut), "p1", cards)
			if !p.Playable && p.WhyNotPlayable == "" {
				t.Errorf("cards %v: not playable but no reason given (%+v)", cards, p)
			}
			if p.Playable && p.WhyNotPlayable != "" {
				t.Errorf("cards %v: playable but carries a reason %s", cards, p.WhyNotPlayable)
			}
		}
	}
}

// TestPreviewMeld_AgreesWithApplyAction is the anti-drift check: Playable must
// mean the engine actually accepts it, and the reason must be the engine's
// own. Same guarantee the offer list has, for the same reason.
func TestPreviewMeld_AgreesWithApplyAction(t *testing.T) {
	candidates := [][]string{
		{"QH", "QD", "QC"},
		{"5H", "6H", "7H", "8H"},
		{"5H", "6H", "JOKER1", "8H"},
		{"QH", "5H"},
		{"QH"},
		{"5H", "6H", "7H"}, // too short for continental's 4-card runs
		{"QH", "QD", "QC", "5H", "6H", "7H", "8H", "JOKER1", "2D"}, // the whole hand
	}

	for _, sc := range []struct {
		name string
		mut  func(*GameState)
	}{
		{"meld phase, own turn", nil},
		{"opponent's turn", func(s *GameState) { s.CurrentTurn = "p2" }},
		{"draw phase", func(s *GameState) { s.Phase = PhaseDraw }},
		{"suspended", func(s *GameState) { s.Phase = PhaseSuspended; s.Status = StatusSuspended }},
		{"with a floor", func(s *GameState) {
			cfg := ProfileContinental
			cfg.InitialMeldMinimum = 50
			s.Rules = cfg
		}},
		{"zolik_classic", func(s *GameState) { s.Rules = ProfileZolikClassic }},
	} {
		t.Run(sc.name, func(t *testing.T) {
			s := previewFixture(sc.mut)
			for _, cards := range candidates {
				p := PreviewMeld(s, "p1", cards)
				_, err := ApplyAction(cloneState(s), "p1", Action{Type: ActionLayMeld, Cards: cards})

				if p.Playable != (err == nil) {
					t.Errorf("cards %v: preview says playable=%v, engine says %v",
						cards, p.Playable, err)
				}
				if err != nil && p.WhyNotPlayable != codeOf(err) {
					t.Errorf("cards %v: preview reason %s, engine reason %s",
						cards, p.WhyNotPlayable, codeOf(err))
				}
			}
		})
	}
}

// TestPreviewMeld_DoesNotMutate guards the same aliasing hazard
// BuildGameStateMsg has: PreviewMeld dry-runs a real action, so it must clone
// first or previewing a card would quietly remove it from the player's hand.
func TestPreviewMeld_DoesNotMutate(t *testing.T) {
	s := previewFixture(nil)
	before := append([]string(nil), s.Hands["p1"]...)
	meldsBefore := len(s.Melds["p1"])

	for i := 0; i < 5; i++ {
		PreviewMeld(s, "p1", []string{"QH", "QD", "QC"})
	}

	if got := s.Hands["p1"]; len(got) != len(before) {
		t.Fatalf("hand changed from previewing: %v -> %v", before, got)
	}
	for i := range before {
		if s.Hands["p1"][i] != before[i] {
			t.Fatalf("hand changed from previewing: %v -> %v", before, s.Hands["p1"])
		}
	}
	if len(s.Melds["p1"]) != meldsBefore {
		t.Errorf("previewing put a meld on the table")
	}
}
