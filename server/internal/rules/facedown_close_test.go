package rules

import "testing"

// The closing gesture's factual half: WentOutByDiscard is set exactly when a
// discard is what ended the deal, and it does not survive into the next one.
// (The presentational half — whether that card is drawn face down — lives in
// the module's View and is tested beside it.)

func TestGoOut_ViaDiscard_RecordsWentOutByDiscard(t *testing.T) {
	p := "p1"
	// The final deal, so the match completes and the ended deal stays on the
	// table — mid-match the next deal is dealt in the same breath and rightly
	// wipes both the pile and the flag.
	st := baseActiveState(7, p)
	st.Phase = PhaseMeld
	st.Hands[p] = []string{"9S"}
	st.RoundReqMet[p] = true

	outcome, err := ApplyAction(st, p, Action{Type: ActionDiscard, Card: "9S"})
	if err != nil {
		t.Fatalf("discard: %v", err)
	}
	if !outcome.State.WentOutByDiscard {
		t.Fatalf("expected WentOutByDiscard after a deal-ending discard")
	}
	if top := outcome.State.DiscardPile[len(outcome.State.DiscardPile)-1]; top != "9S" {
		t.Fatalf("expected the closing card on top of the pile, got %q", top)
	}
}

func TestGoOut_ViaLayOff_DoesNotRecordWentOutByDiscard(t *testing.T) {
	p := "p1"
	st := baseActiveState(7, p)
	st.Hands[p] = []string{"9H"}
	st.RoundReqMet[p] = true
	st.Melds[p] = [][]string{{"5H", "6H", "7H", "8H"}}
	st.MeldMeta[p] = []MeldInfo{{MeldID: "m1", Type: MeldRun, OwnerID: p}}

	outcome, err := ApplyAction(st, p, Action{Type: ActionLayOff, MeldID: "m1", Card: "9H"})
	if err != nil {
		t.Fatalf("lay off: %v", err)
	}
	if outcome.State.WentOutByDiscard {
		t.Fatalf("a deal ended by a lay-off has no closing discard to remember")
	}
}

func TestWentOutByDiscard_ClearedByTheNextDeal(t *testing.T) {
	p := "p1"
	st := baseActiveState(1, p)
	st.WentOutByDiscard = true
	// Enough cards to actually deal the next game.
	st.DrawPile = nil
	st.DeckSeed = 7

	ns, err := StartNextGame(st, p)
	if err != nil {
		t.Fatalf("StartNextGame: %v", err)
	}
	if ns.WentOutByDiscard {
		t.Fatalf("WentOutByDiscard must not outlive the deal it describes")
	}
}

func TestOrdinaryDiscard_DoesNotRecordWentOutByDiscard(t *testing.T) {
	p := "p1"
	st := baseActiveState(1, p)
	st.Phase = PhaseMeld
	st.Hands[p] = []string{"9S", "4C"}

	ns, goOut, err := ValidateDiscard(st, p, "9S", nil)
	if err != nil {
		t.Fatalf("discard: %v", err)
	}
	if goOut || ns.WentOutByDiscard {
		t.Fatalf("a mid-deal discard closed nothing: goOut=%v flag=%v", goOut, ns.WentOutByDiscard)
	}
}
