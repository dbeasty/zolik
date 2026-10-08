package klondike

import (
	"encoding/json"
	"testing"

	"zolik/server/internal/module"
)

var solo = []module.PlayerRef{{ID: "p1", Name: "p1"}}

func deal(t *testing.T, cfg module.MatchConfig, seed int64) *GameState {
	t.Helper()
	raw, err := New().NewMatch(cfg, solo, seed)
	if err != nil {
		t.Fatalf("NewMatch: %v", err)
	}
	s, err := decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// apply runs one action on a hand-built state.
func apply(t *testing.T, s *GameState, a module.Action) (*GameState, []module.Event, error) {
	t.Helper()
	raw, err := encode(s)
	if err != nil {
		t.Fatal(err)
	}
	out, events, err := New().Apply(raw, "p1", a)
	if err != nil {
		return s, nil, err
	}
	next, err := decode(out)
	if err != nil {
		t.Fatal(err)
	}
	return next, events, nil
}

func mustApply(t *testing.T, s *GameState, a module.Action) *GameState {
	t.Helper()
	next, _, err := apply(t, s, a)
	if err != nil {
		t.Fatalf("%+v: %v", a, err)
	}
	return next
}

func refusal(t *testing.T, s *GameState, a module.Action, want string) {
	t.Helper()
	_, _, err := apply(t, s, a)
	if got := module.CodeOf(err); got != want {
		t.Fatalf("%+v: got %q, want %q", a, got, want)
	}
}

// board builds a state from columns written as (down, up) pairs, with
// everything else empty. Cards not placed are irrelevant to these tests.
func board(cols ...Column) *GameState {
	s := tableOf(module.MatchConfig{})
	s.Player, s.Status = "p1", statusActive
	s.Cols = make([]Column, numColumns)
	for i := range s.Cols {
		s.Cols[i] = Column{Down: []string{}, Up: []string{}}
	}
	copy(s.Cols, cols)
	s.Found = [][]string{{}, {}, {}, {}}
	s.Stock, s.Waste = []string{"2C"}, []string{} // something left, so the game is not stuck
	return s
}

func col(down []string, up ...string) Column {
	if down == nil {
		down = []string{}
	}
	if up == nil {
		up = []string{}
	}
	return Column{Down: down, Up: up}
}

func mv(target string, cards ...string) module.Action {
	return module.Action{Verb: VerbMove, Cards: cards, Target: target}
}

func TestDealIsSevenColumnsAndAStock(t *testing.T) {
	s := deal(t, module.MatchConfig{}, 1)
	seen := map[string]bool{}
	note := func(c string) {
		if seen[c] {
			t.Fatalf("%s dealt twice", c)
		}
		seen[c] = true
	}
	for i, c := range s.Cols {
		if len(c.Down) != i || len(c.Up) != 1 {
			t.Errorf("column %d: %d down, %d up", i, len(c.Down), len(c.Up))
		}
		for _, x := range append(append([]string{}, c.Down...), c.Up...) {
			note(x)
		}
	}
	if len(s.Stock) != 24 {
		t.Errorf("stock has %d", len(s.Stock))
	}
	for _, x := range s.Stock {
		note(x)
	}
	if len(seen) != 52 {
		t.Errorf("%d distinct cards", len(seen))
	}
}

func TestOnlyOneSeat(t *testing.T) {
	_, err := New().NewMatch(module.MatchConfig{}, append(solo, module.PlayerRef{ID: "p2"}), 1)
	if module.CodeOf(err) != ErrWrongPlayerCount {
		t.Fatalf("two players: %v", err)
	}
	raw, _ := New().NewMatch(module.MatchConfig{}, solo, 1)
	if _, _, err := New().Apply(raw, "p2", module.Action{Verb: VerbDraw}); module.CodeOf(err) != ErrNotYourTurn {
		t.Fatalf("stranger drew: %v", err)
	}
}

func TestBuildDownInAlternatingColours(t *testing.T) {
	s := board(col(nil, "8S"), col(nil, "7H"), col(nil, "7C"), col(nil, "6S"))
	mustApply(t, s, mv("t1", "7H"))
	refusal(t, s, mv("t1", "7C"), ErrDoesNotBuild)
	refusal(t, s, mv("t1", "6S"), ErrDoesNotBuild)
}

func TestARunMovesWholeAndOnlyWhole(t *testing.T) {
	s := board(col(nil, "9H"), col([]string{"KD"}, "8S", "7H", "6C"))
	s = mustApply(t, s, mv("t1", "8S", "7H", "6C"))
	if got := s.Cols[0].Up; len(got) != 4 {
		t.Fatalf("column 1 now %v", got)
	}
	// The king under the run turned up by itself.
	if got := s.Cols[1].Up; len(got) != 1 || got[0] != "KD" || len(s.Cols[1].Down) != 0 {
		t.Fatalf("column 2 now %+v", s.Cols[1])
	}

	s2 := board(col(nil, "9H"), col(nil, "8S", "7H", "6C"))
	refusal(t, s2, mv("t1", "8S", "7H"), ErrNotWholeRun)
}

func TestOnlyAKingFillsAnEmptyColumn(t *testing.T) {
	s := board(col(nil), col(nil, "QH"), col([]string{"3C"}, "KS", "QH"))
	refusal(t, s, mv("t1", "QH"), ErrKingOnly)
	s = mustApply(t, s, mv("t1", "KS", "QH"))
	if len(s.Cols[0].Up) != 2 {
		t.Fatal("king run did not land")
	}
}

func TestFoundationsBuildUpBySuit(t *testing.T) {
	s := board(col(nil, "AH"), col(nil, "2H"), col(nil, "2S"))
	refusal(t, s, mv("f-H", "2H"), ErrFoundationOrder)
	refusal(t, s, mv("f-S", "AH"), ErrFoundationOrder)
	s = mustApply(t, s, mv("f-H", "AH"))
	refusal(t, s, mv("f-H", "2S"), ErrFoundationOrder)
	s = mustApply(t, s, mv("f-H", "2H"))
	if len(s.Found[1]) != 2 {
		t.Fatalf("hearts: %v", s.Found[1])
	}
}

func TestTakingACardBackIsAnOption(t *testing.T) {
	s := board(col(nil, "4S"))
	s.Found[1] = []string{"AH", "2H", "3H"}
	if _, _, err := apply(t, s, mv("t1", "3H")); err != nil {
		t.Fatalf("take back with the option on: %v", err)
	}
	s.TakeBack = false
	refusal(t, s, mv("t1", "3H"), ErrTakeBackOff)
}

func TestTheStockTurnsOneOrThree(t *testing.T) {
	for _, draw := range []int{1, 3} {
		s := deal(t, module.MatchConfig{Options: module.Options{OptDraw: draw}}, 2)
		top := s.Stock[len(s.Stock)-1]
		s = mustApply(t, s, module.Action{Verb: VerbDraw})
		if len(s.Waste) != draw || len(s.Stock) != 24-draw {
			t.Fatalf("draw %d: waste %d stock %d", draw, len(s.Waste), len(s.Stock))
		}
		if s.Waste[0] != top {
			t.Fatalf("draw %d: the stock's top card %s should be first out, waste %v", draw, top, s.Waste)
		}
	}
}

func TestRedealLimit(t *testing.T) {
	s := deal(t, module.MatchConfig{Options: module.Options{OptRedeals: 0}}, 3)
	refusal(t, s, module.Action{Verb: VerbRecycle}, ErrStockNotEmpty)
	for len(s.Stock) > 0 {
		s = mustApply(t, s, module.Action{Verb: VerbDraw})
		if s.Status != statusActive {
			return // ended itself: nothing could move and the stock is spent
		}
	}
	refusal(t, s, module.Action{Verb: VerbDraw}, ErrStockEmpty)
	refusal(t, s, module.Action{Verb: VerbRecycle}, ErrNoRedeals)

	s2 := deal(t, module.MatchConfig{Options: module.Options{OptRedeals: 2}}, 3)
	s2.Waste, s2.Stock = append(s2.Waste, s2.Stock...), []string{}
	s2.Moved = true
	for i := 0; i < 2; i++ {
		s2 = mustApply(t, s2, module.Action{Verb: VerbRecycle})
		s2.Waste, s2.Stock = append(s2.Waste, s2.Stock...), []string{}
		s2.Moved = true
	}
	refusal(t, s2, module.Action{Verb: VerbRecycle}, ErrNoRedeals)
}

func TestRecyclePreservesOrder(t *testing.T) {
	s := deal(t, module.MatchConfig{}, 4)
	order := append([]string{}, s.Stock...)
	for len(s.Stock) > 0 {
		s = mustApply(t, s, module.Action{Verb: VerbDraw})
	}
	s.Moved = true // as if a card had moved this pass
	s = mustApply(t, s, module.Action{Verb: VerbRecycle})
	if len(s.Waste) != 0 || len(s.Stock) != 24 {
		t.Fatalf("waste %d stock %d", len(s.Waste), len(s.Stock))
	}
	for i := range order {
		if s.Stock[i] != order[i] {
			t.Fatalf("stock order changed at %d: %v vs %v", i, s.Stock, order)
		}
	}
}

func TestStandardScoring(t *testing.T) {
	s := board(col([]string{"5C"}, "AH"), col(nil, "9S"))
	s.Waste = []string{"8H"}
	s = mustApply(t, s, mv("t2", "8H")) // waste to tableau +5
	if s.Score != 5 {
		t.Fatalf("waste to tableau: %d", s.Score)
	}
	s = mustApply(t, s, mv("f-H", "AH")) // +10 to foundation, +5 turned up
	if s.Score != 20 {
		t.Fatalf("to foundation and turn up: %d", s.Score)
	}
	s.Cols[2] = col(nil, "2C")
	s = mustApply(t, s, mv("t3", "AH")) // back down: -15
	if s.Score != 5 {
		t.Fatalf("take back: %d", s.Score)
	}
	// And it is not progress to put it straight back up.
	idle := s.Idle
	s = mustApply(t, s, mv("f-H", "AH"))
	if s.Idle != idle+1 {
		t.Fatalf("a card going back up counted as progress")
	}
}

func TestVegasScoring(t *testing.T) {
	s := deal(t, module.MatchConfig{Variation: "vegas"}, 5)
	if s.Score != -52 || s.Draw != 3 || s.Redeals != 0 {
		t.Fatalf("vegas table: %+v", s)
	}
	b := board(col(nil, "AH"))
	b.Scoring, b.Score = scoreVegas, -52
	b = mustApply(t, b, mv("f-H", "AH"))
	if b.Score != -47 {
		t.Fatalf("vegas foundation: %d", b.Score)
	}
}

func TestRecyclePenalty(t *testing.T) {
	for draw, want := range map[int]int{1: 100, 3: 20} {
		s := board(col(nil, "KS"))
		s.Draw, s.Score, s.Moved = draw, 500, true
		s.Stock, s.Waste = []string{}, []string{"2C", "3C"}
		s = mustApply(t, s, module.Action{Verb: VerbRecycle})
		if s.Score != 500-want {
			t.Errorf("draw %d: score %d", draw, s.Score)
		}
	}
}

func TestUndoTakesBackAMoveButNeverATurnUp(t *testing.T) {
	s := board(col(nil, "9H"), col(nil, "8S"), col([]string{"QD"}, "8C"))
	s = mustApply(t, s, mv("t1", "8S"))
	s = mustApply(t, s, module.Action{Verb: VerbUndo})
	if len(s.Cols[0].Up) != 1 || len(s.Cols[1].Up) != 1 || s.Cols[1].Up[0] != "8S" {
		t.Fatalf("undo did not carry the card home: %+v", s.Cols[:2])
	}
	refusal(t, s, module.Action{Verb: VerbUndo}, ErrNothingToUndo)

	// This one turns the queen up: there is nothing to undo afterwards.
	s = mustApply(t, s, mv("t1", "8C"))
	refusal(t, s, module.Action{Verb: VerbUndo}, ErrNothingToUndo)

	// Nor after a draw, which shows a card too.
	s = mustApply(t, s, module.Action{Verb: VerbDraw})
	refusal(t, s, module.Action{Verb: VerbUndo}, ErrNothingToUndo)
}

func TestAutoFinishAndWin(t *testing.T) {
	s := board()
	s.Stock = []string{}
	// Every card face up, kings at the base, alternating down.
	var a, b []string
	for r := 13; r >= 1; r-- {
		if r%2 == 0 {
			a = append(a, ranks[r-1]+"H")
			b = append(b, ranks[r-1]+"S")
		} else {
			a = append(a, ranks[r-1]+"C")
			b = append(b, ranks[r-1]+"D")
		}
	}
	var c, d []string
	for r := 13; r >= 1; r-- {
		if r%2 == 0 {
			c = append(c, ranks[r-1]+"D")
			d = append(d, ranks[r-1]+"C")
		} else {
			c = append(c, ranks[r-1]+"S")
			d = append(d, ranks[r-1]+"H")
		}
	}
	s.Cols[0], s.Cols[1], s.Cols[2], s.Cols[3] = col(nil, a...), col(nil, b...), col(nil, c...), col(nil, d...)

	s.AutoFinish = false
	refusal(t, s, module.Action{Verb: VerbAutoFinish}, ErrNotFinishable)
	s.AutoFinish = true
	s, events, err := apply(t, s, module.Action{Verb: VerbAutoFinish})
	if err != nil {
		t.Fatal(err)
	}
	if s.Status != statusWon || s.foundationCount() != 52 {
		t.Fatalf("status %s, %d on foundations", s.Status, s.foundationCount())
	}
	if events[len(events)-1].Type != "won" {
		t.Fatalf("last event %v", events[len(events)-1])
	}
	raw, _ := encode(s)
	done, winners, _ := New().Finished(raw)
	if !done || len(winners) != 1 || winners[0] != "p1" {
		t.Fatalf("Finished: %v %v", done, winners)
	}
}

func TestGivingUpLosesAndNamesNobody(t *testing.T) {
	raw, _ := New().NewMatch(module.MatchConfig{}, solo, 9)
	out, _, err := New().Apply(raw, "p1", module.Action{Verb: VerbGiveUp})
	if err != nil {
		t.Fatal(err)
	}
	done, winners, _ := New().Finished(out)
	if !done || len(winners) != 0 {
		t.Fatalf("Finished: %v %v", done, winners)
	}
	st, _ := New().Standings(out)
	if len(st) != 1 || st[0].Won {
		t.Fatalf("a lost game ranked as won: %+v", st)
	}
}

func TestADeadPositionEndsTheGame(t *testing.T) {
	s := board(col(nil, "KS"), col(nil, "KH"))
	s.Stock = []string{}
	s.Waste = []string{"2C"}
	s.Redeals = 0
	// Nothing on the table can move and the waste cannot be turned again; the
	// only action is giving up, so the next move the player makes — here a
	// pointless one that is still legal would not exist, so apply give-up's
	// sibling: any action settles it. Settle directly.
	if evs := settle(s); s.Status != statusLost || len(evs) == 0 {
		t.Fatalf("dead position left open: %s", s.Status)
	}
}

func TestStateRoundTrips(t *testing.T) {
	raw, _ := New().NewMatch(module.MatchConfig{Variation: "draw3"}, solo, 11)
	var back GameState
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if back.Draw != 3 {
		t.Fatalf("draw %d", back.Draw)
	}
}
