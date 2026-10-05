package canasta

import (
	"os"
	"testing"

	"zolik/server/internal/module"
)

// The captures that used to strand a turn, one per report. Each position is the
// shape the report found, built by hand: the matches that found them were
// played by harnesses (cut stocks, mixed seats, the training env) whose seat
// choices a replay here would not reproduce, and the shape is what the engine
// has to refuse whoever reaches it.
//
// Each asks the same three things: the stranding capture is not on offer, the
// engine refuses it when asked anyway with CAPTURE_LEAVES_NO_DISCARD, and with
// the stock empty the deal ends at the turn change instead of handing the seat
// a turn whose only move is that capture.

// A seat on turn after p4 discards `top` onto an empty pile, with the stock
// gone. p1's side (p1+p3) is open with no canasta.
func strandedAfterDiscard(top string, hand []string, melds []Meld) module.State {
	return fourHanded(func(s *GameState) {
		s.Current = "p4"
		s.Phase = phaseMeld
		s.DrawPile = nil
		s.DiscardPile = nil
		s.Teams[1].HasMelded = true
		s.Teams[1].Melds = []Meld{{ID: "t1-9", TeamID: 1, Rank: "9", Cards: []string{"9C", "9D", "9S"}}}
		s.Hands["p4"] = []string{top, "5S", "6S"}
		s.Teams[0].HasMelded = true
		s.Teams[0].Melds = melds
		s.Hands["p1"] = hand
	})
}

// atDraw is the same position one move on, at p1's draw: what the engine was
// left holding before the fix, when the deal did not end.
func atDraw(t *testing.T, raw module.State, top string) module.State {
	t.Helper()
	s := mustDecode(t, raw)
	s.Current = "p1"
	s.Phase = phaseDraw
	s.Hands["p4"] = removeOne(s.Hands["p4"], top)
	s.DiscardPile = append(s.DiscardPile, top)
	s.MeldsAtTurnStart = true
	out, err := encode(s)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func removeOne(hand []string, card string) []string {
	out, _ := removeCards(hand, []string{card})
	return out
}

// assertStranding checks the three things for one capture.
func assertStranding(t *testing.T, before module.State, top string, capture module.Action) {
	t.Helper()
	m := New()

	draw := atDraw(t, before, top)
	offers, err := m.LegalActions(draw, "p1")
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range offers {
		if o.Enabled && o.Verb == VerbTakePile {
			t.Errorf("a stranding capture is on offer: %+v", o)
		}
	}
	if _, code := apply(t, draw, "p1", capture); code != ErrCaptureLeavesNoDiscard {
		t.Errorf("capture %+v: got %q, want %s", capture, code, ErrCaptureLeavesNoDiscard)
	}

	_, events, err := m.Apply(before, "p4", module.Action{Verb: VerbDiscard, Cards: []string{top}})
	if err != nil {
		t.Fatalf("p4's discard: %v", err)
	}
	if !dealEndedExhausted(events) {
		t.Errorf("the deal did not end on an empty stock with no capture to make: %v", events)
	}
}

func dealEndedExhausted(events []module.Event) bool {
	for _, e := range events {
		if e.Type == "deal_ended" && e.Data["exhausted"] == true {
			return true
		}
	}
	return false
}

// Reproduction 1: stock cut to 8 cards a deal, seed 122. Open, no canasta,
// holding a joker, the king of hearts and the ten of spades, with the king of
// clubs on the pile. The only capture melds the kings with the joker and leaves
// the ten — which cannot be discarded, since that is going out.
func TestCaptureLeavingOneUnshedableCardIsRefused(t *testing.T) {
	before := strandedAfterDiscard("KC",
		[]string{"JOKER2", "KH", "TS"},
		[]Meld{{ID: "t0-Q", TeamID: 0, Rank: "Q", Cards: []string{"QC", "QD", "QH"}}})
	assertStranding(t, before, "KC", module.Action{Verb: VerbTakePile, Cards: []string{"KH", "JOKER2"}})
}

// Reproduction 1, second shape: random-move seats with Hard, seed 278. Two tens
// in hand and the ten of hearts on a black three: the capture spends both tens
// and hands back the three, alone.
func TestCaptureHandingBackOnlyABuriedCardIsRefused(t *testing.T) {
	before := strandedAfterDiscard("TH",
		[]string{"TS", "TD"},
		[]Meld{{ID: "t0-Q", TeamID: 0, Rank: "Q", Cards: []string{"QC", "QD", "QH"}}})
	s := mustDecode(t, before)
	s.DiscardPile = []string{"3C"}
	before, _ = encode(s)
	assertStranding(t, before, "TH", module.Action{Verb: VerbTakePile, Cards: []string{"TS", "TD"}})
}

// Reproduction 2: Hard against Easy, seed 1000087 (PR #235's measurements).
// The pile is a single four, the hand a single joker, and the side has fours
// on the table. Capturing onto them spends nothing and gains nothing, so the
// joker is still the whole hand: no discard (going out), no lay-off (it would
// empty the hand without a canasta), and no meld.
func TestCaptureOntoAMeldWithOneCardInHandIsRefused(t *testing.T) {
	before := strandedAfterDiscard("4H",
		[]string{"JOKER1"},
		[]Meld{{ID: "t0-4", TeamID: 0, Rank: "4", Cards: []string{"4C", "4D", "4S"}}})
	assertStranding(t, before, "4H", module.Action{Verb: VerbTakePile, Target: "t0-4"})
}

// Reproduction 3: TestBotDoesNotCaptureIntoAnOpeningItCannotFinish's hand with
// the stock gone. Unopened on 3035, so needing 120: the capture is 30 and the
// hand after it looks like 100 more, but only by laying all but one card. The
// bot already declined it; the engine now refuses it to anyone, and with
// nothing to draw instead the deal is over.
func TestCaptureIntoAnUnfinishableOpeningIsRefused(t *testing.T) {
	hand := []string{"TC", "TH", "KS", "KC", "KD", "JH", "JC", "JS", "QS", "QC", "QD"}
	openingHand := hand
	m := New()
	draw := twoHanded(func(s *GameState) {
		s.Teams[0].Score, s.Teams[1].Score = 3035, 3955
		s.Hands["p1"] = append([]string(nil), hand...)
		s.DiscardPile = []string{"TD"}
		s.DrawPile = nil
	})
	offers, err := m.LegalActions(draw, "p1")
	if err != nil {
		t.Fatal(err)
	}
	if verbs := enabledVerbs(offers); len(verbs) != 0 {
		t.Errorf("enabled with an empty stock and no finishable capture: %v", verbs)
	}
	if _, code := apply(t, draw, "p1", module.Action{Verb: VerbTakePile, Cards: []string{"TC", "TH"}}); code != ErrCaptureLeavesNoDiscard {
		t.Errorf("got %q, want %s", code, ErrCaptureLeavesNoDiscard)
	}

	// One move earlier: p2 discards the ten, and that ends the deal.
	before := twoHanded(func(s *GameState) {
		s.Teams[0].Score, s.Teams[1].Score = 3035, 3955
		s.Hands["p1"] = append([]string(nil), hand...)
		s.Current, s.Phase = "p2", phaseMeld
		s.Teams[1].HasMelded = true
		s.Hands["p2"] = []string{"TD", "5S", "6S"}
		s.DrawPile = nil
	})
	_, events, err := m.Apply(before, "p2", module.Action{Verb: VerbDiscard, Cards: []string{"TD"}})
	if err != nil {
		t.Fatal(err)
	}
	if !dealEndedExhausted(events) {
		t.Errorf("the deal went on: %v", events)
	}

	// And the hand with two more cards in it is an opening that is all there:
	// the capture stands, and so does the deal.
	withNine := twoHanded(func(s *GameState) {
		s.Teams[0].Score, s.Teams[1].Score = 3035, 3955
		s.Hands["p1"] = append(append([]string(nil), openingHand...), "9C", "5S")
		s.DiscardPile = []string{"TD"}
		s.DrawPile = nil
	})
	// A side that has not opened may not lay off, so the opening is the capture
	// (30) and three melds of 30 — kings, jacks, queens — with two cards left
	// to discard one of and keep one.
	if _, code := apply(t, withNine, "p1", module.Action{Verb: VerbTakePile, Cards: []string{"TC", "TH"}}); code != "" {
		t.Errorf("a finishable opening capture was refused: %s", code)
	}
}

// Reproduction 4: the training env's "no move on p3 (hard) after 2 actions in
// a row". The seat captured, took it back, and had nothing. Here the stock is
// not empty, so the deal goes on and the question is the seat's turn: no
// candidate the learner is offered and no move the Hard bot makes may be the
// capture that empties the hand, and the Hard bot finishes the turn on a
// discard.
func TestTrainingSeatIsNeverOfferedACaptureThatEmptiesTheHand(t *testing.T) {
	m := New()
	raw := fourHanded(func(s *GameState) {
		s.Current, s.Phase = "p3", phaseDraw
		s.Teams[0].HasMelded = true
		s.Teams[0].Melds = []Meld{{ID: "t0-Q", TeamID: 0, Rank: "Q", Cards: []string{"QC", "QD", "QH"}}}
		s.Hands["p3"] = []string{"7S", "JOKER1"}
		s.DiscardPile = []string{"7D"}
		s.MeldsAtTurnStart = true
	})
	if _, code := apply(t, raw, "p3", module.Action{Verb: VerbTakePile, Cards: []string{"7S", "JOKER1"}}); code != ErrCaptureLeavesNoDiscard {
		t.Errorf("got %q, want %s", code, ErrCaptureLeavesNoDiscard)
	}
	offers, err := m.LegalActions(raw, "p3")
	if err != nil {
		t.Fatal(err)
	}
	cands, err := learnGame{}.Candidates(raw, "p3", offers)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cands {
		for _, a := range c.Steps() {
			if a.Verb == VerbTakePile {
				t.Errorf("the learner is offered a stranding capture: %+v", c)
			}
		}
	}

	for step := 0; step < 10; step++ {
		s := mustDecode(t, raw)
		if s.Current != "p3" {
			return
		}
		offers, err := m.LegalActions(raw, "p3")
		if err != nil {
			t.Fatal(err)
		}
		a, ok := m.Bot().Act(raw, module.BotSeat{PlayerID: "p3", Skill: module.SkillHard}, offers)
		if !ok {
			t.Fatalf("no move for p3 after %d actions: hand %v, offers %v", step, s.Hands["p3"], enabledVerbs(offers))
		}
		if a.Verb == VerbTakePile || isUndo(a.Verb) {
			t.Fatalf("p3 played %+v", a)
		}
		next, code := apply(t, raw, "p3", a)
		if code != "" {
			t.Fatalf("p3's %+v was refused: %s", a, code)
		}
		raw = next
	}
	t.Fatal("p3's turn never ended")
}

// Reproduction 5: classic, four seats, match seed 7025, deal 6 — the state the
// training env saved when p1 had no move. The stock is empty, the pile a lone
// six of clubs, p1's hand the four of hearts, and p1's side open with sixes on
// the table and no canasta. The capture onto the sixes leaves the four alone.
//
// The saved state is the position after the fault, at p1's draw. The capture
// is now refused, so p1 has nothing at all — and the turn change that led here
// ends the deal instead.
func TestSavedNoMoveStateEndsTheDeal(t *testing.T) {
	data, err := os.ReadFile("testdata/canasta-nomove-state.json")
	if err != nil {
		t.Fatal(err)
	}
	raw := module.State(data)
	m := New()

	offers, err := m.LegalActions(raw, "p1")
	if err != nil {
		t.Fatal(err)
	}
	if verbs := enabledVerbs(offers); len(verbs) != 0 {
		t.Errorf("p1 still has %v", verbs)
	}
	if _, code := apply(t, raw, "p1", module.Action{Verb: VerbTakePile, Target: "t1-6"}); code != ErrCaptureLeavesNoDiscard {
		t.Errorf("got %q, want %s", code, ErrCaptureLeavesNoDiscard)
	}

	s := mustDecode(t, raw)
	s.Current = "p0" // p1's predecessor, about to hand the turn on
	if events := advanceTurn(s); !dealEndedExhausted(events) {
		t.Errorf("handing the turn to p1 did not end the deal: %v", events)
	}
}

// Samba's other way into the pile has the same hole: the top card goes onto a
// sequence and not into the hand, so a seat holding one card and no canasta
// takes it and cannot discard. applyTakeTop refused that already, but the
// empty-stock check counted it as a move, so the deal went on with the seat
// unable to make it.
func TestTakeTopThatCannotFinishDoesNotKeepTheDealAlive(t *testing.T) {
	before := sambaTable(func(s *GameState) {
		s.Current, s.Phase = "p2", phaseMeld
		s.DrawPile = nil
		s.DiscardPile = []string{"KC"}
		s.Teams[1].HasMelded = true
		s.Hands["p2"] = []string{"8H", "5S", "6S"}
		s.Teams[0].HasMelded = true
		s.Teams[0].MeldSeq = 1
		s.Teams[0].Melds = []Meld{{ID: "t0-seq1", Kind: meldRun, Suit: "H", Cards: []string{"5H", "6H", "7H"}}}
		s.Hands["p1"] = []string{"QS"}
	})
	_, events, err := New().Apply(before, "p2", module.Action{Verb: VerbDiscard, Cards: []string{"8H"}})
	if err != nil {
		t.Fatal(err)
	}
	if !dealEndedExhausted(events) {
		t.Errorf("the deal went on with only an unfinishable take_top: %v", events)
	}
}

// The search the engine asks agrees with applyDiscard about every discard, so
// it cannot call a turn finished that the engine would then refuse to end.
func TestDiscardEndsTurnAgreesWithApplyDiscard(t *testing.T) {
	m := New()
	for _, cfg := range sweepConfigs {
		players := benchPlayers(cfg.seats)
		state, err := m.NewMatch(module.MatchConfig{Variation: cfg.variation}, players, 7)
		if err != nil {
			t.Fatal(err)
		}
		state = cutStock(state, 8)
		for step := 0; step < 300; step++ {
			if done, _, _ := m.Finished(state); done {
				break
			}
			actor := module.ActiveSeat(m, state, players[0].ID, players)
			if actor == "" {
				break
			}
			s := mustDecode(t, state)
			accepted := false
			for _, c := range s.Hands[actor] {
				if ok, _ := probe(m, state, actor, module.Action{Verb: VerbDiscard, Cards: []string{c}}); ok {
					accepted = true
				}
			}
			if got := discardEndsTurn(s, actor); got != accepted {
				t.Fatalf("%s/%d step %d: discardEndsTurn %v, applyDiscard accepts one: %v (hand %v)",
					cfg.variation, cfg.seats, step, got, accepted, s.Hands[actor])
			}
			offers, err := m.LegalActions(state, actor)
			if err != nil {
				t.Fatal(err)
			}
			seat := module.BotSeat{PlayerID: actor, Skill: module.SkillMedium, Seed: int64(step)}
			a, ok := m.Bot().Act(state, seat, offers)
			if !ok {
				t.Fatalf("no move for %s", actor)
			}
			next, _, err := m.Apply(state, actor, a)
			if err != nil {
				t.Fatal(err)
			}
			state = next
		}
	}
}
