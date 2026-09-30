package ai

import (
	"fmt"
	"testing"

	"zolik/server/internal/rules"
)

// The positions below are the wedges internal/ai/sim's whole-match sweep
// found, cut down to the decision that caused each one. The sweep is what
// proves the table now finishes; these pin *why*, one decision at a time.

// continentalDeal is the visible state of a Continental turn in its meld phase
// — drawn, about to discard — for a seat that is not yet down.
func continentalDeal(deal int, actor string) VisibleState {
	v := meldPhaseState(rules.ProfileContinental, actor)
	v.GameNumber = deal
	return v
}

// Deal seven asks for three runs. A set of threes is three loose cards under
// that contract, and the agent used to guard it as a finished meld while it
// shed its run material instead — the shape every hard bot converged on in the
// deal that never ended.
func TestDealSevenShedsTheSetNotTheRunMaterial(t *testing.T) {
	const actor = "ai:hard:1:0"
	hand := []string{"4H", "5H", "6H", "9S", "TS", "JS", "8C", "9C", "TC", "3C", "3D", "3S", "2H"}
	act := NewHeuristicAgent("hard").ChooseAction(continentalDeal(7, actor), hand)
	if act.Type != rules.ActionDiscard {
		t.Fatalf("wanted a discard, got %+v", act)
	}
	switch act.Card {
	case "3C", "3D", "3S":
	default:
		t.Fatalf("discarded %s: under a three-runs contract the set of threes is the loose material", act.Card)
	}
}

// Deal six asks for one set and two runs. Four fours and four sixes are eight
// cards of set material for one set's worth of contract; protecting all of it
// left the hand nowhere to grow the second run it lacked.
func TestContractPlanLeavesSurplusSetsLoose(t *testing.T) {
	const actor = "p1"
	hand := []string{"4D", "4S", "4H", "4C", "6H", "6C", "5C", "2C", "3C", "6S", "6D", "4S"}
	v := continentalDeal(6, actor)
	g := goalFor(v, actor, hand)
	if !g.quota || g.needSets != 1 || g.needRuns != 2 {
		t.Fatalf("goal = %+v, want a quota of one set and two runs", g)
	}
	plan := bestContractPlan(hand, v.Rules, g)
	setMaterial := 0
	for i, c := range hand {
		r := rules.CardRank(c)
		if plan.used[i] && (r == rules.CardRank("4C") || r == rules.CardRank("6C")) && rules.CardSuit(c) != "C" {
			setMaterial++
		}
	}
	if setMaterial > rules.MaxSetSize {
		t.Fatalf("plan keeps %d off-suit fours and sixes for a contract with one set in it: %v", setMaterial, plan.used)
	}
}

// A down seat's pair is only an investment while the hand has room to finish
// it and still discard. Two cards in hand after this discard cannot become a
// set and a discard, so the pair is two loose cards — and keeping it, while
// throwing back every card drawn, is how a seat stayed down and never went out.
func TestADownPairWithNoRoomIsNotProtected(t *testing.T) {
	const actor = "ai:hard:1:0"
	v := continentalDeal(2, actor)
	v.RoundReqMet[actor] = true
	act := NewHeuristicAgent("hard").ChooseAction(v, []string{"5H", "5S", "2D"})
	if act.Type != rules.ActionDiscard || act.Card == "2D" {
		t.Fatalf("got %+v: holding 5H 5S with nothing else in hand can never lay the set", act)
	}
}

// Two packs deal every card twice. J♣ J♣ is not two thirds of a set — a set
// takes one card per suit — and counting it as a pair made a seat hold both
// jacks while the table waited on a card it could not give.
func TestTheSameCardTwiceIsNotAPair(t *testing.T) {
	g := goal{}
	if n := fragmentPartners([]string{"JC", "JC", "2H"}, 0, rules.ProfileContinental, g); n != 0 {
		t.Fatalf("JC has %d partner(s) in JC JC 2H, want 0", n)
	}
	if n := fragmentPartners([]string{"JC", "JD", "2H"}, 0, rules.ProfileContinental, g); n != 1 {
		t.Fatalf("JC has %d partner(s) in JC JD 2H, want 1", n)
	}
}

// Under a quota the goal names the kinds of meld that count. Once down, every
// kind does.
func TestGoalFollowsTheContract(t *testing.T) {
	const actor = "p1"
	for deal, want := range map[int][2]bool{1: {true, false}, 3: {false, true}, 5: {true, true}, 7: {false, true}} {
		g := goalFor(continentalDeal(deal, actor), actor, nil)
		if g.wants(rules.MeldSet) != want[0] || g.wants(rules.MeldRun) != want[1] {
			t.Errorf("deal %d: wants set=%v run=%v, want %v", deal, g.wants(rules.MeldSet), g.wants(rules.MeldRun), want)
		}
	}
	v := continentalDeal(7, actor)
	v.RoundReqMet[actor] = true
	if g := goalFor(v, actor, nil); !g.wants(rules.MeldSet) {
		t.Error("a down seat can lay any meld; the goal should want sets again")
	}
}

// Four sixes and four fours clear a 35-point floor at forty. Searched only as
// three and three they come to thirty, and the agent sat on a hand that could
// go down — the same "erring low is also a bug" as Canasta's reachableValue.
func TestTheFloorSearchTriesLongerMelds(t *testing.T) {
	const actor = "p1"
	v := continentalDeal(1, actor)
	hand := []string{"6C", "6D", "6H", "6S", "4C", "4D", "4H", "4S", "2H"}
	combo, ok := findInitialMeldPlan(rulesStateForAI(v, actor), actor, hand)
	if !ok {
		t.Fatal("no plan found for 6666 + 4444 against a 35-point floor")
	}
	total := 0
	for _, m := range combo {
		mv, err := rules.ValidateMeld(m, v.Rules)
		if err != nil {
			t.Fatalf("planned an invalid meld %v: %v", m, err)
		}
		total += mv.NaturalValue
	}
	if total < v.Rules.InitialMeldMinimum {
		t.Fatalf("plan %v is worth %d, under the floor", combo, total)
	}
}

// fullSetsTable is a deal-one table with every set full but one.
func fullSetsTable(open []string) VisibleState {
	v := continentalDeal(1, "p1")
	v.RoundReqMet = map[string]bool{"p1": true, "p2": true}
	v.DeckCount = 2
	add := func(owner string, cards ...string) {
		id := fmt.Sprintf("%s-%d", owner, len(v.Melds[owner]))
		v.Melds[owner] = append(v.Melds[owner], cards)
		v.MeldMeta[owner] = append(v.MeldMeta[owner], rules.MeldInfo{MeldID: id, Type: rules.MeldSet, OwnerID: owner})
	}
	add("p1", "9C", "9D", "9H", "9S")
	add("p1", "5C", "5D", "5H", "5S")
	add("p2", open...)
	return v
}

// The last opening on the table is kept for going out on. Filling it with a
// hand that cannot then empty itself leaves a deal nobody at the table can
// finish: every set full, every seat down to cards that fit nowhere.
func TestTheLastOpeningIsKeptForGoingOut(t *testing.T) {
	v := fullSetsTable([]string{"JD", "JH", "JS"})
	if !closesTable(v, []string{"JC", "2H", "7D"}, "p2-0", "JC") {
		t.Error("laying JC with two cards left over shuts the table; it should be refused")
	}
	if closesTable(v, []string{"JC", "2H"}, "p2-0", "JC") {
		t.Error("laying JC and discarding the last card goes out; it must be allowed")
	}
	a := NewHeuristicAgent("hard")
	v.CurrentTurn = "p1"
	if act := a.ChooseAction(v, []string{"JC", "2H", "7D"}); act.Type == rules.ActionLayOff {
		t.Fatalf("laid off %s onto the last opening with cards still to shed", act.Card)
	}
}

// An opening that nothing left off the table can fill is not an opening. This
// is the shape of a deal-one table the sweep found wedged: three aces waiting
// on the ace of spades, a spade run waiting on the eight — and both packs'
// AS, both 8S and all four jokers already laid. The sevens taking their last
// suit is then the table shutting, however open the aces and the run look.
func TestAnOpeningNoCardCanFillIsClosed(t *testing.T) {
	table := func(aceOfSpadesLaidTwice bool) VisibleState {
		v := continentalDeal(1, "p1")
		v.RoundReqMet = map[string]bool{"p1": true, "p2": true}
		v.DeckCount = 2
		add := func(owner string, kind rules.MeldType, cards ...string) {
			v.Melds[owner] = append(v.Melds[owner], cards)
			v.MeldMeta[owner] = append(v.MeldMeta[owner], rules.MeldInfo{
				MeldID: fmt.Sprintf("%s-%d", owner, len(v.MeldMeta[owner])), Type: kind, OwnerID: owner,
			})
		}
		add("p2", rules.MeldSet, "7C", "7D", "7H") // p2-0, the lay-off target
		add("p2", rules.MeldSet, "AC", "AD", "AH")
		add("p1", rules.MeldRun, "AS", "2S", "3S", "4S", "5S", "6S", "7S")
		add("p1", rules.MeldSet, "8C", "8D", "8S", "JOKER2")
		add("p1", rules.MeldSet, "8H", "8S", "8D", "JOKER1")
		add("p1", rules.MeldSet, "TC", "TD", "TH", "JOKER2")
		if aceOfSpadesLaidTwice {
			add("p1", rules.MeldSet, "AD", "AH", "AS", "JOKER1")
		} else {
			add("p1", rules.MeldSet, "KD", "KH", "KS", "JOKER1")
		}
		return v
	}
	hand := []string{"7S", "2H", "9D"}
	if closesTable(table(false), hand, "p2-0", "7S") {
		t.Error("an ace of spades is still out, so the aces are open and the table is not shut")
	}
	if !closesTable(table(true), hand, "p2-0", "7S") {
		t.Error("every card that fits the aces or the run is laid; filling the sevens shuts the table")
	}
}

// The ledger keeps a bounded history and an exact count. A wedged deal used to
// grow the match state by every discard it ever made.
func TestLedgerCapsItsDiscardHistory(t *testing.T) {
	var l Ledger
	l.reset(1)
	st := rules.GameState{GameNumber: 1}
	const n = maxLedgerDiscards + 50
	for i := range n {
		l.Observe(Before(st), st, "karel", rules.Action{Type: rules.ActionDiscard, Card: fmt.Sprintf("%dC", 2+i%8)})
	}
	if len(l.Discards) != maxLedgerDiscards {
		t.Fatalf("kept %d discards, want the last %d", len(l.Discards), maxLedgerDiscards)
	}
	v := VisibleFor(st, l, "rita")
	if v.discardCount() != n {
		t.Fatalf("discard count = %d, want %d", v.discardCount(), n)
	}
	if last := l.Discards[len(l.Discards)-1].Card; last != fmt.Sprintf("%dC", 2+(n-1)%8) {
		t.Fatalf("the newest discard should be last; got %s", last)
	}
	// A ledger from before the count existed still answers from its list.
	old := Ledger{Deal: 1, Discards: []SeenDiscard{{Player: "karel", Card: "QD"}}}
	if got := VisibleFor(st, old, "rita").discardCount(); got != 1 {
		t.Fatalf("legacy ledger count = %d, want 1", got)
	}
}
