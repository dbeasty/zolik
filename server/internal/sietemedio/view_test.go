package sietemedio

import (
	"strings"
	"testing"

	"zolik/server/internal/module"
)

func zoneByID(vm module.ViewModel, id string) *module.Zone {
	for i := range vm.Zones {
		if vm.Zones[i].ID == id {
			return &vm.Zones[i]
		}
	}
	return nil
}

func seatByID(vm module.ViewModel, id string) *module.Seat {
	for i := range vm.Seats {
		if vm.Seats[i].PlayerID == id {
			return &vm.Seats[i]
		}
	}
	return nil
}

// TestTheFirstCardIsYoursAlone — the hidden information in this game runs the
// ordinary way round, unlike blackjack's: each seat's first card is that seat's
// own. The count still says it is there, which is how a client draws a card
// back-up without being told what it is.
func TestTheFirstCardIsYoursAlone(t *testing.T) {
	s := withTable(t, func(s *GameState) {
		s.Seats[1].Cards = []string{"7D", "2H"}
	})

	own, err := New().View(rawOf(t, s), "p2")
	if err != nil {
		t.Fatalf("View: %v", err)
	}
	z := zoneByID(own, handZoneID("p2"))
	if z == nil || len(z.Cards) != 2 {
		t.Fatalf("p2 cannot see their own hand: %+v", z)
	}

	other, err := New().View(rawOf(t, s), "p3")
	if err != nil {
		t.Fatalf("View: %v", err)
	}
	z = zoneByID(other, handZoneID("p2"))
	if z == nil {
		t.Fatal("p2's hand is missing from p3's board")
	}
	if len(z.Cards) != 1 || z.Cards[0].Card != "2H" {
		t.Errorf("p3 sees %+v of p2's hand; only the card drawn face up should be there", z.Cards)
	}
	if z.Count != 2 {
		t.Errorf("p2's hand is %d cards on p3's board, want 2 — the count is how a back is drawn", z.Count)
	}
}

// TestAPartlyHiddenHandShowsWhatIsOnShowRatherThanTheTotal — sending the real
// total would hand every viewer the card the zone just took out.
func TestAPartlyHiddenHandShowsWhatIsOnShowRatherThanTheTotal(t *testing.T) {
	s := withTable(t, func(s *GameState) {
		s.Seats[1].Cards = []string{"7D", "2H"} // 7 and 2: eleven, in halves 22
	})
	vm, err := New().View(rawOf(t, s), "p3")
	if err != nil {
		t.Fatalf("View: %v", err)
	}
	seat := seatByID(vm, "p2")
	if seat == nil {
		t.Fatal("p2 is missing from the board")
	}
	var keys []string
	for _, f := range seat.Facts {
		keys = append(keys, f.LabelKey)
		if f.LabelKey == "sietemedio.seat.total" {
			t.Errorf("p3 is told p2's whole total (%q) while a card is face down", f.Value)
		}
		if f.LabelKey == "sietemedio.seat.showing" && f.Value != "2" {
			t.Errorf("p2 is showing %q, want 2", f.Value)
		}
	}
	if !strings.Contains(strings.Join(keys, " "), "sietemedio.seat.showing") {
		t.Errorf("p2's board facts say nothing about what is on show: %v", keys)
	}
}

// TestAHandWithNothingOnShowIsGivenNoTotal — the bank before its turn, and
// every seat still on the card it was dealt. "Showing 0" is a number about a
// card nobody can see, and on a board it reads as a hand worth nothing rather
// than as one nobody has looked at.
func TestAHandWithNothingOnShowIsGivenNoTotal(t *testing.T) {
	s := withTable(t, func(s *GameState) {
		s.Seats[1].Cards = []string{"7D"}
	})
	vm, err := New().View(rawOf(t, s), "p3")
	if err != nil {
		t.Fatalf("View: %v", err)
	}
	seat := seatByID(vm, "p2")
	if seat == nil {
		t.Fatal("p2 is missing from the board")
	}
	for _, f := range seat.Facts {
		if f.LabelKey == "sietemedio.seat.showing" || f.LabelKey == "sietemedio.seat.total" {
			t.Errorf("p2 has one card, face down, and the board says %q: %q", f.LabelKey, f.Value)
		}
	}
}

// TestTheBankTurnsItsCardOverWhenItsTurnComes — the one piece of ceremony in
// this game, and the moment the whole table learns what it is playing against.
func TestTheBankTurnsItsCardOverWhenItsTurnComes(t *testing.T) {
	s := withTable(t, func(s *GameState) {
		s.Seats[0].Cards = []string{"4H"}
		s.Seats[1].Stood = true
		s.Seats[2].Cards = []string{"5C"}
		s.Current = 2
	})

	before, err := New().View(rawOf(t, s), "p2")
	if err != nil {
		t.Fatalf("View: %v", err)
	}
	if z := zoneByID(before, handZoneID("p1")); z == nil || len(z.Cards) != 0 || z.Count != 1 {
		t.Fatalf("the bank's card is visible before its turn: %+v", z)
	}

	// p3 stands, which is the last player — the bank's turn starts.
	s = mustApply(t, s, "p3", VerbStand)
	if s.Phase != phaseBank {
		t.Fatalf("phase is %q, want %q", s.Phase, phaseBank)
	}
	after, err := New().View(rawOf(t, s), "p2")
	if err != nil {
		t.Fatalf("View: %v", err)
	}
	if z := zoneByID(after, handZoneID("p1")); z == nil || len(z.Cards) != 1 {
		t.Fatalf("the bank did not turn its card over: %+v", z)
	}
}

// TestAnOpenTableHidesNothing — the Abierta variation is the table to learn
// the game at, and it is the FirstCardDown option that makes it one.
func TestAnOpenTableHidesNothing(t *testing.T) {
	m := New()
	raw, err := m.NewMatch(module.MatchConfig{Variation: VarAbierta}, table(), 3)
	if err != nil {
		t.Fatalf("NewMatch: %v", err)
	}
	// Stake everybody in, which deals the round.
	s := stateOf(t, raw)
	for i := range s.Seats {
		if s.isBanker(i) || s.Seats[i].NoRoom {
			continue
		}
		s, err = stake(t, s, s.Seats[i].PlayerID, s.MinStake)
		if err != nil {
			t.Fatalf("stake: %v", err)
		}
	}
	vm, err := m.View(rawOf(t, s), "p3")
	if err != nil {
		t.Fatalf("View: %v", err)
	}
	for i := range s.Seats {
		z := zoneByID(vm, handZoneID(s.Seats[i].PlayerID))
		if z == nil {
			continue
		}
		if len(z.Cards) != z.Count {
			t.Errorf("%s shows %d of %d cards at an open table", s.Seats[i].PlayerID, len(z.Cards), z.Count)
		}
	}
}

// TestOpenViewRevealsEverything — a game that is over has no secrets, which is
// what a replay needs.
func TestOpenViewRevealsEverything(t *testing.T) {
	s := withTable(t, func(s *GameState) {})
	vm, err := New().OpenView(rawOf(t, s))
	if err != nil {
		t.Fatalf("OpenView: %v", err)
	}
	for i := range s.Seats {
		z := zoneByID(vm, handZoneID(s.Seats[i].PlayerID))
		if z == nil || len(z.Cards) != len(s.Seats[i].Cards) {
			t.Errorf("%s's hand is still hidden in an open view: %+v", s.Seats[i].PlayerID, z)
		}
	}
}

// TestTheBoardSaysWhoHoldsTheBank — the bank moves during the match, so "who
// am I playing against" is a fact the board has to carry rather than something
// a client could infer from seat order.
func TestTheBoardSaysWhoHoldsTheBank(t *testing.T) {
	s := withTable(t, func(s *GameState) { s.Banker = 2 })
	vm, err := New().View(rawOf(t, s), "p1")
	if err != nil {
		t.Fatalf("View: %v", err)
	}
	seat := seatByID(vm, "p3")
	if seat == nil {
		t.Fatal("p3 is missing from the board")
	}
	found := false
	for _, k := range seat.LabelKeys {
		if k == "sietemedio.seat.bank" {
			found = true
		}
	}
	if !found {
		t.Errorf("p3 holds the bank but the board does not say so: %v", seat.LabelKeys)
	}
	for _, f := range vm.Header {
		if f.LabelKey == "sietemedio.header.bank" {
			if f.Params["playerId"] != "p3" {
				t.Errorf("the header names %v as the bank, want p3", f.Params["playerId"])
			}
			return
		}
	}
	t.Error("the header does not name the bank")
}

// TestStandingsRankByChips — the only measure this game has.
func TestStandingsRankByChips(t *testing.T) {
	s := withTable(t, func(s *GameState) {
		s.Seats[0].Stack = 100
		s.Seats[1].Stack = 900
		s.Seats[2].Stack = 500
	})
	standings, err := New().Standings(rawOf(t, s))
	if err != nil {
		t.Fatalf("Standings: %v", err)
	}
	if len(standings) != 3 || standings[0].PlayerID != "p2" || standings[2].PlayerID != "p1" {
		t.Fatalf("standings are %+v", standings)
	}
}
