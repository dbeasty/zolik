package holdem

import (
	"math/rand"
	"slices"
	"strconv"
	"testing"

	"zolik/server/internal/module"
)

// tableChips is every chip at the table: in front of the seats, in the pot,
// and bet on the street not yet swept in. Seats that got up still count —
// their chips are frozen in front of them, not gone.
func tableChips(s *GameState) int {
	n := s.Pot
	for i := range s.Seats {
		n += s.Seats[i].Stack + s.Seats[i].Bet
	}
	return n
}

// shortStacks is the repro for chips vanishing from a cash table: four seats,
// two of them short, nothing posted, p1 to act.
func shortStacks(stacks ...int) module.State {
	return table(len(stacks), func(s *GameState) {
		s.Format = FormatCash
		s.Board = nil
		for i := range s.Seats {
			s.Seats[i].Stack = stacks[i]
		}
		s.Seats[0].Hole = []string{"AS", "AH"}
		s.Seats[1].Hole = []string{"KS", "KH"}
		s.Seats[2].Hole = []string{"QS", "QH"}
		s.Seats[3].Hole = []string{"JS", "JH"}
		s.MinRaise = 20
	})
}

func mustApply(t *testing.T, raw module.State, player string, a module.Action) module.State {
	t.Helper()
	next, code := apply(t, raw, player, a)
	if code != "" {
		t.Fatalf("%s %s refused: %s", player, a.Verb, code)
	}
	return next
}

// A raiser who gets up over two short all-ins leaves chips above both that
// neither could ever win. They used to be awarded to nobody, and the table
// lost them; they go back to the player who put them in.
func TestChipsNobodyCanWinGoBack(t *testing.T) {
	raw := shortStacks(1000, 50, 80, 1000)
	start := tableChips(mustDecode(t, raw))

	raw = mustApply(t, raw, "p1", raiseTo(200))
	raw = mustApply(t, raw, "p2", module.Action{Verb: VerbCall}) // all in, 50
	raw = mustApply(t, raw, "p3", module.Action{Verb: VerbCall}) // all in, 80
	raw = mustApply(t, raw, "p1", module.Action{Verb: module.VerbLeave})
	raw = mustApply(t, raw, "p4", module.Action{Verb: VerbFold})

	s := mustDecode(t, raw)
	if s.LastHand == nil {
		t.Fatal("the hand should have run out to a showdown")
	}
	if got := tableChips(s); got != start {
		t.Fatalf("table holds %d chips after the hand, want %d", got, start)
	}
	want := []Returned{{PlayerID: "p1", Amount: 120}}
	if !slices.Equal(s.LastHand.Returned, want) {
		t.Errorf("returned %+v, want %+v", s.LastHand.Returned, want)
	}
	if got := s.seat("p1").Stack; got != 920 {
		t.Errorf("p1 left with %d, want 920 (1000 less the 80 the all-ins matched)", got)
	}
	// The pots are only what the two all-ins could win: 50 from three seats,
	// then 30 more from p1 and p3.
	total := 0
	for _, p := range s.LastHand.Pots {
		total += p.Amount
	}
	if total != 210 {
		t.Errorf("pots total %d, want 210: %+v", total, s.LastHand.Pots)
	}

	vm, err := New().View(raw, "p2")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range vm.Status {
		if f.LabelKey == "holdem.status.returned" && f.Params["playerId"] == "p1" && f.Params["amount"] == 120 {
			found = true
		}
	}
	if !found {
		t.Errorf("no status line says p1 got 120 back: %+v", vm.Status)
	}
}

// Left alone in the hand, a player wins what they could have lost, not the
// chips above it — and a raise nobody called is handed back, not won.
func TestASoleContenderWinsOnlyWhatTheyCovered(t *testing.T) {
	t.Run("all in, raiser gets up", func(t *testing.T) {
		raw := shortStacks(1000, 50, 1000, 1000)
		start := tableChips(mustDecode(t, raw))

		raw = mustApply(t, raw, "p1", raiseTo(200))
		raw = mustApply(t, raw, "p2", module.Action{Verb: VerbCall}) // all in, 50
		raw = mustApply(t, raw, "p1", module.Action{Verb: module.VerbLeave})
		raw = mustApply(t, raw, "p3", module.Action{Verb: VerbFold})
		raw = mustApply(t, raw, "p4", module.Action{Verb: VerbFold})

		s := mustDecode(t, raw)
		if s.LastHand == nil || !s.LastHand.Uncontested {
			t.Fatalf("want an uncontested hand, got %+v", s.LastHand)
		}
		if got := tableChips(s); got != start {
			t.Fatalf("table holds %d chips, want %d", got, start)
		}
		if got := s.seat("p2").Stack; got != 100 {
			t.Errorf("p2 has %d, want 100 — its 50 and p1's 50 that matched it", got)
		}
		if got := s.seat("p1").Stack; got != 950 {
			t.Errorf("p1 has %d, want 950 — the 150 nobody matched comes back", got)
		}
		if p := s.LastHand.Pots; len(p) != 1 || p[0].Amount != 100 || p[0].Winners[0] != "p2" {
			t.Errorf("pots %+v, want one of 100 to p2", p)
		}
	})

	t.Run("everyone folds to a raise", func(t *testing.T) {
		raw := table(3, func(s *GameState) {
			s.Seats[1].Bet, s.Seats[1].Committed, s.Seats[1].Stack = 10, 10, 990
			s.Seats[2].Bet, s.Seats[2].Committed, s.Seats[2].Stack = 20, 20, 980
			s.CurrentBet = 20
			s.Current = 0
		})
		start := tableChips(mustDecode(t, raw))
		raw = mustApply(t, raw, "p1", raiseTo(60))
		raw = mustApply(t, raw, "p2", module.Action{Verb: VerbFold})
		raw = mustApply(t, raw, "p3", module.Action{Verb: VerbFold})

		s := mustDecode(t, raw)
		if got := tableChips(s); got != start {
			t.Fatalf("table holds %d chips, want %d", got, start)
		}
		if got := s.seat("p1").Stack; got != 1030 {
			t.Errorf("p1 has %d, want 1030", got)
		}
		if p := s.LastHand.Pots; len(p) != 1 || p[0].Amount != 50 {
			t.Errorf("pots %+v, want 50 won (the blinds and the 20 p3 matched)", p)
		}
		want := []Returned{{PlayerID: "p1", Amount: 40}}
		if !slices.Equal(s.LastHand.Returned, want) {
			t.Errorf("returned %+v, want %+v", s.LastHand.Returned, want)
		}
	})
}

// An all-in for less than a full raise does not reopen the betting, so a seat
// that already acted may call the difference or fold — not raise again.
func TestNoReraiseAfterAShortAllIn(t *testing.T) {
	raw := table(3, func(s *GameState) {
		s.Seats[2].Stack = 150
	})
	raw = mustApply(t, raw, "p1", raiseTo(100))
	raw = mustApply(t, raw, "p2", module.Action{Verb: VerbCall})
	raw = mustApply(t, raw, "p3", raiseTo(150)) // all in, short of the 200 a full raise needs

	if s := mustDecode(t, raw); s.Seats[s.Current].PlayerID != "p1" {
		t.Fatalf("p1 should be asked for the 50, current is %s", s.Seats[s.Current].PlayerID)
	}
	if _, code := apply(t, raw, "p1", raiseTo(400)); code != ErrBettingNotReopened {
		t.Errorf("re-raise: code %q, want %s", code, ErrBettingNotReopened)
	}
	offers, err := New().LegalActions(raw, "p1")
	if err != nil {
		t.Fatal(err)
	}
	raise := hasOffer(offers, VerbRaise)
	if raise == nil || raise.Enabled || raise.WhyNot != ErrBettingNotReopened {
		t.Errorf("raise offer = %+v, want disabled with %s", raise, ErrBettingNotReopened)
	}
	if call := hasOffer(offers, VerbCall); call == nil || !call.Enabled {
		t.Errorf("call offer = %+v, want enabled", call)
	}
	mustApply(t, raw, "p1", module.Action{Verb: VerbCall})
}

// The odd chip of a split goes to the first winner clockwise from the button,
// not to whichever winner has the lowest seat number.
func TestOddChipGoesClockwiseFromTheButton(t *testing.T) {
	s := &GameState{
		Button: 1,
		Board:  []string{"AC", "AD", "AH", "KS", "KC"},
		Seats: []Seat{
			{PlayerID: "a", Committed: 10, Hole: []string{"2C", "3D"}},
			{PlayerID: "b", Committed: 1, Folded: true},
			{PlayerID: "c", Committed: 10, Hole: []string{"4C", "5D"}},
		},
	}
	pots := distributePots(s, []int{0, 2})
	if len(pots) != 1 || !slices.Equal(pots[0].Winners, []string{"c", "a"}) {
		t.Fatalf("pots %+v, want one split c, a", pots)
	}
	if a, c := s.seat("a").Stack, s.seat("c").Stack; c != 11 || a != 10 {
		t.Errorf("split gave a %d and c %d, want 10 and 11 — c is first left of the button", a, c)
	}
}

// Chips are never made or lost, whatever the players do: random play at cash
// tables in every variation, with players getting up mid-hand at random.
func TestChipsAreConserved(t *testing.T) {
	for _, variation := range []string{"", VarOmaha, VarDraw} {
		for seed := int64(1); seed <= 60; seed++ {
			playOut(t, variation, seed)
		}
	}
}

func playOut(t *testing.T, variation string, seed int64) {
	t.Helper()
	rnd := rand.New(rand.NewSource(seed))
	seats := 3 + rnd.Intn(4)
	ids := make([]string, seats)
	for i := range ids {
		ids[i] = "p" + strconv.Itoa(i+1)
	}
	m := New()
	raw, err := m.NewMatch(module.MatchConfig{
		Variation: variation,
		Options:   module.Options{OptFormat: FormatCash},
	}, people(ids...), seed)
	if err != nil {
		t.Fatalf("new match: %v", err)
	}
	start := tableChips(mustDecode(t, raw))
	where := func(step int) string {
		return "variation " + strconv.Quote(variation) + " seed " + strconv.FormatInt(seed, 10) + " step " + strconv.Itoa(step)
	}

	for step := 0; step < 2000; step++ {
		s := mustDecode(t, raw)
		if s.Status != "active" {
			return
		}
		if got := tableChips(s); got != start {
			t.Fatalf("%s: table holds %d chips, want %d", where(step), got, start)
		}

		// Somebody gets up now and then, on turn or not.
		if rnd.Intn(25) == 0 {
			var sitting []string
			for _, st := range s.Seats {
				if !st.Left {
					sitting = append(sitting, st.PlayerID)
				}
			}
			if len(sitting) > 1 {
				id := sitting[rnd.Intn(len(sitting))]
				if next, _, err := m.Apply(raw, id, module.Action{Verb: module.VerbLeave}); err == nil {
					raw = next
					continue
				}
			}
		}

		if s.Break.Open {
			for _, id := range order(s) {
				if !s.Break.Ready[id] {
					raw = mustApply(t, raw, id, module.Action{Verb: module.VerbContinue})
					break
				}
			}
			continue
		}

		if s.Current < 0 {
			t.Fatalf("%s: active hand with nobody on turn", where(step))
		}
		seat := &s.Seats[s.Current]
		offers, err := m.LegalActions(raw, seat.PlayerID)
		if err != nil {
			t.Fatalf("%s: offers: %v", where(step), err)
		}
		var enabled []module.ActionOffer
		for _, o := range offers {
			if o.Enabled && o.Verb != module.VerbLeave {
				enabled = append(enabled, o)
			}
		}
		if len(enabled) == 0 {
			t.Fatalf("%s: %s on turn with nothing to do", where(step), seat.PlayerID)
		}
		o := enabled[rnd.Intn(len(enabled))]
		a := module.Action{Verb: o.Verb}
		switch o.Verb {
		case VerbRaise:
			lo, hi := o.Params[0].Min, o.Params[0].Max
			amount := lo
			if hi > lo && rnd.Intn(3) > 0 {
				amount = lo + rnd.Intn(hi-lo+1)
			}
			a.Params = map[string]string{ParamAmount: strconv.Itoa(amount)}
		case VerbDiscard:
			a.Cards = append([]string(nil), seat.Hole[:1+rnd.Intn(min(3, len(seat.Hole)))]...)
		}
		next, _, err := m.Apply(raw, seat.PlayerID, a)
		if err != nil {
			t.Fatalf("%s: %s %s %v refused: %v", where(step), seat.PlayerID, a.Verb, a.Params, err)
		}
		raw = next
	}
}
