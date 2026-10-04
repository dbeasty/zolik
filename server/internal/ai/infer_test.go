package ai

import (
	"math"
	"math/rand"
	"testing"

	"zolik/server/internal/cardinfer"
	"zolik/server/internal/rules"
)

// playedState deals a real match and plays it some way with Medium bots,
// observing every action into a ledger — so the position and its public
// record are ones the game produces, pickups and passes included.
func playedState(t *testing.T, seats int, seed int64, actions int) (rules.GameState, Ledger) {
	t.Helper()
	ids := []string{"p1", "p2", "p3", "p4"}[:seats]
	gs, err := rules.StartMatch(rules.ProfileZolikClassic, ids, seed, ids[0])
	if err != nil {
		t.Fatal(err)
	}
	var l Ledger
	l.Deal = gs.GameNumber
	agent := NewAgent("medium", seed)
	for i := 0; i < actions && gs.Status == rules.StatusActive && gs.Phase != rules.PhaseIntermission; i++ {
		p := gs.CurrentTurn
		a := agent.ChooseAction(VisibleFor(gs, l, p), gs.Hands[p])
		before := Before(gs)
		out, err := rules.ApplyAction(gs, p, a)
		if err != nil {
			t.Fatalf("seed %d action %d: %v", seed, i, err)
		}
		gs = out.State
		l.Observe(before, gs, p, a)
	}
	return gs, l
}

// scramble deals every hand but me's, and the stock, afresh from the same
// cards: same sizes, different contents.
func scramble(gs rules.GameState, me string, rng *rand.Rand) rules.GameState {
	var pool []string
	for id, h := range gs.Hands {
		if id != me {
			pool = append(pool, h...)
		}
	}
	pool = append(pool, gs.DrawPile...)
	rng.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })
	out := gs
	out.Hands = map[string][]string{}
	for _, id := range gs.TurnOrder {
		if id == me {
			out.Hands[id] = gs.Hands[id]
			continue
		}
		n := len(gs.Hands[id])
		out.Hands[id], pool = pool[:n], pool[n:]
	}
	out.DrawPile = pool
	return out
}

// TestInferDoesNotPeek: scrambling the hidden hands and the stock changes
// nothing the inference says, for any seat, at any point of a played deal.
func TestInferDoesNotPeek(t *testing.T) {
	rng := rand.New(rand.NewSource(5))
	for seed := int64(1); seed <= 6; seed++ {
		for _, n := range []int{10, 40, 90} {
			gs, l := playedState(t, 2+int(seed%3), seed, n)
			for _, me := range gs.TurnOrder {
				var want, got cardinfer.Estimate
				Infer(VisibleFor(gs, l, me), gs.Hands[me], me, &want)
				for k := 0; k < 3; k++ {
					sc := scramble(gs, me, rng)
					Infer(VisibleFor(sc, l, me), sc.Hands[me], me, &got)
					if want != got {
						t.Fatalf("seed %d after %d actions: %s's estimate moved when hidden cards did", seed, n, me)
					}
				}
			}
		}
	}
}

// TestInferOnPlayedPositions: on real positions the estimate keeps its
// promises — every opponent's expected cards are its hand size — and it puts
// more weight on the cards opponents actually hold than the unseen pool's
// average does. Not a test of strength, a test that it is not noise.
func TestInferOnPlayedPositions(t *testing.T) {
	hit, base := 0.0, 0.0
	for seed := int64(1); seed <= 30; seed++ {
		gs, l := playedState(t, 4, seed, 30+int(seed)*3)
		me := gs.CurrentTurn
		v := VisibleFor(gs, l, me)
		var e cardinfer.Estimate
		Infer(v, gs.Hands[me], me, &e)
		pool := 0.0
		for _, u := range e.Unseen {
			pool += u
		}
		for i, id := range Opponents(v, me) {
			sum := 0.0
			for _, x := range e.Hold[i] {
				sum += x
			}
			if math.Abs(sum-float64(len(gs.Hands[id]))) > 1e-6 {
				t.Fatalf("seed %d: %s expected %.4f cards, holds %d", seed, id, sum, len(gs.Hands[id]))
			}
			for _, c := range gs.Hands[id] {
				k := cardinfer.Slot(c)
				hit += e.Hold[i][k] / float64(len(gs.Hands[id]))
				base += e.Unseen[k] / pool
			}
		}
	}
	t.Logf("mean estimate on held cards %.4f vs a uniform prior's %.4f", hit, base)
	if hit <= base {
		t.Errorf("the estimate is no better than the prior on cards actually held: %.4f <= %.4f", hit, base)
	}
}

func TestLedgerRecordsPasses(t *testing.T) {
	gs := rules.GameState{
		GameNumber: 1, Round: 2, Phase: rules.PhaseDraw, CurrentTurn: "a", Status: rules.StatusActive,
		TurnOrder:   []string{"a", "b"},
		Hands:       map[string][]string{"a": {"2H"}, "b": {"3H"}},
		DrawPile:    []string{"9C", "TC"},
		DiscardPile: []string{"KD"},
		RoundReqMet: map[string]bool{},
		Rules:       rules.ProfileZolikClassic,
	}
	l := Ledger{Deal: 1}
	before := Before(gs)
	out, err := rules.ApplyAction(gs, "a", rules.Action{Type: rules.ActionDrawCard, DrawFrom: rules.DrawFromDeck})
	if err != nil {
		t.Fatal(err)
	}
	l.Observe(before, out.State, "a", rules.Action{Type: rules.ActionDrawCard, DrawFrom: rules.DrawFromDeck})
	if len(l.Passes) != 1 || l.Passes[0] != (SeenDiscard{Player: "a", Card: "KD"}) {
		t.Fatalf("passes %v", l.Passes)
	}
	if v := VisibleFor(out.State, l, "b"); len(v.DealPasses) != 1 {
		t.Fatalf("visible passes %v", v.DealPasses)
	}
}

// TestHardKeepsTheCardTheNextSeatWants is the bug the inference was built
// for: the next seat took the 9♦ and the 10♦ off the pile, so the 8♦ is the
// card it wants. Hard with the inference (HardInferProfile) keeps it,
// although it is the dearest loose card.
func TestHardKeepsTheCardTheNextSeatWants(t *testing.T) {
	hand := []string{"8D", "7H", "4S", "4H", "4C"}
	v := VisibleState{
		GameNumber: 1, Round: 4, Phase: string(rules.PhaseMeld), CurrentTurn: "me",
		TurnOrder:   []string{"me", "next"},
		DiscardPile: []string{"5C"},
		KnownHeld:   map[string][]string{"next": {"9D", "TD"}},
		HandCounts:  map[string]int{"me": len(hand), "next": 9},
		DeckCount:   2,
		Melds:       map[string][][]string{},
		MeldMeta:    map[string][]rules.MeldInfo{},
		RoundReqMet: map[string]bool{"me": false, "next": false},
		TotalScores: map[string]int{},
		Rules:       rules.ResolveConfig(rules.ProfileZolikClassic),
	}
	hard := NewAgentWithProfile(HardInferProfile(), 1)
	k := newKnowledge(v, hand, "me", hard.prof)
	got := hard.pickDiscard(hand, v, "me", k, rand.New(rand.NewSource(1)), false)
	if got == "8D" {
		t.Fatalf("Hard discarded the 8D the next seat is collecting")
	}
	classic := NewAgentWithProfile(HardClassicProfile(), 1)
	k = newKnowledge(v, hand, "me", classic.prof)
	t.Logf("hard-infer sheds %s; hard sheds %s", got, classic.pickDiscard(hand, v, "me", k, rand.New(rand.NewSource(1)), false))
}
