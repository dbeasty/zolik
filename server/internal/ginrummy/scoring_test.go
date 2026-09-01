package ginrummy

import "testing"

func newScoringState() *GameState {
	return &GameState{
		Players:     []string{"p1", "p2"},
		Scores:      map[string]int{"p1": 0, "p2": 0},
		HandsWon:    map[string]int{},
		TargetScore: 100,
		LineBonuses: true,
	}
}

func TestScoreHand_KnockerBeatsDefenderByTheDifference(t *testing.T) {
	s := newScoringState()
	s.Knocker = "p1"
	s.KnockerDeadwood = 4
	s.Hands = map[string][]string{"p2": {"KH", "QD"}} // 10 + 10 = 20 deadwood

	res := scoreHand(s)
	if res.Kind != "knock" || res.Winner != "p1" {
		t.Fatalf("got %+v, want a knock won by p1", res)
	}
	if res.HandDelta != 16 { // 20 - 4
		t.Errorf("delta = %d, want 16", res.HandDelta)
	}
	if s.Scores["p1"] != 16 || s.Scores["p2"] != 0 {
		t.Errorf("scores = %+v", s.Scores)
	}
}

func TestScoreHand_UndercutSwingsToTheDefenderPlusBonus(t *testing.T) {
	s := newScoringState()
	s.Knocker = "p1"
	s.KnockerDeadwood = 8
	s.Hands = map[string][]string{"p2": {"3D"}} // 3, level or lower than the knocker

	res := scoreHand(s)
	if res.Kind != "undercut" || res.Winner != "p2" {
		t.Fatalf("got %+v, want an undercut won by p2", res)
	}
	if res.HandDelta != 30 { // (8-3) + 25
		t.Errorf("delta = %d, want 30", res.HandDelta)
	}
}

func TestScoreHand_DefenderLevelWithKnockerIsAlsoAnUndercut(t *testing.T) {
	s := newScoringState()
	s.Knocker = "p1"
	s.KnockerDeadwood = 5
	s.Hands = map[string][]string{"p2": {"5H"}} // exactly level

	res := scoreHand(s)
	if res.Kind != "undercut" || res.Winner != "p2" {
		t.Fatalf("a defender level with the knocker undercuts, got %+v", res)
	}
	if res.HandDelta != 25 { // (5-5) + 25
		t.Errorf("delta = %d, want 25", res.HandDelta)
	}
}

func TestScoreHand_GinScoresDefendersFullHandPlusBonus(t *testing.T) {
	s := newScoringState()
	s.Knocker = "p1"
	s.KnockGin = true
	s.KnockerDeadwood = 0
	s.Hands = map[string][]string{"p2": {"KH", "QD", "AH"}} // 10+10+1 = 21

	res := scoreHand(s)
	if res.Kind != "gin" || res.Winner != "p1" {
		t.Fatalf("got %+v", res)
	}
	if res.HandDelta != 46 { // 21 + 25
		t.Errorf("delta = %d, want 46", res.HandDelta)
	}
}

func TestApplyLineBonuses_AddsBoxesAndTheGameBonus(t *testing.T) {
	s := newScoringState()
	s.Scores = map[string]int{"p1": 100, "p2": 40}
	s.HandsWon = map[string]int{"p1": 3, "p2": 1}
	res := HandResult{Deltas: map[string]int{"p1": 0, "p2": 0}}

	applyLineBonuses(s, "p1", &res)

	wantWinner := 100 + 25*3 + 100 // boxes + the game
	wantLoser := 40 + 25*1
	if s.Scores["p1"] != wantWinner {
		t.Errorf("winner score = %d, want %d", s.Scores["p1"], wantWinner)
	}
	if s.Scores["p2"] != wantLoser {
		t.Errorf("loser score = %d, want %d", s.Scores["p2"], wantLoser)
	}
	if res.Deltas["p1"] != 25*3+100 || res.Deltas["p2"] != 25*1 {
		t.Errorf("deltas = %+v", res.Deltas)
	}
}

func TestApplyLineBonuses_DoublesTheGameBonusOnAShutout(t *testing.T) {
	s := newScoringState()
	s.Scores = map[string]int{"p1": 100, "p2": 0} // p2 never scored a point
	s.HandsWon = map[string]int{"p1": 4, "p2": 0}
	res := HandResult{Deltas: map[string]int{"p1": 0, "p2": 0}}

	applyLineBonuses(s, "p1", &res)

	want := 100 + 25*4 + 200
	if s.Scores["p1"] != want {
		t.Errorf("shutout score = %d, want %d", s.Scores["p1"], want)
	}
}

func TestApplyLineBonuses_OffWhenTheOptionIsOff(t *testing.T) {
	s := newScoringState()
	s.LineBonuses = false
	s.Scores = map[string]int{"p1": 100, "p2": 40}
	s.HandsWon = map[string]int{"p1": 3, "p2": 1}
	res := HandResult{Deltas: map[string]int{"p1": 0, "p2": 0}}

	applyLineBonuses(s, "p1", &res)

	if s.Scores["p1"] != 100 || s.Scores["p2"] != 40 {
		t.Errorf("no bonus should apply when the option is off, got %+v", s.Scores)
	}
}

func TestMatchWinner_FirstToPassTheTarget(t *testing.T) {
	s := newScoringState()
	s.Scores = map[string]int{"p1": 99, "p2": 40}
	if _, ok := matchWinner(s); ok {
		t.Fatal("99 has not yet passed a target of 100")
	}
	s.Scores["p1"] = 100
	winner, ok := matchWinner(s)
	if !ok || winner != "p1" {
		t.Fatalf("expected p1 to win at exactly the target, got %q, %v", winner, ok)
	}
}
