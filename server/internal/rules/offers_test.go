package rules

import (
	"testing"
)

// The agreement test (offers_agreement_test.go) proves LegalActions can't
// disagree with the engine. These tests pin the parts a client actually
// reads and the agreement test cannot see: that the offer set is complete
// and stably ordered, that disabled offers carry a usable reason, and that
// the card lists are scoped to the viewer.

func offerFixture(mut func(*GameState)) GameState {
	s := GameState{
		Status:        StatusActive,
		Rules:         continentalNoFloor(),
		GameNumber:    1,
		Round:         3, // past continental's discard lock
		Phase:         PhaseMeld,
		CurrentTurn:   "p1",
		TurnOrder:     []string{"p1", "p2"},
		DealStarterID: "p1",
		DrawPile:      []string{"2C", "3C", "4C"},
		DiscardPile:   []string{"9C", "TC"},
		Hands: map[string][]string{
			"p1": {"4H", "9H", "KS", "KD"},
			"p2": {"2D", "3D"},
		},
		Melds: map[string][][]string{
			"p2": {{"5H", "6H", "7H", "8H"}},
		},
		MeldMeta: map[string][]MeldInfo{
			"p2": {{MeldID: "meld_1", Type: MeldRun, OwnerID: "p2"}},
		},
		RoundReqMet: map[string]bool{"p1": true},
		GameScores:  map[string][]int{},
		TotalScores: map[string]int{},
		NextMeldSeq: 1,
	}
	if mut != nil {
		mut(&s)
	}
	return s
}

func TestLegalActions_AlwaysReturnsTheCompleteSet(t *testing.T) {
	// A client renders controls from this list, so a missing offer and a
	// disabled offer must never be confusable: the full set ships every
	// time, whatever the state.
	want := []string{
		OfferDrawDeck, OfferDrawDiscard, OfferLayMeld,
		LayOffOfferID("meld_1"), SwapJokerOfferID("meld_1"),
		OfferDiscard,
		OfferUndoDrawDiscard, OfferUndoLayOff, OfferUndoLayMeld, OfferUndoTurn,
	}

	for _, tc := range []struct {
		name string
		mut  func(*GameState)
	}{
		{"active turn", nil},
		{"opponent's turn", func(s *GameState) { s.CurrentTurn = "p2" }},
		{"suspended", func(s *GameState) { s.Phase = PhaseSuspended; s.Status = StatusSuspended }},
		{"completed", func(s *GameState) { s.Status = StatusCompleted }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			offers := LegalActions(offerFixture(tc.mut), "p1")
			for _, id := range want {
				if FindOffer(offers, id) == nil {
					t.Errorf("offer %q missing from the set", id)
				}
			}
			if len(offers) != len(want) {
				t.Errorf("got %d offers, want %d", len(offers), len(want))
			}
		})
	}
}

func TestLegalActions_EveryDisabledOfferSaysWhy(t *testing.T) {
	// "Greyed out with no reason" is the failure mode this whole mechanism
	// exists to remove; a disabled offer with an empty WhyNot would put the
	// client right back to guessing.
	for _, pid := range []string{"p1", "p2"} {
		for _, o := range LegalActions(offerFixture(nil), pid) {
			if !o.Enabled && o.WhyNot == "" {
				t.Errorf("%s: offer %q is disabled with no reason", pid, o.ID)
			}
			if o.Enabled && o.WhyNot != "" {
				t.Errorf("%s: offer %q is enabled but carries whyNot=%s", pid, o.ID, o.WhyNot)
			}
		}
	}
}

func TestLegalActions_StableOrderAcrossCalls(t *testing.T) {
	// Melds live in a Go map, whose iteration order is randomised. An offer
	// list that reshuffles between pushes makes controls jump around and
	// makes tests flake, so the order has to be imposed.
	s := offerFixture(func(s *GameState) {
		s.Melds["p1"] = [][]string{{"QH", "QD", "QC"}}
		s.MeldMeta["p1"] = []MeldInfo{{MeldID: "meld_2", Type: MeldSet, OwnerID: "p1"}}
		s.Melds["p3"] = [][]string{{"JH", "JD", "JC"}}
		s.MeldMeta["p3"] = []MeldInfo{{MeldID: "meld_3", Type: MeldSet, OwnerID: "p3"}}
	})

	first := LegalActions(s, "p1")
	for i := 0; i < 50; i++ {
		got := LegalActions(s, "p1")
		if len(got) != len(first) {
			t.Fatalf("offer count changed between calls: %d vs %d", len(got), len(first))
		}
		for j := range got {
			if got[j].ID != first[j].ID {
				t.Fatalf("offer order changed at %d: %q vs %q", j, got[j].ID, first[j].ID)
			}
		}
	}
}

func TestLegalActions_LeaksNoOtherPlayersCards(t *testing.T) {
	// Offers are computed per viewer and broadcast per viewer. A card list
	// built from someone else's hand would be a hidden-information leak
	// through a brand-new channel, so assert the negative directly.
	s := offerFixture(nil)
	for _, o := range LegalActions(s, "p1") {
		for _, sel := range []*Selector{o.Source, o.Target} {
			if sel == nil || sel.Zone != ZoneHand {
				continue
			}
			for _, c := range sel.Cards {
				if !containsString(s.Hands["p1"], c) {
					t.Errorf("offer %q lists card %q that is not in p1's hand", o.ID, c)
				}
			}
		}
	}

	// The opponent's view carries no card lists at all — there is nothing
	// for them to act on, so there is nothing to send.
	for _, o := range LegalActions(s, "p2") {
		if o.Source != nil && o.Source.Zone == ZoneHand && len(o.Source.Cards) > 0 {
			t.Errorf("offer %q sent card list %v to a player whose turn it is not", o.ID, o.Source.Cards)
		}
	}
}

func TestLegalActions_LayOffHintsWhichEndOfARun(t *testing.T) {
	// p2's run is 5H-6H-7H-8H; p1 holds 4H (front only) and 9H (end only).
	offers := LegalActions(offerFixture(nil), "p1")
	o := FindOffer(offers, LayOffOfferID("meld_1"))
	if o == nil || !o.Enabled {
		t.Fatalf("expected an enabled lay-off offer, got %+v", o)
	}

	got := map[string][]string{}
	for _, p := range o.Source.Placements {
		got[p.Card] = p.Positions
	}
	if want := []string{"front"}; !equalStrings(got["4H"], want) {
		t.Errorf("4H positions = %v, want %v", got["4H"], want)
	}
	if want := []string{"end"}; !equalStrings(got["9H"], want) {
		t.Errorf("9H positions = %v, want %v", got["9H"], want)
	}
	if _, ok := got["KS"]; ok {
		t.Errorf("KS does not extend a heart run but was offered: %v", o.Source.Placements)
	}
	// Cards mirrors Placements exactly — a simple client reads one, a
	// drag-and-drop client reads the other, and they cannot disagree.
	if !equalStrings(o.Source.Cards, cardsOf(o.Source.Placements)) {
		t.Errorf("Cards %v != cardsOf(Placements) %v", o.Source.Cards, cardsOf(o.Source.Placements))
	}
}

func TestLegalActions_LayOffBlockedBeforeGoingDown(t *testing.T) {
	// The rule a client currently re-derives as
	// `canLayOff = isMyTurn && phase==='meld' && roundReqMet[me]`. It now
	// arrives as an answer, with the engine's own reason attached.
	s := offerFixture(func(s *GameState) { s.RoundReqMet["p1"] = false })
	o := FindOffer(LegalActions(s, "p1"), LayOffOfferID("meld_1"))
	if o.Enabled {
		t.Fatalf("lay-off should be unavailable before going down")
	}
	if o.WhyNot != ErrRoundReqNotMet {
		t.Errorf("whyNot = %s, want %s", o.WhyNot, ErrRoundReqNotMet)
	}
}

func TestLegalActions_DiscardLockIsReportedAsSuch(t *testing.T) {
	// The other expression clients re-derive:
	// `discardLocked = discardDrawMinRound > 1 && round < discardDrawMinRound`.
	s := offerFixture(func(s *GameState) {
		s.Phase = PhaseDraw
		s.Round = 1 // continental locks the pile until round 3
	})
	o := FindOffer(LegalActions(s, "p1"), OfferDrawDiscard)
	if o.Enabled {
		t.Fatalf("discard pickup should be locked in round 1 under continental")
	}
	if o.WhyNot != ErrDiscardLocked {
		t.Errorf("whyNot = %s, want %s", o.WhyNot, ErrDiscardLocked)
	}
	if len(o.Source.Cards) != 0 {
		t.Errorf("a locked pile should offer no cards, got %v", o.Source.Cards)
	}
}

func TestLegalActions_PileCardsFollowThePickupMode(t *testing.T) {
	// A profile knob, answered by the server rather than by a client
	// switching on the profile *name*: top_only offers the top card,
	// any_from_pile offers the whole pile.
	t.Run("top_only", func(t *testing.T) {
		s := offerFixture(func(s *GameState) {
			s.Phase = PhaseDraw
			s.DiscardPile = []string{"9C", "TC", "JC"}
		})
		o := FindOffer(LegalActions(s, "p1"), OfferDrawDiscard)
		if !equalStrings(o.Source.Cards, []string{"JC"}) {
			t.Errorf("top_only cards = %v, want [JC]", o.Source.Cards)
		}
	})

	t.Run("any_from_pile", func(t *testing.T) {
		s := offerFixture(func(s *GameState) {
			s.Rules = ProfileZolikClassic
			s.Phase = PhaseDraw
			s.DiscardPile = []string{"9C", "TC", "JC"}
		})
		o := FindOffer(LegalActions(s, "p1"), OfferDrawDiscard)
		if !equalStrings(o.Source.Cards, []string{"9C", "TC", "JC"}) {
			t.Errorf("any_from_pile cards = %v, want the whole pile", o.Source.Cards)
		}
	})
}

func TestLegalActions_DiscardExcludesAnUnplayableJoker(t *testing.T) {
	// A joker can't be discarded unless it ends the hand — the rule
	// MeldTable.tsx currently approximates with `startsWith('JOKER')`.
	s := offerFixture(func(s *GameState) {
		s.Rules = ProfileZolikClassic
		s.Hands["p1"] = []string{"4H", "JOKER1"}
	})
	o := FindOffer(LegalActions(s, "p1"), OfferDiscard)
	if !o.Enabled {
		t.Fatalf("discard should be available: 4H is discardable")
	}
	if containsString(o.Source.Cards, "JOKER1") {
		t.Errorf("joker offered as a discard with cards still in hand: %v", o.Source.Cards)
	}

	// ...but the same joker as the last card, already down, ends the deal
	// and is therefore discardable.
	s2 := offerFixture(func(s *GameState) {
		s.Rules = ProfileZolikClassic
		s.Hands["p1"] = []string{"JOKER1"}
	})
	o2 := FindOffer(LegalActions(s2, "p1"), OfferDiscard)
	if !containsString(o2.Source.Cards, "JOKER1") {
		t.Errorf("the joker that ends the hand should be discardable, got %v", o2.Source.Cards)
	}
}

func TestLegalActions_SwapJokerOnlyWhereAJokerSits(t *testing.T) {
	s := offerFixture(func(s *GameState) {
		s.Melds["p2"] = [][]string{
			{"5H", "6H", "7H", "8H"},
			{"2S", "3S", "JOKER2", "5S"},
		}
		s.MeldMeta["p2"] = []MeldInfo{
			{MeldID: "meld_1", Type: MeldRun, OwnerID: "p2"},
			{MeldID: "meld_2", Type: MeldRun, OwnerID: "p2", WildCount: 1},
		}
		s.Hands["p1"] = []string{"4S", "KS"}
		s.NextMeldSeq = 2
	})
	offers := LegalActions(s, "p1")

	if o := FindOffer(offers, SwapJokerOfferID("meld_1")); o.Enabled {
		t.Errorf("joker swap offered on a joker-free meld")
	} else if o.WhyNot != ErrNoJokerInMeld {
		t.Errorf("whyNot = %s, want %s", o.WhyNot, ErrNoJokerInMeld)
	}

	o := FindOffer(offers, SwapJokerOfferID("meld_2"))
	if !o.Enabled {
		t.Fatalf("4S takes the joker's slot in 2S-3S-JOKER, expected an enabled offer")
	}
	if !equalStrings(o.Source.Cards, []string{"4S"}) {
		t.Errorf("swap cards = %v, want [4S] only", o.Source.Cards)
	}
}

func TestLegalActions_UndoWindowsMatchTheSnapshots(t *testing.T) {
	// These four replace the ad-hoc canUndo* booleans the wire has been
	// carrying — the three-times-patched symptom of the missing offer list.
	for _, tc := range []struct {
		name    string
		offerID string
		mut     func(*GameState)
	}{
		{"draw-discard", OfferUndoDrawDiscard, func(s *GameState) {
			// The undo returns these cards to the pile, so they have to be
			// in hand for it to be available at all.
			s.Hands["p1"] = []string{"4H", "9H", "KS", "TC"}
			s.DiscardDrawnCards = []string{"TC"}
		}},
		{"lay-off", OfferUndoLayOff, func(s *GameState) {
			s.LastLayOff = &LayOffSnapshot{
				PlayerID: "p1", MeldID: "meld_1",
				PrevCards: []string{"5H", "6H", "7H", "8H"},
				Cards:     []string{"9H"},
			}
			s.Hands["p1"] = []string{"4H", "KS"}
		}},
		{"lay-meld", OfferUndoLayMeld, func(s *GameState) {
			s.LastMeldLaid = &MeldLaidSnapshot{
				PlayerID: "p1", MeldID: "meld_1",
				Cards: []string{"5H", "6H", "7H", "8H"},
			}
		}},
		{"turn", OfferUndoTurn, func(s *GameState) {
			s.TurnMeldSnapshot = &TurnMeldSnapshot{
				PlayerID: "p1",
				Hands:    map[string][]string{"p1": {"4H"}},
				Melds:    map[string][][]string{}, MeldMeta: map[string][]MeldInfo{},
				DiscardPile: []string{},
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Nothing to undo yet.
			if o := FindOffer(LegalActions(offerFixture(nil), "p1"), tc.offerID); o.Enabled {
				t.Errorf("%s undo offered with no snapshot", tc.name)
			} else if o.WhyNot != ErrNothingToUndo {
				t.Errorf("whyNot = %s, want %s", o.WhyNot, ErrNothingToUndo)
			}
			// Snapshot present.
			if o := FindOffer(LegalActions(offerFixture(tc.mut), "p1"), tc.offerID); !o.Enabled {
				t.Errorf("%s undo not offered despite a snapshot (whyNot=%s)", tc.name, o.WhyNot)
			}
		})
	}
}

func TestLegalActions_LayMeldShapeFollowsTheProfile(t *testing.T) {
	// The shape offer is what lets a client size its staging area without
	// knowing that continental wants 4-card runs and zolik_classic wants 3.
	t.Run("continental", func(t *testing.T) {
		o := FindOffer(LegalActions(offerFixture(nil), "p1"), OfferLayMeld)
		if o.Source.MinCards != 3 { // min(MinSetSize 3, MinRunSize 4)
			t.Errorf("minCards = %d, want 3", o.Source.MinCards)
		}
		// Four cards in hand, non-final deal: one must stay back to discard.
		if o.Source.MaxCards != 3 {
			t.Errorf("maxCards = %d, want 3", o.Source.MaxCards)
		}
	})

	t.Run("final deal lets the hand empty", func(t *testing.T) {
		s := offerFixture(func(s *GameState) { s.GameNumber = 7 })
		o := FindOffer(LegalActions(s, "p1"), OfferLayMeld)
		if o.Source.MaxCards != 4 {
			t.Errorf("maxCards = %d, want 4 on the final deal", o.Source.MaxCards)
		}
	})
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
