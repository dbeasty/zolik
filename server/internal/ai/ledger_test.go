package ai

import (
	"reflect"
	"testing"

	"zolik/server/internal/rules"
)

// The ledger's whole job is to remember the two things a position forgets, and
// to forget them again at exactly the right moments. These are the moments.

func TestLedgerRecordsWhoDiscardedWhat(t *testing.T) {
	var l Ledger
	l.reset(1)
	st := rules.GameState{GameNumber: 1}
	l.Observe(Before(st), st, "karel", rules.Action{Type: rules.ActionDiscard, Card: "QD"})
	l.Observe(Before(st), st, "rita", rules.Action{Type: rules.ActionDiscard, Card: "7H"})

	want := []SeenDiscard{{Player: "karel", Card: "QD"}, {Player: "rita", Card: "7H"}}
	if !reflect.DeepEqual(l.Discards, want) {
		t.Fatalf("discards = %+v, want %+v", l.Discards, want)
	}
}

// A pickup is the one thing that genuinely cannot be re-derived: the moment
// somebody takes a card off the pile, the position stops recording where it
// went, and only a watcher knows.
func TestLedgerFollowsACardOffTheDiscardPile(t *testing.T) {
	var l Ledger
	l.reset(1)
	before := rules.GameState{GameNumber: 1, DiscardPile: []string{"2C", "9H"}}
	after := rules.GameState{GameNumber: 1, DiscardPile: []string{"2C"}}
	l.Observe(Before(before), after, "karel", rules.Action{
		Type: rules.ActionDrawCard, DrawFrom: rules.DrawFromDiscard,
	})
	if got := l.Held["karel"]; !reflect.DeepEqual(got, []string{"9H"}) {
		t.Fatalf("held = %v, want [9H]", got)
	}
	// ...and it stops being known the moment he plays it.
	l.Observe(Before(after), after, "karel", rules.Action{
		Type: rules.ActionLayMeld, Cards: []string{"9H", "9S", "9D"},
	})
	if got := l.Held["karel"]; len(got) != 0 {
		t.Fatalf("held = %v after the card was melded, want nothing", got)
	}
}

// Under DiscardPickupAnyFromPile taking one card takes every card above it, so
// how many moved is the engine's answer and not the ledger's guess.
func TestLedgerFollowsAWholeChunkOffThePile(t *testing.T) {
	var l Ledger
	l.reset(1)
	before := rules.GameState{GameNumber: 1, DiscardPile: []string{"2C", "9H", "TD", "JS"}}
	after := rules.GameState{GameNumber: 1, DiscardPile: []string{"2C"}}
	l.Observe(Before(before), after, "sona", rules.Action{
		Type: rules.ActionDrawCard, DrawFrom: rules.DrawFromDiscard,
	})
	if got := l.Held["sona"]; !reflect.DeepEqual(got, []string{"9H", "TD", "JS"}) {
		t.Fatalf("held = %v, want the three cards above the one taken", got)
	}
}

// The reshuffle is the case a naive counter gets exactly backwards. When the
// stock runs out the pile goes back into it, and every card in it becomes
// drawable again — so a counter that accumulated "cards that have been
// discarded" would go on believing they were gone. This one counts the pile as
// it stands, so it needs no code for the case at all; the test is here to pin
// that the ledger does not *add* code that breaks it.
func TestUnseenCountsRecoverAfterAReshuffle(t *testing.T) {
	prof := ProfileFor("hard")
	hand := []string{"KH", "KS"}
	withPile := VisibleState{
		DeckCount:   2,
		DiscardPile: []string{"KC", "KD"},
		Melds:       map[string][][]string{},
		RoundReqMet: map[string]bool{"me": true},
		Rules:       rules.ProfileZolikClassic,
	}
	before := newKnowledge(withPile, hand, "me", prof)
	// Two kings in hand and two on the pile: of the eight in a two-pack deal,
	// four are accounted for.
	if got := before.copiesLeft("KC"); got != 1 {
		t.Fatalf("with KC on the pile, copiesLeft = %d, want 1", got)
	}

	recycled := withPile
	recycled.DiscardPile = nil
	after := newKnowledge(recycled, hand, "me", prof)
	if got := after.copiesLeft("KC"); got != 2 {
		t.Fatalf("after the pile went back into the stock, copiesLeft = %d, want both copies live again", got)
	}
}

// A new deal is a new table. Remembering the last one would be worse than
// remembering nothing, because it would be confidently wrong.
func TestLedgerForgetsOnANewDeal(t *testing.T) {
	var l Ledger
	l.reset(1)
	st := rules.GameState{GameNumber: 1}
	l.Observe(Before(st), st, "karel", rules.Action{Type: rules.ActionDiscard, Card: "QD"})
	l.take("karel", []string{"9H"})

	next := rules.GameState{GameNumber: 2}
	l.Observe(Before(st), next, "karel", rules.Action{Type: rules.ActionDiscard, Card: "3C"})

	if len(l.Discards) != 0 || len(l.Held) != 0 {
		t.Fatalf("the ledger carried deal 1 into deal 2: %+v", l)
	}
	if l.Deal != 2 {
		t.Fatalf("deal = %d, want 2", l.Deal)
	}
}

// Undo rewinds the table to the draw, so it has to rewind what was learned
// since the draw too.
func TestLedgerRewindsWithAnUndo(t *testing.T) {
	var l Ledger
	l.reset(1)
	st := rules.GameState{GameNumber: 1, DiscardPile: []string{"9H"}}
	after := rules.GameState{GameNumber: 1}
	l.Observe(Before(st), after, "karel", rules.Action{
		Type: rules.ActionDrawCard, DrawFrom: rules.DrawFromDiscard,
	})
	l.Observe(Before(after), after, "karel", rules.Action{
		Type: rules.ActionLayMeld, Cards: []string{"9H", "9S", "9D"},
	})
	if len(l.Held["karel"]) != 0 {
		t.Fatalf("held = %v after melding, want nothing", l.Held["karel"])
	}
	l.Observe(Before(after), after, "karel", rules.Action{Type: rules.ActionUndoTurn})
	if got := l.Held["karel"]; !reflect.DeepEqual(got, []string{"9H"}) {
		t.Fatalf("held = %v after undo, want the 9H back in a hand everybody watched take it", got)
	}
}

// Recall is in laps of the table, so that "the last two laps" means the same
// thing at a table of two and a table of six.
func TestRecallReachesBackInLaps(t *testing.T) {
	v := VisibleState{DealDiscards: []SeenDiscard{
		{Player: "a", Card: "2C"}, {Player: "b", Card: "3C"},
		{Player: "a", Card: "4C"}, {Player: "b", Card: "5C"},
		{Player: "a", Card: "6C"}, {Player: "b", Card: "7C"},
	}}
	if got := v.DiscardsBy(0, 2); len(got) != 0 {
		t.Errorf("recall 0 remembered %v, want nothing at all", got)
	}
	one := v.DiscardsBy(1, 2)
	if !reflect.DeepEqual(one, map[string][]string{"a": {"6C"}, "b": {"7C"}}) {
		t.Errorf("one lap = %v, want only the last card from each seat", one)
	}
	all := v.DiscardsBy(RecallPerfect, 2)
	if len(all["a"]) != 3 || len(all["b"]) != 3 {
		t.Errorf("perfect recall = %v, want the whole deal", all)
	}
}
