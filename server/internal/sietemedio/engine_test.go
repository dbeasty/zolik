package sietemedio

import (
	"encoding/json"
	"strconv"
	"testing"

	"zolik/server/internal/module"
)

func seats(ids ...string) []module.PlayerRef {
	out := make([]module.PlayerRef, 0, len(ids))
	for _, id := range ids {
		out = append(out, module.PlayerRef{ID: id, Name: id})
	}
	return out
}

func stateOf(t *testing.T, raw module.State) *GameState {
	t.Helper()
	s, err := decode(raw)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	return s
}

func rawOf(t *testing.T, s *GameState) module.State {
	t.Helper()
	raw, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return raw
}

// pack is a deck that deals these cards in this order. The engine draws off
// the end of the slice, so a test that wrote the cards out in draw order would
// read backwards; this keeps a crafted deal readable.
func pack(cards ...string) []string {
	out := make([]string, 0, len(cards))
	for i := len(cards) - 1; i >= 0; i-- {
		out = append(out, cards[i])
	}
	return out
}

// withTable builds a hand-crafted position: a Mexicana table with p1 banking,
// p2 and p3 staked and dealt, and p2 on turn — so a rule can be pinned exactly
// rather than waited for out of a real shuffle.
func withTable(t *testing.T, mut func(*GameState)) *GameState {
	t.Helper()
	s := tableOf(module.MatchConfig{Variation: VarMexicana})
	s.Status = "active"
	s.Phase = phasePlay
	s.RoundNumber = 1
	s.RoundLimit = 5
	s.Banker = 0
	s.Current = 1
	s.Seats = []Seat{
		{PlayerID: "p1", Stack: 500, Cards: []string{"4H"}},
		{PlayerID: "p2", Stack: 490, Stake: 10, Cards: []string{"5D"}},
		{PlayerID: "p3", Stack: 490, Stake: 10, Cards: []string{"3C"}},
	}
	s.Deck = pack("2C", "3S", "4C", "5C", "6C", "7C", "JD", "QH", "KS")
	mut(s)
	return s
}

func apply(t *testing.T, s *GameState, playerID, verb string) (*GameState, error) {
	t.Helper()
	next, _, err := New().Apply(rawOf(t, s), playerID, module.Action{OfferID: verb, Verb: verb})
	if err != nil {
		return nil, err
	}
	return stateOf(t, next), nil
}

func mustApply(t *testing.T, s *GameState, playerID, verb string) *GameState {
	t.Helper()
	out, err := apply(t, s, playerID, verb)
	if err != nil {
		t.Fatalf("%s %s: %v", playerID, verb, err)
	}
	return out
}

func stake(t *testing.T, s *GameState, playerID string, amount int) (*GameState, error) {
	t.Helper()
	next, _, err := New().Apply(rawOf(t, s), playerID, module.Action{
		OfferID: OfferStake, Verb: VerbStake,
		Params: map[string]string{ParamAmount: strconv.Itoa(amount)},
	})
	if err != nil {
		return nil, err
	}
	return stateOf(t, next), nil
}

// --- the pack and the arithmetic ----------------------------------------

// TestThePackIsForty — no eights, nines or tens, and every card exactly once.
// It is the pack rather than a 52-card deck with three ranks dropped out of
// the view: a seat drawing an eight is a seat playing a different game.
func TestThePackIsForty(t *testing.T) {
	deck := buildDeck(7)
	if len(deck) != 40 {
		t.Fatalf("pack has %d cards, want 40", len(deck))
	}
	seen := map[string]int{}
	for _, c := range deck {
		seen[c]++
		switch rankOf(c) {
		case "8", "9", "T":
			t.Errorf("%q is not in the Spanish pack", c)
		}
	}
	for c, n := range seen {
		if n != 1 {
			t.Errorf("%q appears %d times", c, n)
		}
	}
}

// TestCardValues — the figures are worth half a point, which is the whole
// reason this module counts in halves.
func TestCardValues(t *testing.T) {
	for card, want := range map[string]int{
		"AD": 2, "2H": 4, "3C": 6, "4S": 8, "5D": 10, "6H": 12, "7C": 14,
		"JD": 1, "QH": 1, "KS": 1,
	} {
		if got := cardHalves(card); got != want {
			t.Errorf("%s is worth %d halves, want %d", card, got, want)
		}
	}
}

// TestHalvesText — the one place the ½ is written out, and the figure a player
// reads off the board.
func TestHalvesText(t *testing.T) {
	for halves, want := range map[int]string{
		0: "0", 1: "½", 2: "1", 13: "6½", 14: "7", 15: "7½", 16: "8",
	} {
		if got := halvesText(halves); got != want {
			t.Errorf("halvesText(%d) = %q, want %q", halves, got, want)
		}
	}
}

// TestThreeFiguresAndASevenIsSevenAndAHalf — the hand the game is named after,
// made the way it is most often made.
func TestThreeFiguresAndASevenIsSevenAndAHalf(t *testing.T) {
	hand := []string{"7D", "JH"}
	if !IsSevenAndAHalf(hand) {
		t.Fatalf("7 and a figure is %s, want 7½", halvesText(Total(hand)))
	}
	if IsBust(hand) {
		t.Fatal("7½ is not bust")
	}
	if over := []string{"7D", "JH", "QS"}; !IsBust(over) {
		t.Fatalf("%v totals %s, which should be past 7½", over, halvesText(Total(over)))
	}
}

// --- the deal and the turn order ----------------------------------------

// TestTheBankIsCutForRatherThanGivenToTheHost — the bank has a real edge at
// this game, so handing it to seat zero every match would hand that seat the
// edge. Over a spread of seeds the cut has to land somewhere else at least
// sometimes.
func TestTheBankIsCutForRatherThanGivenToTheHost(t *testing.T) {
	m := New()
	bankers := map[string]int{}
	for seed := int64(1); seed <= 40; seed++ {
		raw, err := m.NewMatch(module.MatchConfig{}, seats("p1", "p2", "p3"), seed)
		if err != nil {
			t.Fatalf("NewMatch: %v", err)
		}
		s := stateOf(t, raw)
		bankers[s.Seats[s.Banker].PlayerID]++
	}
	if len(bankers) < 3 {
		t.Errorf("the cut only ever found %d of 3 seats: %v", len(bankers), bankers)
	}
}

// TestTheBankerDoesNotStake — the bank answers the stakes; it does not make
// one. A banker shown a stake control would be shown a move the game does not
// have.
func TestTheBankerDoesNotStake(t *testing.T) {
	s := withTable(t, func(s *GameState) {
		s.Phase = phaseStakes
		s.Current = -1
		for i := range s.Seats {
			s.Seats[i].Cards = nil
			s.Seats[i].Stake = 0
		}
	})
	if _, err := stake(t, s, "p1", 10); module.CodeOf(err) != ErrBankerDoesNotStake {
		t.Fatalf("banker staking: got %v, want %s", err, ErrBankerDoesNotStake)
	}
}

// TestTheBankPlaysLast — the players act from the banker's left and the bank
// answers them, which is the order that gives the bank its advantage.
func TestTheBankPlaysLast(t *testing.T) {
	s := withTable(t, func(s *GameState) { s.Banker = 1 })
	want := []int{2, 0, 1}
	if got := playOrder(s); len(got) != 3 || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Fatalf("play order %v, want %v (the banker last)", got, want)
	}
}

// TestSevenAndAHalfStandsItself — there is no card in the pack that does not
// bust a hand on 7½, so offering a draw on it would be offering a dead
// position. The engine stands it instead.
func TestSevenAndAHalfStandsItself(t *testing.T) {
	s := withTable(t, func(s *GameState) {
		s.Seats[1].Cards = []string{"7D"}
		s.Deck = pack("JH")
	})
	s = mustApply(t, s, "p2", VerbHit)
	if !s.Seats[1].Stood {
		t.Fatalf("p2 drew to 7½ and is still being asked: %+v", s.Seats[1])
	}
	if s.Current == 1 {
		t.Error("p2 is on 7½ and still on turn")
	}
}

// TestGoingOverLosesEvenWhenTheBankGoesOverToo — the bank's whole edge, in one
// round.
func TestGoingOverLosesEvenWhenTheBankGoesOverToo(t *testing.T) {
	s := withTable(t, func(s *GameState) {
		s.Seats[1].Cards = []string{"7D", "7H"} // 14, already past 7½
		s.Seats[1].Stood, s.Seats[1].Shown = true, true
		s.Seats[2].Cards = []string{"5C"}
		s.Seats[2].Stood = true
		s.Current = 0
		s.Phase = phaseBank
		s.Seats[0].Cards = []string{"7S"}
		s.Seats[0].Shown = true
		s.Deck = pack("7C") // the bank draws to 14 as well
	})
	s = mustApply(t, s, "p1", VerbHit)

	last := s.Rounds[len(s.Rounds)-1]
	if !last.BankBust {
		t.Fatalf("the bank should have gone over: %s", halvesText(last.BankTotal))
	}
	if got := last.Outcomes["p2"]; got != outcomeBust {
		t.Errorf("p2 went over first and got %q, want %q", got, outcomeBust)
	}
	if last.Deltas["p2"] != -10 {
		t.Errorf("p2 lost %d, want -10", last.Deltas["p2"])
	}
	if got := last.Outcomes["p3"]; got != outcomeWin {
		t.Errorf("p3 stood and the bank went over, got %q, want %q", got, outcomeWin)
	}
}

// TestALevelHandGoesWhereTheTableSendsIt — the one rule the sources disagree
// about most, both ways round.
func TestALevelHandGoesWhereTheTableSendsIt(t *testing.T) {
	for _, tc := range []struct {
		name       string
		tiesToBank bool
		want       string
		payout     int
	}{
		{"bank wins ties", true, outcomeLose, 0},
		{"stakes back", false, outcomePush, 10},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := withTable(t, func(s *GameState) {
				s.TiesToBank = tc.tiesToBank
				s.Seats[1].Cards = []string{"6D"} // 12
				s.Seats[1].Stood = true
				s.Seats[2].Stake = 0
				s.Seats[2].Cards = nil
				s.Seats[0].Cards = []string{"6S"} // the bank, level on 12
				s.Phase = phaseBank
				s.Current = 0
			})
			s = mustApply(t, s, "p1", VerbStand)
			last := s.Rounds[len(s.Rounds)-1]
			if got := last.Outcomes["p2"]; got != tc.want {
				t.Fatalf("level hand settled %q, want %q", got, tc.want)
			}
			if got := s.Seats[1].Payout; got != tc.payout {
				t.Errorf("payout %d, want %d", got, tc.payout)
			}
		})
	}
}

// TestSevenAndAHalfPaysWhatTheTablePays — double at most tables, even money
// where the bonus is switched off.
func TestSevenAndAHalfPaysWhatTheTablePays(t *testing.T) {
	for _, tc := range []struct {
		name    string
		doubles bool
		payout  int
		delta   int
	}{
		{"double", true, 30, 20},
		{"even money", false, 20, 10},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := withTable(t, func(s *GameState) {
				s.SevenDoubles = tc.doubles
				s.Seats[1].Cards = []string{"7D", "JH"} // 7½
				s.Seats[1].Stood = true
				s.Seats[2].Stake = 0
				s.Seats[2].Cards = nil
				s.Seats[0].Cards = []string{"6S"}
				s.Phase = phaseBank
				s.Current = 0
			})
			s = mustApply(t, s, "p1", VerbStand)
			last := s.Rounds[len(s.Rounds)-1]
			if got := last.Outcomes["p2"]; got != outcomeSeven {
				t.Fatalf("7½ settled %q, want %q", got, outcomeSeven)
			}
			if got := s.Seats[1].Payout; got != tc.payout {
				t.Errorf("payout %d, want %d", got, tc.payout)
			}
			if got := last.Deltas["p2"]; got != tc.delta {
				t.Errorf("delta %d, want %d", got, tc.delta)
			}
			if got := last.Deltas["p1"]; got != -tc.delta {
				t.Errorf("the bank paid %d, want %d — every chip comes out of its stack", got, -tc.delta)
			}
		})
	}
}

// TestTwoSevensAndAHalfIsStillATie — the bonus is for beating the bank, not
// for holding the hand.
func TestTwoSevensAndAHalfIsStillATie(t *testing.T) {
	s := withTable(t, func(s *GameState) {
		s.TiesToBank = true
		s.Seats[1].Cards = []string{"7D", "JH"}
		s.Seats[1].Stood = true
		s.Seats[2].Stake = 0
		s.Seats[2].Cards = nil
		s.Seats[0].Cards = []string{"7S", "QD"}
		s.Phase = phaseBank
		s.Current = 0
	})
	s = mustApply(t, s, "p1", VerbStand)
	if got := s.Rounds[0].Outcomes["p2"]; got != outcomeLose {
		t.Fatalf("7½ against the bank's 7½ settled %q, want %q at a table where the bank takes ties", got, outcomeLose)
	}
}

// --- how the bank moves --------------------------------------------------

// TestTheBankPassesToWhoeverMakesSevenAndAHalf — the classic rule.
func TestTheBankPassesToWhoeverMakesSevenAndAHalf(t *testing.T) {
	s := withTable(t, func(s *GameState) {
		s.BankPass = bankOnSeven
		s.Seats[1].Cards = []string{"7D", "JH"} // 7½
		s.Seats[1].Stood = true
		s.Seats[2].Cards = []string{"4C"}
		s.Seats[2].Stood = true
		s.Seats[0].Cards = []string{"6S"}
		s.Phase = phaseBank
		s.Current = 0
	})
	s = mustApply(t, s, "p1", VerbStand)
	if got := s.Seats[s.Banker].PlayerID; got != "p2" {
		t.Fatalf("the bank went to %q, want p2 — they made 7½", got)
	}
}

// TestTheBankerKeepsItOnTheirOwnSevenAndAHalf — nobody beat them.
func TestTheBankerKeepsItOnTheirOwnSevenAndAHalf(t *testing.T) {
	s := withTable(t, func(s *GameState) {
		s.BankPass = bankOnSeven
		s.Seats[1].Cards = []string{"7D", "JH"}
		s.Seats[1].Stood = true
		s.Seats[2].Stake = 0
		s.Seats[2].Cards = nil
		s.Seats[0].Cards = []string{"7S", "QD"} // the bank is on 7½ too
		s.Phase = phaseBank
		s.Current = 0
	})
	s = mustApply(t, s, "p1", VerbStand)
	if got := s.Seats[s.Banker].PlayerID; got != "p1" {
		t.Fatalf("the bank went to %q, want p1 — they made 7½ themselves", got)
	}
}

// TestTheBankGoesRoundTheTable — the other way tables play it.
func TestTheBankGoesRoundTheTable(t *testing.T) {
	s := withTable(t, func(s *GameState) {
		s.BankPass = bankRound
		s.Seats[1].Stood = true
		s.Seats[2].Stood = true
		s.Seats[0].Cards = []string{"7S"}
		s.Phase = phaseBank
		s.Current = 0
	})
	s = mustApply(t, s, "p1", VerbStand)
	if got := s.Seats[s.Banker].PlayerID; got != "p2" {
		t.Fatalf("the bank went to %q, want p2 — the next seat round", got)
	}
}

// TestAFixedBankStaysPut — and still moves when the banker cannot answer a
// stake any more, because a bank that cannot cover is not a bank.
func TestAFixedBankStaysPut(t *testing.T) {
	s := withTable(t, func(s *GameState) {
		s.BankPass = bankFixed
		s.Seats[1].Stood = true
		s.Seats[2].Stood = true
		s.Seats[0].Cards = []string{"7S"}
		s.Phase = phaseBank
		s.Current = 0
	})
	s = mustApply(t, s, "p1", VerbStand)
	if got := s.Seats[s.Banker].PlayerID; got != "p1" {
		t.Fatalf("a fixed bank moved to %q", got)
	}
}

// --- la banca responde ---------------------------------------------------

// TestTheBankCannotBeStakedPastWhatItHolds — the only refusal in this engine
// that is about another seat's chips, and the only one no client could predict.
func TestTheBankCannotBeStakedPastWhatItHolds(t *testing.T) {
	s := withTable(t, func(s *GameState) {
		s.Phase = phaseStakes
		s.Current = -1
		s.Seats[0].Stack = 60 // the bank: 60 chips, and 7½ pays double
		for i := range s.Seats {
			s.Seats[i].Cards = nil
			s.Seats[i].Stake = 0
		}
		s.Seats[1].Stack = 500
		s.Seats[2].Stack = 500
	})
	if _, err := stake(t, s, "p2", 40); module.CodeOf(err) != ErrBankCannotCover {
		t.Fatalf("staking 40 against a 60-chip bank: got %v, want %s", err, ErrBankCannotCover)
	}
	next, err := stake(t, s, "p2", 30)
	if err != nil {
		t.Fatalf("staking 30 against a 60-chip bank: %v", err)
	}
	// 30 at double is the bank's whole stack, so nothing is left for p3 and it
	// is not waited on rather than being shown a control with no legal value.
	if !next.Seats[2].NoRoom {
		t.Errorf("p3 has no room left but is still being asked: %+v", next.Seats[2])
	}
	if awaiting(next, 2) {
		t.Errorf("p3 is awaited with nothing it could stake")
	}
}

// TestTheBankIsNeverOverdrawn — the invariant the cover limit exists for: a
// settlement that paid out more than the banker held would be a negative stack,
// which is not a state this game has.
func TestTheBankIsNeverOverdrawn(t *testing.T) {
	m := New()
	for seed := int64(1); seed <= 20; seed++ {
		cfg := module.MatchConfig{
			Variation: VarMexicana,
			Options:   module.Options{OptRounds: 12, OptStartingStack: 200, OptMinStake: 25},
		}
		state, err := m.NewMatch(cfg, seats("p1", "p2", "p3", "p4"), seed)
		if err != nil {
			t.Fatalf("NewMatch: %v", err)
		}
		final, _, err := module.PlayWithOffers(m, state, seats("p1", "p2", "p3", "p4"), module.DriverOptions{
			MaxActions: 8000, Prefer: []string{"stake", "hit", "stand"},
		})
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		s := stateOf(t, final)
		for i := range s.Seats {
			if s.Seats[i].Stack < 0 {
				t.Fatalf("seed %d: %s finished on %d chips", seed, s.Seats[i].PlayerID, s.Seats[i].Stack)
			}
		}
		for _, r := range s.Rounds {
			for pid, stack := range r.Stacks {
				if stack < 0 {
					t.Fatalf("seed %d round %d: %s on %d chips", seed, r.Number, pid, stack)
				}
			}
		}
	}
}

// TestChipsAreConservedAcrossTheTable — there is no house here. Every chip a
// seat wins came out of the banker's stack, so the whole table's chips are a
// constant rather than merely accounted for, which is a sharper invariant than
// a game against a house can state.
func TestChipsAreConservedAcrossTheTable(t *testing.T) {
	m := New()
	players := seats("p1", "p2", "p3")
	for seed := int64(1); seed <= 15; seed++ {
		cfg := module.MatchConfig{Variation: VarCantina, Options: module.Options{OptRounds: 12}}
		state, err := m.NewMatch(cfg, players, seed)
		if err != nil {
			t.Fatalf("NewMatch: %v", err)
		}
		final, _, err := module.PlayWithOffers(m, state, players, module.DriverOptions{
			MaxActions: 8000, Prefer: []string{"stake", "stand", "hit"},
		})
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		s := stateOf(t, final)
		total := 0
		for i := range s.Seats {
			total += s.Seats[i].Stack
		}
		if want := len(players) * s.StartingStack; total != want {
			t.Errorf("seed %d: the table holds %d chips, started with %d", seed, total, want)
		}
	}
}
