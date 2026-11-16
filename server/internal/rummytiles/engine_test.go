package rummytiles

import (
	"encoding/json"
	"testing"

	"zolik/server/internal/module"
)

func newMatch(t *testing.T, seed int64, playerIDs ...string) module.State {
	t.Helper()
	players := make([]module.PlayerRef, 0, len(playerIDs))
	for _, id := range playerIDs {
		players = append(players, module.PlayerRef{ID: id, Name: id})
	}
	st, err := New().NewMatch(module.MatchConfig{}, players, seed)
	if err != nil {
		t.Fatalf("NewMatch: %v", err)
	}
	return st
}

func stateOf(t *testing.T, raw module.State) *GameState {
	t.Helper()
	s, err := decode(raw)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	return s
}

// withState builds a hand-crafted two-player position, p1 on turn with an
// open workspace, so table/hand/tray content can be pinned exactly.
func withState(t *testing.T, mut func(*GameState)) module.State {
	t.Helper()
	s := &GameState{
		Status:      "active",
		Players:     []string{"p1", "p2"},
		Current:     "p1",
		Hands:       map[string][]string{"p1": {}, "p2": {}},
		Pool:        []string{"1-R", "2-R", "3-R"},
		Sets:        nil,
		InitialMeld: map[string]bool{},
		NextSetID:   1,
		Scores:      map[string]int{"p1": 0, "p2": 0},
		TargetScore: 200,
	}
	s.Workspace = &Workspace{}
	mut(s)
	raw, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return raw
}

func apply(t *testing.T, raw module.State, playerID string, a module.Action) (module.State, error) {
	t.Helper()
	next, _, err := New().Apply(raw, playerID, a)
	return next, err
}

func moduleAction(verb, target string, cards []string, params map[string]string) module.Action {
	return module.Action{Verb: verb, Target: target, Cards: cards, Params: params}
}

// --- dealing -----------------------------------------------------------

func TestNewMatch_DealsFourteenEachAndFillsThePool(t *testing.T) {
	s := stateOf(t, newMatch(t, 3, "p1", "p2", "p3"))
	for _, p := range s.Players {
		if len(s.Hands[p]) != 14 {
			t.Errorf("%s has %d tiles, want 14", p, len(s.Hands[p]))
		}
	}
	if len(s.Pool) != 106-3*14 {
		t.Errorf("pool = %d, want %d", len(s.Pool), 106-3*14)
	}
	if len(s.Sets) != 0 {
		t.Errorf("expected no sets at deal, got %d", len(s.Sets))
	}
	if s.Workspace == nil {
		t.Fatal("expected an open workspace for the first player on turn")
	}
}

func TestNewMatch_RejectsOutOfRangePlayerCounts(t *testing.T) {
	if _, err := New().NewMatch(module.MatchConfig{}, []module.PlayerRef{{ID: "p1"}}, 1); err == nil {
		t.Error("one player should be refused")
	}
	five := []module.PlayerRef{{ID: "1"}, {ID: "2"}, {ID: "3"}, {ID: "4"}, {ID: "5"}}
	if _, err := New().NewMatch(module.MatchConfig{}, five, 1); err == nil {
		t.Error("five players should be refused")
	}
}

// --- place / add / take -------------------------------------------------

func TestPlace_MovesTilesFromHandToANewWorkspaceSet(t *testing.T) {
	raw := withState(t, func(s *GameState) {
		s.Hands["p1"] = []string{"7-R", "7-B", "7-O", "9-K"}
	})
	next, err := apply(t, raw, "p1", moduleAction(VerbPlace, "", []string{"7-R", "7-B", "7-O"}, nil))
	if err != nil {
		t.Fatalf("place: %v", err)
	}
	s := stateOf(t, next)
	if len(s.Hands["p1"]) != 1 {
		t.Fatalf("expected one tile left in hand, got %v", s.Hands["p1"])
	}
	if len(s.Workspace.Sets) != 1 || len(s.Workspace.Sets[0].Cards) != 3 {
		t.Fatalf("expected one new three-tile set, got %+v", s.Workspace.Sets)
	}
}

func TestPlace_RefusesATileNotInHand(t *testing.T) {
	raw := withState(t, func(s *GameState) { s.Hands["p1"] = []string{"7-R"} })
	if _, err := apply(t, raw, "p1", moduleAction(VerbPlace, "", []string{"7-B"}, nil)); err == nil {
		t.Fatal("expected a refusal")
	} else if module.CodeOf(err) != ErrTileNotInHand {
		t.Errorf("expected %s, got %v", ErrTileNotInHand, err)
	}
}

func TestAdd_MayNotTouchAnOriginalSetBeforeInitialMeld(t *testing.T) {
	raw := withState(t, func(s *GameState) {
		s.InitialMeld["p1"] = false
		s.Sets = []Set{{ID: "s0", Kind: "group", Cards: []string{"7-R", "7-B", "7-O"}}}
		s.Workspace = &Workspace{Sets: cloneSets(s.Sets)}
		s.Hands["p1"] = []string{"7-K"}
	})
	if _, err := apply(t, raw, "p1", moduleAction(VerbAdd, "s0", []string{"7-K"}, nil)); err == nil {
		t.Fatal("expected a refusal before the initial meld")
	} else if module.CodeOf(err) != ErrInitialMeldOnly {
		t.Errorf("expected %s, got %v", ErrInitialMeldOnly, err)
	}
}

func TestAdd_MayTouchAFreshlyPlacedSetBeforeInitialMeld(t *testing.T) {
	raw := withState(t, func(s *GameState) {
		s.InitialMeld["p1"] = false
		s.Hands["p1"] = []string{"7-R", "7-B", "7-O", "7-K"}
	})
	raw, err := apply(t, raw, "p1", moduleAction(VerbPlace, "", []string{"7-R", "7-B", "7-O"}, nil))
	if err != nil {
		t.Fatalf("place: %v", err)
	}
	s := stateOf(t, raw)
	setID := s.Workspace.Sets[0].ID
	if _, err := apply(t, raw, "p1", moduleAction(VerbAdd, setID, []string{"7-K"}, nil)); err != nil {
		t.Fatalf("expected add onto this turn's own new set to be allowed, got %v", err)
	}
}

func TestAdd_MayComeFromTheTray(t *testing.T) {
	raw := withState(t, func(s *GameState) {
		s.InitialMeld["p1"] = true
		s.Sets = []Set{
			{ID: "s0", Kind: "run", Cards: []string{"3-R", "4-R", "5-R"}},
			{ID: "s1", Kind: "group", Cards: []string{"9-R", "9-B", "9-O"}},
		}
		s.Workspace = &Workspace{Sets: cloneSets(s.Sets)}
		s.Hands["p1"] = []string{"1-K"} // needs a hand tile played to reach commit later; irrelevant here
	})
	raw, err := apply(t, raw, "p1", moduleAction(VerbTake, "s1", []string{"9-R"}, nil))
	if err != nil {
		t.Fatalf("take: %v", err)
	}
	s := stateOf(t, raw)
	if len(s.Workspace.Tray) != 1 {
		t.Fatalf("expected one tile in the tray, got %v", s.Workspace.Tray)
	}
	if _, err := apply(t, raw, "p1", moduleAction(VerbAdd, "s0", []string{"9-R"}, nil)); err != nil {
		t.Fatalf("expected add from the tray to work, got %v", err)
	}
}

func TestTake_EmptyingASetRemovesItFromTheWorkspace(t *testing.T) {
	raw := withState(t, func(s *GameState) {
		s.InitialMeld["p1"] = true
		s.Sets = []Set{{ID: "s0", Kind: "group", Cards: []string{"7-R", "7-B"}}}
		s.Workspace = &Workspace{Sets: cloneSets(s.Sets)}
	})
	next, err := apply(t, raw, "p1", moduleAction(VerbTake, "s0", []string{"7-R", "7-B"}, nil))
	if err != nil {
		t.Fatalf("take: %v", err)
	}
	s := stateOf(t, next)
	if len(s.Workspace.Sets) != 0 {
		t.Fatalf("expected the emptied set to disappear, got %+v", s.Workspace.Sets)
	}
	if len(s.Workspace.Tray) != 2 {
		t.Fatalf("expected both tiles in the tray, got %v", s.Workspace.Tray)
	}
}

// --- reset_turn ----------------------------------------------------------

func TestResetTurn_ReturnsHandTilesAndDiscardsTheWorkspace(t *testing.T) {
	raw := withState(t, func(s *GameState) {
		s.Hands["p1"] = []string{"7-R", "7-B", "7-O"}
	})
	raw, err := apply(t, raw, "p1", moduleAction(VerbPlace, "", []string{"7-R", "7-B", "7-O"}, nil))
	if err != nil {
		t.Fatalf("place: %v", err)
	}
	next, err := apply(t, raw, "p1", moduleAction(VerbResetTurn, "", nil, nil))
	if err != nil {
		t.Fatalf("reset_turn: %v", err)
	}
	s := stateOf(t, next)
	if len(s.Hands["p1"]) != 3 {
		t.Fatalf("expected the hand restored, got %v", s.Hands["p1"])
	}
	if len(s.Workspace.Sets) != 0 {
		t.Fatalf("expected an empty workspace, got %+v", s.Workspace.Sets)
	}
}

func TestResetTurn_AlwaysReturnsTheTableToWhatItWas(t *testing.T) {
	raw := withState(t, func(s *GameState) {
		s.InitialMeld["p1"] = true
		s.Sets = []Set{{ID: "s0", Kind: "group", Cards: []string{"7-R", "7-B", "7-O"}}}
		s.Workspace = &Workspace{Sets: cloneSets(s.Sets)}
	})
	raw, err := apply(t, raw, "p1", moduleAction(VerbTake, "s0", []string{"7-R"}, nil))
	if err != nil {
		t.Fatalf("take: %v", err)
	}
	next, err := apply(t, raw, "p1", moduleAction(VerbResetTurn, "", nil, nil))
	if err != nil {
		t.Fatalf("reset_turn: %v", err)
	}
	s := stateOf(t, next)
	if len(s.Workspace.Sets) != 1 || len(s.Workspace.Sets[0].Cards) != 3 {
		t.Fatalf("expected the table restored to one three-tile set, got %+v", s.Workspace.Sets)
	}
	if len(s.Workspace.Tray) != 0 {
		t.Fatalf("expected an empty tray, got %v", s.Workspace.Tray)
	}
}

// --- the initial meld -----------------------------------------------------

func TestCommit_RefusedBelowThirtyPointsOnTheFirstLay(t *testing.T) {
	raw := withState(t, func(s *GameState) {
		s.Hands["p1"] = []string{"1-R", "1-B", "1-O"} // a valid group worth only 3
	})
	raw, err := apply(t, raw, "p1", moduleAction(VerbPlace, "", []string{"1-R", "1-B", "1-O"}, nil))
	if err != nil {
		t.Fatalf("place: %v", err)
	}
	if _, err := apply(t, raw, "p1", moduleAction(VerbCommit, "", nil, nil)); err == nil {
		t.Fatal("expected the initial meld to be refused below 30")
	} else if module.CodeOf(err) != ErrInitialMeldLow {
		t.Errorf("expected %s, got %v", ErrInitialMeldLow, err)
	}
}

func TestCommit_AcceptsThirtyOrMoreAndRecordsTheInitialMeld(t *testing.T) {
	raw := withState(t, func(s *GameState) {
		// 36 points, plus a spare tile so the hand does not empty and end
		// the round before the assertions below can run.
		s.Hands["p1"] = []string{"9-R", "9-B", "9-O", "9-K", "1-R"}
	})
	raw, err := apply(t, raw, "p1", moduleAction(VerbPlace, "", []string{"9-R", "9-B", "9-O", "9-K"}, nil))
	if err != nil {
		t.Fatalf("place: %v", err)
	}
	next, err := apply(t, raw, "p1", moduleAction(VerbCommit, "", nil, nil))
	if err != nil {
		t.Fatalf("commit: %v", err)
	}
	s := stateOf(t, next)
	if !s.InitialMeld["p1"] {
		t.Error("expected the initial meld to be recorded")
	}
	if len(s.Sets) != 1 {
		t.Fatalf("expected the set to land on the table, got %+v", s.Sets)
	}
}

func TestCommit_JokerInTheInitialMeldCountsAsTheTileItRepresents(t *testing.T) {
	// 9,9,joker(=9): 27 points, still short of 30 — the joker must not count
	// as its 30-point hand penalty here.
	raw := withState(t, func(s *GameState) {
		s.Hands["p1"] = []string{"9-R", "9-B", "JOKER1"}
	})
	raw, err := apply(t, raw, "p1", moduleAction(VerbPlace, "", []string{"9-R", "9-B", "JOKER1"}, nil))
	if err != nil {
		t.Fatalf("place: %v", err)
	}
	if _, err := apply(t, raw, "p1", moduleAction(VerbCommit, "", nil, nil)); err == nil {
		t.Fatal("27 points should still be refused — the joker must not count as 30")
	}
}

// --- ending a round --------------------------------------------------------

func TestCommit_GoingOutEndsTheRoundAndScores(t *testing.T) {
	raw := withState(t, func(s *GameState) {
		s.InitialMeld["p1"] = true
		s.Hands["p1"] = []string{"9-R", "9-B", "9-O"}
		s.Hands["p2"] = []string{"5-R", "JOKER1"}
	})
	next, err := apply(t, raw, "p1", moduleAction(VerbPlace, "", []string{"9-R", "9-B", "9-O"}, nil))
	if err != nil {
		t.Fatalf("place: %v", err)
	}
	next, err = apply(t, next, "p1", moduleAction(VerbCommit, "", nil, nil))
	if err != nil {
		t.Fatalf("commit: %v", err)
	}
	s := stateOf(t, next)
	if len(s.Rounds) != 1 || s.Rounds[0].Kind != "out" || s.Rounds[0].Winner != "p1" {
		t.Fatalf("expected a round won by p1 going out, got %+v", s.Rounds)
	}
	// p2's hand (5 + joker's 30) is negated; p1 gets the sum.
	if s.Scores["p2"] != -35 {
		t.Errorf("p2 score = %d, want -35", s.Scores["p2"])
	}
	if s.Scores["p1"] != 35 {
		t.Errorf("p1 score = %d, want 35", s.Scores["p1"])
	}
}

func TestDraw_DiscardsTheWorkspaceAndPassesTheTurn(t *testing.T) {
	raw := withState(t, func(s *GameState) {
		s.Hands["p1"] = []string{"7-R", "7-B", "7-O"}
	})
	raw, err := apply(t, raw, "p1", moduleAction(VerbPlace, "", []string{"7-R"}, nil))
	if err != nil {
		t.Fatalf("place: %v", err)
	}
	next, err := apply(t, raw, "p1", moduleAction(VerbDraw, "", nil, nil))
	if err != nil {
		t.Fatalf("draw: %v", err)
	}
	s := stateOf(t, next)
	if !hasTile(s.Hands["p1"], "7-R") {
		t.Error("expected the placed tile returned to hand")
	}
	// The original three, plus the one tile drawing itself adds.
	if len(s.Hands["p1"]) != 4 {
		t.Fatalf("expected hand restored (3) plus one drawn tile (1) = 4, got %v", s.Hands["p1"])
	}
	if s.Current != "p2" {
		t.Errorf("expected the turn to pass to p2, got %s", s.Current)
	}
	if s.Workspace == nil || len(s.Workspace.Sets) != 0 {
		t.Errorf("expected a fresh workspace for p2, got %+v", s.Workspace)
	}
}

// The written rule is "the pool runs dry and nobody can play": one player
// with nothing to draw only passes, and the round ends once every player has
// passed in a row. It used to end on the first empty-pool draw.
func TestDraw_EmptyPoolEndsTheRoundOnceNobodyCanPlay(t *testing.T) {
	raw := withState(t, func(s *GameState) {
		s.Pool = nil
		s.PoolExhaustionLowestWins = true
		s.Hands["p1"] = []string{"1-R"}
		s.Hands["p2"] = []string{"2-R", "3-B"}
	})
	offers, _ := New().LegalActions(raw, "p1")
	for _, o := range offers {
		if o.ID == OfferDraw && (o.LabelKey != "rummytiles.offer.pass" || !o.Enabled || o.Source != nil) {
			t.Errorf("an empty pool offers a pass, got %+v", o)
		}
	}
	next, events, err := New().Apply(raw, "p1", moduleAction(VerbDraw, "", nil, nil))
	if err != nil {
		t.Fatalf("pass: %v", err)
	}
	s := stateOf(t, next)
	if len(s.Rounds) != 0 || s.Current != "p2" || len(s.Hands["p1"]) != 1 {
		t.Fatalf("one pass must not end the round: rounds %+v, %s on, p1 holds %v", s.Rounds, s.Current, s.Hands["p1"])
	}
	if len(events) != 1 || events[0].Type != "turn_passed" {
		t.Errorf("events = %+v, want turn_passed", events)
	}

	next, err = apply(t, next, "p2", moduleAction(VerbDraw, "", nil, nil))
	if err != nil {
		t.Fatalf("second pass: %v", err)
	}
	s = stateOf(t, next)
	if len(s.Rounds) != 1 || s.Rounds[0].Kind != "pool_exhausted" {
		t.Fatalf("expected a pool-exhausted round ending, got %+v", s.Rounds)
	}
	if s.Rounds[0].Winner != "p1" { // p1 holds the lowest hand value (1 vs 5)
		t.Errorf("expected p1 (lowest hand) recorded as the round winner, got %q", s.Rounds[0].Winner)
	}
}

// A play between the passes starts the count again: the table it changed may
// give the players who passed a play.
func TestDraw_APlayAfterThePoolRunsDryRestartsTheCount(t *testing.T) {
	raw := withState(t, func(s *GameState) {
		s.Pool = nil
		s.Players = []string{"p1", "p2", "p3"}
		s.Hands["p1"] = []string{"1-R"}
		s.Hands["p2"] = []string{"7-R", "8-R", "9-R", "2-B"}
		s.Hands["p3"] = []string{"5-K"}
		s.Scores["p3"] = 0
		s.InitialMeld = map[string]bool{"p2": true}
	})
	next, err := apply(t, raw, "p1", moduleAction(VerbDraw, "", nil, nil))
	if err != nil {
		t.Fatalf("p1 pass: %v", err)
	}
	next, err = apply(t, next, "p2", moduleAction(VerbPlace, "", []string{"7-R", "8-R", "9-R"}, nil))
	if err != nil {
		t.Fatalf("p2 place: %v", err)
	}
	next, err = apply(t, next, "p2", moduleAction(VerbCommit, "", nil, nil))
	if err != nil {
		t.Fatalf("p2 commit: %v", err)
	}
	for _, p := range []string{"p3", "p1"} {
		if next, err = apply(t, next, p, moduleAction(VerbDraw, "", nil, nil)); err != nil {
			t.Fatalf("%s pass: %v", p, err)
		}
	}
	if s := stateOf(t, next); len(s.Rounds) != 0 || s.Current != "p2" {
		t.Fatalf("two passes since p2 played must not end a three-player round: %+v, %s on", s.Rounds, s.Current)
	}
	next, err = apply(t, next, "p2", moduleAction(VerbDraw, "", nil, nil))
	if err != nil {
		t.Fatalf("p2 pass: %v", err)
	}
	if s := stateOf(t, next); len(s.Rounds) != 1 || s.Rounds[0].Kind != "pool_exhausted" {
		t.Fatalf("three passes in a row end it, got %+v", s.Rounds)
	}
}

// Target score Off with the round limit Off left a match no end condition.
// It now plays to the variation's target, and the written rules say so.
func TestEndConditions_BothOffPlaysToTheDefaultTarget(t *testing.T) {
	cfg := module.MatchConfig{Variation: "standard", Options: module.Options{OptTargetScore: 0, OptRoundLimit: 0}}
	raw, err := New().NewMatch(cfg, []module.PlayerRef{{ID: "p1"}, {ID: "p2"}}, 1)
	if err != nil {
		t.Fatal(err)
	}
	s := stateOf(t, raw)
	if s.TargetScore != 200 || s.RoundLimit != 0 {
		t.Fatalf("target %d, round limit %d; want 200 and 0", s.TargetScore, s.RoundLimit)
	}
	s.Scores["p1"] = 200
	if w, ok := matchWinner(s); !ok || w != "p1" {
		t.Fatalf("reaching 200 ends the match, got %q %v", w, ok)
	}
	// A match stored before the fix, with both still zero, ends too.
	s.TargetScore = 0
	if w, ok := matchWinner(s); !ok || w != "p1" {
		t.Fatalf("an old Off/Off state: got %q %v", w, ok)
	}

	sections, _ := New().Rules(cfg)
	if !hasRule(sections, "rummytiles.rules.target", 200) {
		t.Errorf("the rules must state the target the match plays to: %+v", sections)
	}

	// Both on: both are written, since either ends the match.
	both := module.MatchConfig{Variation: "standard", Options: module.Options{OptTargetScore: 300, OptRoundLimit: 5}}
	sections, _ = New().Rules(both)
	if !hasRule(sections, "rummytiles.rules.target", 300) || !hasRule(sections, "rummytiles.rules.roundLimit", 5) {
		t.Errorf("both end conditions must be written: %+v", sections)
	}
}

func hasRule(sections []module.RuleSection, key string, n int) bool {
	for _, sec := range sections {
		for _, it := range sec.Items {
			if it.LabelKey == key && it.Params["n"] == n {
				return true
			}
		}
	}
	return false
}

// --- Apply does not mutate the caller's bytes -------------------------------

func TestApply_RefusalLeavesTheCallersStateUntouched(t *testing.T) {
	raw := withState(t, func(s *GameState) { s.Hands["p1"] = []string{"7-R"} })
	before := append(module.State(nil), raw...)
	if _, err := apply(t, raw, "p1", moduleAction(VerbPlace, "", []string{"9-Z"}, nil)); err == nil {
		t.Fatal("expected a refusal for a tile not in hand")
	}
	if string(raw) != string(before) {
		t.Error("a refused action must not mutate the caller's bytes")
	}
}
