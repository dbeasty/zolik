package holdem

import (
	"strconv"
	"testing"

	"zolik/server/internal/module"
)

func apply(t *testing.T, raw module.State, player string, a module.Action) (module.State, string) {
	t.Helper()
	next, _, err := New().Apply(raw, player, a)
	if err != nil {
		return raw, module.CodeOf(err)
	}
	return next, ""
}

func mustDecode(t *testing.T, raw module.State) *GameState {
	t.Helper()
	s, err := decode(raw)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	return s
}

func raiseTo(n int) module.Action {
	return module.Action{Verb: VerbRaise, Params: map[string]string{ParamAmount: strconv.Itoa(n)}}
}

// table builds a state directly, so a betting rule can be tested at the exact
// moment it applies rather than being played into existence.
func table(seats int, mutate func(s *GameState)) module.State {
	s := &GameState{
		Status:        "active",
		Street:        streetPreflop,
		BigBlind:      20,
		SmallBlind:    10,
		StartingStack: 1000,
		MinRaise:      20,
		HandNumber:    1,
		Button:        0,
		Current:       0,
		Deck:          buildDeck(),
	}
	for i := 0; i < seats; i++ {
		s.Seats = append(s.Seats, Seat{
			PlayerID: "p" + strconv.Itoa(i+1),
			Stack:    1000,
			Hole:     []string{"2C", "7D"},
		})
	}
	if mutate != nil {
		mutate(s)
	}
	raw, err := encode(s)
	if err != nil {
		panic(err)
	}
	return raw
}

// --- blinds and position ----------------------------------------------------

// TestHeadsUpBlindsAndPosition covers the one positional rule that is not
// "clockwise from the button". Getting it wrong is invisible until a two-handed
// game plays out strangely, so it is pinned rather than trusted.
func TestHeadsUpBlindsAndPosition(t *testing.T) {
	m := New()
	raw, err := m.NewMatch(module.MatchConfig{}, refs("p1", "p2"), 1)
	if err != nil {
		t.Fatalf("NewMatch: %v", err)
	}
	s := mustDecode(t, raw)

	button := s.Seats[s.Button]
	other := s.Seats[(s.Button+1)%2]

	if button.Bet != s.SmallBlind {
		t.Errorf("heads up, the button posts the small blind: bet is %d, want %d",
			button.Bet, s.SmallBlind)
	}
	if other.Bet != s.BigBlind {
		t.Errorf("the other seat posts the big blind: bet is %d, want %d",
			other.Bet, s.BigBlind)
	}
	if s.Seats[s.Current].PlayerID != button.PlayerID {
		t.Errorf("heads up, the button acts first before the flop; %q is on turn",
			s.Seats[s.Current].PlayerID)
	}
}

// TestThreeHandedBlinds is the ordinary case, for contrast: small blind left of
// the button, big blind left of that, and the seat after the big blind acts.
func TestThreeHandedBlinds(t *testing.T) {
	m := New()
	raw, err := m.NewMatch(module.MatchConfig{}, refs("p1", "p2", "p3"), 1)
	if err != nil {
		t.Fatalf("NewMatch: %v", err)
	}
	s := mustDecode(t, raw)

	sb := (s.Button + 1) % 3
	bb := (s.Button + 2) % 3
	if s.Seats[sb].Bet != s.SmallBlind {
		t.Errorf("small blind is on seat %d with bet %d, want %d", sb, s.Seats[sb].Bet, s.SmallBlind)
	}
	if s.Seats[bb].Bet != s.BigBlind {
		t.Errorf("big blind is on seat %d with bet %d, want %d", bb, s.Seats[bb].Bet, s.BigBlind)
	}
	// Three-handed, the seat after the big blind is the button.
	if s.Current != s.Button {
		t.Errorf("seat %d is on turn, want the button (%d)", s.Current, s.Button)
	}
}

// TestBigBlindHasTheOption — posting a blind is not acting, so a big blind who
// is merely called round to still gets to raise.
func TestBigBlindHasTheOption(t *testing.T) {
	raw := table(3, func(s *GameState) {
		// Blinds posted, everyone has called to the big blind.
		s.Seats[1].Bet, s.Seats[1].Committed, s.Seats[1].Stack = 20, 20, 980
		s.Seats[2].Bet, s.Seats[2].Committed, s.Seats[2].Stack = 20, 20, 980
		s.Seats[0].Bet, s.Seats[0].Committed, s.Seats[0].Stack = 20, 20, 980
		s.Seats[0].Acted, s.Seats[1].Acted = true, true
		s.Seats[2].Acted = false // the big blind, not yet acted
		s.CurrentBet = 20
		s.Current = 2
	})

	// The street must not have closed: the big blind still has a decision.
	s := mustDecode(t, raw)
	if bettingClosed(s) {
		t.Fatal("the street closed before the big blind had its option")
	}
	offers, err := New().LegalActions(raw, "p3")
	if err != nil {
		t.Fatalf("LegalActions: %v", err)
	}
	if o := module.FindOffer(offers, OfferCheck); o == nil || !o.Enabled {
		t.Error("the big blind should be able to check its option")
	}
	if o := module.FindOffer(offers, OfferRaise); o == nil || !o.Enabled {
		t.Error("the big blind should be able to raise its option")
	}
}

// --- betting rules ----------------------------------------------------------

func TestBettingRules(t *testing.T) {
	cases := []struct {
		name   string
		setup  func(s *GameState)
		player string
		action module.Action
		want   string
	}{
		{
			name: "cannot check facing a bet",
			setup: func(s *GameState) {
				s.CurrentBet = 40
				s.Seats[0].Bet = 0
			},
			player: "p1",
			action: module.Action{Verb: VerbCheck},
			want:   ErrCannotCheck,
		},
		{
			name: "can check having matched it",
			setup: func(s *GameState) {
				s.CurrentBet = 40
				s.Seats[0].Bet, s.Seats[0].Stack = 40, 960
			},
			player: "p1",
			action: module.Action{Verb: VerbCheck},
		},
		{
			name: "cannot call with nothing to call",
			setup: func(s *GameState) {
				s.CurrentBet = 0
			},
			player: "p1",
			action: module.Action{Verb: VerbCall},
			want:   ErrNothingToCall,
		},
		{
			name: "a raise must beat the current bet",
			setup: func(s *GameState) {
				s.CurrentBet, s.MinRaise = 40, 20
			},
			player: "p1",
			action: raiseTo(40),
			want:   ErrRaiseTooSmall,
		},
		{
			name: "a raise must be at least the last raise's size",
			setup: func(s *GameState) {
				// Somebody raised from 20 to 100, so the next raise must reach
				// 180 — not 120, which is the bug of using the big blind for
				// ever.
				s.CurrentBet, s.MinRaise = 100, 80
			},
			player: "p1",
			action: raiseTo(150),
			want:   ErrRaiseTooSmall,
		},
		{
			name: "a full raise is fine",
			setup: func(s *GameState) {
				s.CurrentBet, s.MinRaise = 100, 80
			},
			player: "p1",
			action: raiseTo(180),
		},
		{
			name: "all in for less than a full raise is allowed",
			setup: func(s *GameState) {
				s.CurrentBet, s.MinRaise = 100, 80
				s.Seats[0].Stack, s.Seats[0].Bet = 120, 0
			},
			player: "p1",
			action: raiseTo(120),
		},
		{
			name: "but not more than the stack",
			setup: func(s *GameState) {
				s.CurrentBet, s.MinRaise = 100, 80
				s.Seats[0].Stack, s.Seats[0].Bet = 120, 0
			},
			player: "p1",
			action: raiseTo(200),
			want:   ErrNotEnoughChips,
		},
		{
			name: "cannot raise at all when the stack cannot beat the bet",
			setup: func(s *GameState) {
				s.CurrentBet, s.MinRaise = 100, 80
				s.Seats[0].Stack, s.Seats[0].Bet = 50, 0
			},
			player: "p1",
			action: raiseTo(50),
			want:   ErrCannotRaise,
		},
		{
			name: "a raise needs an amount",
			setup: func(s *GameState) {
				s.CurrentBet, s.MinRaise = 40, 20
			},
			player: "p1",
			action: module.Action{Verb: VerbRaise},
			want:   ErrAmountRequired,
		},
		{
			name: "and it has to be a number",
			setup: func(s *GameState) {
				s.CurrentBet, s.MinRaise = 40, 20
			},
			player: "p1",
			action: module.Action{Verb: VerbRaise, Params: map[string]string{ParamAmount: "lots"}},
			want:   ErrAmountNotNumber,
		},
		{
			name:   "not your turn",
			setup:  func(s *GameState) { s.Current = 1 },
			player: "p1",
			action: module.Action{Verb: VerbFold},
			want:   ErrNotYourTurn,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw := table(3, tc.setup)
			_, got := apply(t, raw, tc.player, tc.action)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// TestAFullRaiseReopensTheBetting, and an all-in for less does not. This is the
// rule that decides whether a player who has already acted gets another turn,
// and it is the one most often skipped.
func TestAFullRaiseReopensTheBetting(t *testing.T) {
	t.Run("a full raise gives everyone another decision", func(t *testing.T) {
		raw := table(3, func(s *GameState) {
			s.CurrentBet, s.MinRaise = 100, 80
			for i := range s.Seats {
				s.Seats[i].Bet, s.Seats[i].Stack, s.Seats[i].Acted = 100, 900, true
			}
			s.Seats[0].Acted = false
			s.Current = 0
		})
		next, code := apply(t, raw, "p1", raiseTo(180))
		if code != "" {
			t.Fatalf("raise refused: %s", code)
		}
		s := mustDecode(t, next)
		for i := 1; i < 3; i++ {
			if s.Seats[i].Acted {
				t.Errorf("seat %d should owe another decision after a full raise", i)
			}
		}
		if s.MinRaise != 80 {
			t.Errorf("min raise is %d, want 80 (the size of the raise just made)", s.MinRaise)
		}
	})

	t.Run("an all-in for less does not", func(t *testing.T) {
		raw := table(3, func(s *GameState) {
			s.CurrentBet, s.MinRaise = 100, 80
			for i := range s.Seats {
				s.Seats[i].Bet, s.Seats[i].Stack, s.Seats[i].Acted = 100, 900, true
			}
			s.Seats[0].Bet, s.Seats[0].Stack, s.Seats[0].Acted = 0, 150, false
			s.Current = 0
		})
		next, code := apply(t, raw, "p1", raiseTo(150))
		if code != "" {
			t.Fatalf("all-in refused: %s", code)
		}
		s := mustDecode(t, next)
		for i := 1; i < 3; i++ {
			if !s.Seats[i].Acted {
				t.Errorf("seat %d should not get a fresh decision from an all-in for less", i)
			}
		}
	})
}

// --- pots -------------------------------------------------------------------

// TestSidePots is the arithmetic that exists because a player can only win what
// they could lose.
func TestSidePots(t *testing.T) {
	// Three players all in for different amounts, and the short stack has the
	// best hand: it can take the main pot but not a chip more.
	s := &GameState{
		Board: []string{"2C", "7D", "9S", "JH", "KC"},
		Seats: []Seat{
			// Short stack, all in for 50, holding the nuts here (a set of nines).
			{PlayerID: "short", Committed: 50, Stack: 0, AllIn: true, Hole: []string{"9C", "9D"}},
			// Middle, all in for 150, second best (two pair).
			{PlayerID: "middle", Committed: 150, Stack: 0, AllIn: true, Hole: []string{"JC", "KD"}},
			// Big stack, in for 150 too, worst hand.
			{PlayerID: "big", Committed: 150, Stack: 0, AllIn: true, Hole: []string{"3C", "4D"}},
		},
	}
	pots := distributePots(s, []int{0, 1, 2})

	if len(pots) != 2 {
		t.Fatalf("expected a main pot and one side pot, got %d: %+v", len(pots), pots)
	}
	// Main pot: 50 from each of three players.
	if pots[0].Amount != 150 || len(pots[0].Winners) != 1 || pots[0].Winners[0] != "short" {
		t.Errorf("main pot is %+v, want 150 to short", pots[0])
	}
	// Side pot: 100 more from each of the two deeper players, which the short
	// stack is not eligible for even holding the best hand.
	if pots[1].Amount != 200 || len(pots[1].Winners) != 1 || pots[1].Winners[0] != "middle" {
		t.Errorf("side pot is %+v, want 200 to middle", pots[1])
	}
	if s.seat("short").Stack != 150 {
		t.Errorf("short took %d, want exactly the main pot (150)", s.seat("short").Stack)
	}
	if s.seat("big").Stack != 0 {
		t.Errorf("big took %d with the worst hand, want 0", s.seat("big").Stack)
	}
}

// TestSplitPot is the case that made Finished return a list of winners.
func TestSplitPot(t *testing.T) {
	s := &GameState{
		// The board plays: both players' hole cards are irrelevant.
		Board: []string{"AC", "AD", "AH", "KS", "KC"},
		Seats: []Seat{
			{PlayerID: "a", Committed: 100, Hole: []string{"2C", "3D"}},
			{PlayerID: "b", Committed: 100, Hole: []string{"4C", "5D"}},
		},
	}
	pots := distributePots(s, []int{0, 1})
	if len(pots) != 1 {
		t.Fatalf("expected one pot, got %d", len(pots))
	}
	if len(pots[0].Winners) != 2 {
		t.Fatalf("expected two winners, got %v", pots[0].Winners)
	}
	if s.Seats[0].Stack != 100 || s.Seats[1].Stack != 100 {
		t.Errorf("split gave %d and %d, want 100 each", s.Seats[0].Stack, s.Seats[1].Stack)
	}
}

// TestOddChipIsNotLost — three-way split of a pot that does not divide.
func TestOddChipIsNotLost(t *testing.T) {
	s := &GameState{
		Board: []string{"AC", "AD", "AH", "KS", "KC"},
		Seats: []Seat{
			{PlayerID: "a", Committed: 34, Hole: []string{"2C", "3D"}},
			{PlayerID: "b", Committed: 33, Hole: []string{"4C", "5D"}},
			{PlayerID: "c", Committed: 33, Hole: []string{"6C", "7D"}},
		},
	}
	before := 34 + 33 + 33
	distributePots(s, []int{0, 1, 2})
	after := 0
	for i := range s.Seats {
		after += s.Seats[i].Stack
	}
	if after != before {
		t.Errorf("%d chips came out of a %d pot", after, before)
	}
}

// TestFoldingToOnePlayerShowsNothing. A pot won without a showdown is won
// without showing, and revealing it would leak information the winner is
// entitled to keep.
func TestFoldingToOnePlayerShowsNothing(t *testing.T) {
	raw := table(2, func(s *GameState) {
		s.HandLimit = 5
		s.Pot = 30
		s.Seats[0].Bet, s.Seats[0].Committed, s.Seats[0].Stack = 0, 10, 990
		s.Seats[1].Bet, s.Seats[1].Committed, s.Seats[1].Stack = 0, 20, 980
		s.Current = 0
	})
	next, code := apply(t, raw, "p1", module.Action{Verb: VerbFold})
	if code != "" {
		t.Fatalf("fold refused: %s", code)
	}
	s := mustDecode(t, next)
	if s.LastHand == nil {
		t.Fatal("the hand should have ended")
	}
	if !s.LastHand.Uncontested {
		t.Error("a hand everybody folded out of is uncontested")
	}
	if len(s.LastHand.Shown) != 0 {
		t.Errorf("nothing should be shown, got %+v", s.LastHand.Shown)
	}
}

// --- ending -----------------------------------------------------------------

// TestALevelMatchHasTwoWinners is the poker case that forced `winners []string`
// through the whole stack.
func TestALevelMatchHasTwoWinners(t *testing.T) {
	s := &GameState{
		Status:    "active",
		HandLimit: 1,
		// Both hands already played, stacks exactly level.
		HandNumber: 1,
		Seats: []Seat{
			{PlayerID: "p1", Stack: 1000},
			{PlayerID: "p2", Stack: 1000},
		},
	}
	endMatch(s)
	if s.Status != "completed" {
		t.Fatalf("status is %q", s.Status)
	}
	if len(s.Winners) != 2 {
		t.Fatalf("winners are %v, want both seats", s.Winners)
	}

	raw, err := encode(s)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	done, winners, err := New().Finished(raw)
	if err != nil || !done {
		t.Fatalf("Finished = %v, %v", done, err)
	}
	if len(winners) != 2 {
		t.Errorf("Finished reported %v, want two winners", winners)
	}
}

// TestBustedSeatsLeave — a seat with no chips is out of the match, and the
// match ends when only one is left.
func TestBustedSeatsLeave(t *testing.T) {
	s := &GameState{
		Status:        "active",
		StartingStack: 1000,
		BigBlind:      20,
		SmallBlind:    10,
		Button:        0,
		Seed:          1,
		Seats: []Seat{
			{PlayerID: "p1", Stack: 3000},
			{PlayerID: "p2", Stack: 0},
			{PlayerID: "p3", Stack: 0},
		},
	}
	startHand(s)

	if s.Status != "completed" {
		t.Fatalf("with one seat holding chips the match is over, status is %q", s.Status)
	}
	if len(s.Winners) != 1 || s.Winners[0] != "p1" {
		t.Errorf("winners are %v, want [p1]", s.Winners)
	}
}

// TestApplyDoesNotMutateItsInput — free behind an opaque State, pinned so
// nobody reintroduces the aliasing hazard the rummy engine needed a test for.
func TestApplyDoesNotMutateItsInput(t *testing.T) {
	raw := table(3, func(s *GameState) {
		s.CurrentBet, s.MinRaise = 40, 20
	})
	before := string(raw)

	if _, code := apply(t, raw, "p1", raiseTo(60)); code != "" {
		t.Fatalf("raise refused: %s", code)
	}
	if _, err := New().LegalActions(raw, "p1"); err != nil {
		t.Fatalf("LegalActions: %v", err)
	}
	if _, err := New().View(raw, "p1"); err != nil {
		t.Fatalf("View: %v", err)
	}
	if string(raw) != before {
		t.Error("the caller's state changed underneath it")
	}
}
