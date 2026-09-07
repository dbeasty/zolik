package blackjack

import (
	"encoding/json"
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

func newMatch(t *testing.T, seed int64, cfg module.MatchConfig) module.State {
	t.Helper()
	st, err := New().NewMatch(cfg, seats("p1", "p2"), seed)
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

// stack is a shoe that deals these cards in this order. The engine draws off
// the end of the slice, so a test that wrote the cards out in draw order
// would read backwards; this keeps a crafted deal readable.
func stack(cards ...string) []string {
	out := make([]string, 0, len(cards))
	for i := len(cards) - 1; i >= 0; i-- {
		out = append(out, cards[i])
	}
	return out
}

// withTable builds a hand-crafted position: a Vegas table mid-round with p1 on
// turn, so a rule can be pinned exactly rather than waited for out of a real
// shoe.
func withTable(t *testing.T, mut func(*GameState)) module.State {
	t.Helper()
	s := tableOf(module.MatchConfig{Variation: "vegas"})
	s.Status = "active"
	s.Phase = phasePlay
	s.RoundNumber = 1
	s.RoundLimit = 5
	s.Peeked = true
	s.Current, s.CurrentHand = 0, 0
	s.Seats = []Seat{
		{PlayerID: "p1", Stack: 490, Bet: 10, Staked: 10, Hands: []Hand{
			{ID: handID("p1", 0), Cards: []string{"9H", "7D"}, Bet: 10},
		}},
		{PlayerID: "p2", Stack: 490, Bet: 10, Staked: 10, Hands: []Hand{
			{ID: handID("p2", 0), Cards: []string{"TS", "8C"}, Bet: 10, Done: true},
		}},
	}
	s.Dealer = []string{"6S", "TD"}
	s.Shoe = stack("2C", "3C", "4C", "5C", "6C", "7C", "8C", "9C")
	mut(s)
	raw, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return raw
}

func apply(t *testing.T, raw module.State, playerID string, verb string) (module.State, error) {
	t.Helper()
	next, _, err := New().Apply(raw, playerID, module.Action{OfferID: verb, Verb: verb})
	return next, err
}

func bet(t *testing.T, raw module.State, playerID string, amount int) (module.State, error) {
	t.Helper()
	next, _, err := New().Apply(raw, playerID, module.Action{
		OfferID: OfferBet, Verb: VerbBet,
		Params: map[string]string{ParamAmount: itoa(amount)},
	})
	return next, err
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		return "-" + string(b)
	}
	return string(b)
}

// seatOf finds a seat by id, for assertions.
func seatOf(t *testing.T, s *GameState, id string) *Seat {
	t.Helper()
	st := s.seat(id)
	if st == nil {
		t.Fatalf("no seat %q", id)
	}
	return st
}

// --- betting -------------------------------------------------------------

func TestNewMatch_OpensWithEverySeatBetting(t *testing.T) {
	s := stateOf(t, newMatch(t, 7, module.MatchConfig{}))
	if s.Phase != phaseBets {
		t.Fatalf("a fresh match is in phase %q, want %q", s.Phase, phaseBets)
	}
	if len(s.Dealer) != 0 {
		t.Errorf("cards were dealt before anybody staked anything: %v", s.Dealer)
	}
	for _, seat := range s.Seats {
		if seat.Stack != s.StartingStack {
			t.Errorf("%s sat down with %d, want %d", seat.PlayerID, seat.Stack, s.StartingStack)
		}
	}
}

func TestBet_RefusesBelowTheTableMinimumAndAboveTheStack(t *testing.T) {
	raw := newMatch(t, 7, module.MatchConfig{Options: module.Options{OptMinBet: 25}})

	if _, err := bet(t, raw, "p1", 24); module.CodeOf(err) != ErrBetTooSmall {
		t.Errorf("a stake under the minimum was refused with %v, want %s", err, ErrBetTooSmall)
	}
	if _, err := bet(t, raw, "p1", 100000); module.CodeOf(err) != ErrNotEnoughChips {
		t.Errorf("a stake over the stack was refused with %v, want %s", err, ErrNotEnoughChips)
	}
	next, err := bet(t, raw, "p1", 25)
	if err != nil {
		t.Fatalf("a legal stake was refused: %v", err)
	}
	if _, err := bet(t, next, "p1", 25); module.CodeOf(err) != ErrAlreadyBet {
		t.Errorf("betting twice was refused with %v, want %s", err, ErrAlreadyBet)
	}
}

func TestBet_TheLastStakeDealsTheRound(t *testing.T) {
	raw := newMatch(t, 7, module.MatchConfig{})
	raw, err := bet(t, raw, "p1", 10)
	if err != nil {
		t.Fatalf("bet: %v", err)
	}
	if s := stateOf(t, raw); len(s.Dealer) != 0 {
		t.Fatal("the table dealt before every seat had staked")
	}
	raw, err = bet(t, raw, "p2", 10)
	if err != nil {
		t.Fatalf("bet: %v", err)
	}

	s := stateOf(t, raw)
	if len(s.Dealer) != 2 {
		t.Fatalf("the dealer has %d cards, want 2", len(s.Dealer))
	}
	for _, seat := range s.Seats {
		if len(seat.Hands) != 1 || len(seat.Hands[0].Cards) != 2 {
			t.Fatalf("%s was dealt %+v, want one hand of two cards", seat.PlayerID, seat.Hands)
		}
		if seat.Stack != s.StartingStack-10 {
			t.Errorf("%s has %d chips behind, want %d", seat.PlayerID, seat.Stack, s.StartingStack-10)
		}
	}
}

// --- hitting, standing and busting ---------------------------------------

func TestHit_BustEndsTheHandWithNoFurtherDecision(t *testing.T) {
	raw := withTable(t, func(s *GameState) {
		s.Seats[0].Hands[0].Cards = []string{"TS", "6H"}
		s.Shoe = stack("TC")
	})
	next, err := apply(t, raw, "p1", VerbHit)
	if err != nil {
		t.Fatalf("hit: %v", err)
	}
	s := stateOf(t, next)
	hand := s.Seats[0].Hands[0]
	if !IsBust(hand.Cards) || !hand.Done {
		t.Fatalf("26 should be a finished, busted hand: %+v", hand)
	}
}

func TestHit_TwentyOneNeedsNoDecision(t *testing.T) {
	raw := withTable(t, func(s *GameState) {
		s.Seats[0].Hands[0].Cards = []string{"TS", "6H"}
		s.Shoe = stack("5C")
	})
	next, err := apply(t, raw, "p1", VerbHit)
	if err != nil {
		t.Fatalf("hit: %v", err)
	}
	if hand := stateOf(t, next).Seats[0].Hands[0]; !hand.Done {
		t.Fatalf("a hand on twenty-one is still being asked for a move: %+v", hand)
	}
}

func TestBust_LosesEvenWhenTheDealerBustsToo(t *testing.T) {
	raw := withTable(t, func(s *GameState) {
		s.Seats[0].Hands[0].Cards = []string{"TS", "6H"}
		s.Seats[1].Hands[0] = Hand{ID: handID("p2", 0), Cards: []string{"TS", "8C"}, Bet: 10, Done: true}
		s.Dealer = []string{"TH", "6D"}
		// p1 busts on the ten; the dealer then draws a ten of their own.
		s.Shoe = stack("TC", "TD")
	})
	next, err := apply(t, raw, "p1", VerbHit)
	if err != nil {
		t.Fatalf("hit: %v", err)
	}

	s := stateOf(t, next)
	if !s.Rounds[0].DealerBust {
		t.Fatalf("this round was set up for the dealer to bust: %+v", s.Rounds[0])
	}
	if got := s.Seats[0].Hands[0].Outcome; got != outcomeBust {
		t.Errorf("p1 busted first and got %q", got)
	}
	if got := s.Rounds[0].Deltas["p1"]; got != -10 {
		t.Errorf("p1's busted stake came back as %d, want -10", got)
	}
	if got := s.Rounds[0].Deltas["p2"]; got != 10 {
		t.Errorf("p2 stood and the dealer busted, so p2 is %d up, want 10", got)
	}
}

// --- the dealer's own rule -------------------------------------------------

func TestDealer_StandsOrHitsSoft17AsTheTableIsSet(t *testing.T) {
	for _, tc := range []struct {
		name  string
		hits  bool
		cards int
	}{
		{name: "stands", hits: false, cards: 2},
		{name: "hits", hits: true, cards: 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := withTable(t, func(s *GameState) {
				s.DealerHitsSoft17 = tc.hits
				s.Dealer = []string{"AS", "6D"} // soft seventeen
				s.Shoe = stack("2C")
			})
			next, err := apply(t, raw, "p1", VerbStand)
			if err != nil {
				t.Fatalf("stand: %v", err)
			}
			if got := len(stateOf(t, next).Dealer); got != tc.cards {
				t.Errorf("the dealer finished on %d cards, want %d", got, tc.cards)
			}
		})
	}
}

func TestDealer_DoesNotDrawWhenThereIsNothingLeftToBeat(t *testing.T) {
	raw := withTable(t, func(s *GameState) {
		s.Seats[0].Hands[0] = Hand{ID: handID("p1", 0), Cards: []string{"TS", "9H", "8D"}, Bet: 10}
		s.Seats[1].Hands[0] = Hand{ID: handID("p2", 0), Cards: []string{"TC", "9S", "7D"}, Bet: 10, Done: true}
		s.Dealer = []string{"6S", "5D"} // eleven: it would certainly draw
		s.Shoe = stack("2C", "3C")
	})
	next, err := apply(t, raw, "p1", VerbStand)
	if err != nil {
		t.Fatalf("stand: %v", err)
	}
	if got := len(stateOf(t, next).Dealer); got != 2 {
		t.Errorf("every hand was already bust, so the dealer drew %d cards it did not need", got-2)
	}
}

// --- naturals ------------------------------------------------------------

func TestBlackjack_PaysTheTableRate(t *testing.T) {
	for _, tc := range []struct {
		name   string
		pays   int
		payout int
	}{
		{name: "3:2", pays: pays3to2, payout: 15},
		{name: "6:5", pays: pays6to5, payout: 12},
		{name: "even money", pays: paysEven, payout: 10},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := withTable(t, func(s *GameState) {
				s.BlackjackPayout = tc.pays
				s.Seats[0].Hands[0] = Hand{ID: handID("p1", 0), Cards: []string{"AS", "KH"}, Bet: 10, Done: true}
				s.Seats[1].Hands[0] = Hand{ID: handID("p2", 0), Cards: []string{"TS", "8C"}, Bet: 10}
				s.Current, s.CurrentHand = 1, 0
				s.Dealer = []string{"9S", "8D"}
			})
			next, err := apply(t, raw, "p2", VerbStand)
			if err != nil {
				t.Fatalf("stand: %v", err)
			}
			s := stateOf(t, next)
			if got := s.Seats[0].Hands[0].Outcome; got != outcomeBlackjack {
				t.Fatalf("a natural settled as %q", got)
			}
			if got := s.Rounds[0].Deltas["p1"]; got != tc.payout {
				t.Errorf("a natural on a stake of 10 won %d, want %d", got, tc.payout)
			}
		})
	}
}

func TestBlackjack_PushesAgainstADealerBlackjack(t *testing.T) {
	// A stacked shoe dealt through the engine's own order — one card round
	// the table, then a second — so this is the real deal path, peek
	// included, rather than a position dropped into the middle of one.
	raw := withTable(t, func(s *GameState) {
		s.Phase = phaseBets
		s.AllowInsurance = false
		s.Current, s.CurrentHand = -1, -1
		s.Peeked = false
		s.Dealer = nil
		s.Seats[0] = Seat{PlayerID: "p1", Stack: 500}
		s.Seats[1] = Seat{PlayerID: "p2", Stack: 500}
		s.Shoe = stack("AS", "9C", "AD", "KH", "8D", "QC")
	})
	raw = betAll(t, raw, 10)

	s := stateOf(t, raw)
	if len(s.Rounds) != 1 {
		t.Fatalf("a dealer blackjack settles the round where it is found; phase is %q", s.Phase)
	}
	if !s.Rounds[0].DealerBlackjack {
		t.Fatalf("the dealer was dealt %v, which this test needs to be a blackjack", s.Dealer)
	}
	if got := s.Seats[0].Hands[0].Outcome; got != outcomePush {
		t.Errorf("natural against natural settled as %q, want a push", got)
	}
	if got := s.Rounds[0].Deltas["p1"]; got != 0 {
		t.Errorf("the push moved %d chips", got)
	}
	if got := s.Rounds[0].Deltas["p2"]; got != -10 {
		t.Errorf("the seat without a natural lost %d, want 10", -got)
	}
}

// --- doubling ------------------------------------------------------------

func TestDouble_TakesOneCardAndDoublesTheStake(t *testing.T) {
	raw := withTable(t, func(s *GameState) {
		s.Seats[0].Hands[0].Cards = []string{"6H", "5D"}
		s.Shoe = stack("9C")
	})
	next, err := apply(t, raw, "p1", VerbDouble)
	if err != nil {
		t.Fatalf("double: %v", err)
	}
	s := stateOf(t, next)
	hand := s.Seats[0].Hands[0]
	if hand.Bet != 20 || !hand.Doubled {
		t.Errorf("the stake went to %d, want 20 and marked doubled: %+v", hand.Bet, hand)
	}
	if len(hand.Cards) != 3 || !hand.Done {
		t.Errorf("a double takes exactly one card and ends the hand: %+v", hand)
	}
	if got := seatOf(t, s, "p1").Staked; got != 20 {
		t.Errorf("the seat has staked %d, want 20", got)
	}
}

func TestDouble_RefusedOnceTheHandHasThreeCards(t *testing.T) {
	raw := withTable(t, func(s *GameState) {
		s.Seats[0].Hands[0].Cards = []string{"2H", "3D", "4C"}
	})
	if _, err := apply(t, raw, "p1", VerbDouble); module.CodeOf(err) != ErrCannotDouble {
		t.Errorf("doubling a drawn-to hand was refused with %v, want %s", err, ErrCannotDouble)
	}
}

func TestDouble_HonoursTheDoubleAfterSplitRule(t *testing.T) {
	for _, allowed := range []bool{true, false} {
		raw := withTable(t, func(s *GameState) {
			s.DoubleAfterSplit = allowed
			s.Seats[0].Hands[0] = Hand{
				ID: handID("p1", 0), Cards: []string{"5H", "6D"}, Bet: 10, Split: true,
			}
		})
		_, err := apply(t, raw, "p1", VerbDouble)
		if allowed && err != nil {
			t.Errorf("double after split is on, but was refused: %v", err)
		}
		if !allowed && module.CodeOf(err) != ErrCannotDouble {
			t.Errorf("double after split is off, refusal was %v, want %s", err, ErrCannotDouble)
		}
	}
}

// --- splitting -----------------------------------------------------------

func TestSplit_MakesTwoHandsEachWithItsOwnStake(t *testing.T) {
	raw := withTable(t, func(s *GameState) {
		s.Seats[0].Hands[0].Cards = []string{"8H", "8D"}
		s.Shoe = stack("3C", "9S")
	})
	next, err := apply(t, raw, "p1", VerbSplit)
	if err != nil {
		t.Fatalf("split: %v", err)
	}
	s := stateOf(t, next)
	seat := seatOf(t, s, "p1")
	if len(seat.Hands) != 2 {
		t.Fatalf("a split made %d hands", len(seat.Hands))
	}
	if got := []string{seat.Hands[0].Cards[0], seat.Hands[1].Cards[0]}; got[0] != "8H" || got[1] != "8D" {
		t.Errorf("the pair was split into %v", got)
	}
	if len(seat.Hands[0].Cards) != 2 {
		t.Errorf("the hand in play was not dealt its second card: %+v", seat.Hands[0])
	}
	if seat.Staked != 20 {
		t.Errorf("the second hand's stake was not taken: staked %d, want 20", seat.Staked)
	}
	if !seat.Hands[0].Split || !seat.Hands[1].Split {
		t.Error("both hands should be marked as coming from a split")
	}
}

func TestSplit_TensAreSplitByValueNotByRank(t *testing.T) {
	raw := withTable(t, func(s *GameState) {
		s.Seats[0].Hands[0].Cards = []string{"KH", "QD"}
	})
	if _, err := apply(t, raw, "p1", VerbSplit); err != nil {
		t.Errorf("a king and a queen are two ten-value cards and split: %v", err)
	}
}

func TestSplit_AcesTakeOneCardEachAndTwentyOneIsNotABlackjack(t *testing.T) {
	raw := withTable(t, func(s *GameState) {
		s.Seats[0].Hands[0].Cards = []string{"AH", "AD"}
		s.Seats[1].Hands[0].Done = true
		s.Dealer = []string{"9S", "8D"}
		s.Shoe = stack("KC", "QS")
	})
	next, err := apply(t, raw, "p1", VerbSplit)
	if err != nil {
		t.Fatalf("split: %v", err)
	}
	s := stateOf(t, next)
	seat := seatOf(t, s, "p1")
	for i, h := range seat.Hands {
		if len(h.Cards) != 2 || !h.Done {
			t.Fatalf("split ace hand %d is %+v, want two cards and no further decision", i, h)
		}
	}
	if got := seat.Hands[0].Outcome; got != outcomeWin {
		t.Errorf("twenty-one after splitting aces settled as %q, want a plain win", got)
	}
	if got := s.Rounds[0].Deltas["p1"]; got != 20 {
		t.Errorf("two winning split hands on stakes of 10 paid %d, want 20 — a blackjack rate would pay more", got)
	}
}

func TestSplit_StopsAtTheTablesLimit(t *testing.T) {
	raw := withTable(t, func(s *GameState) {
		s.MaxSplits = 1
		s.Seats[0].Hands = []Hand{
			{ID: handID("p1", 0), Cards: []string{"8H", "8D"}, Bet: 10, Split: true},
			{ID: handID("p1", 1), Cards: []string{"8S", "8C"}, Bet: 10, Split: true},
		}
		s.Seats[0].Staked = 20
	})
	if _, err := apply(t, raw, "p1", VerbSplit); module.CodeOf(err) != ErrCannotSplit {
		t.Errorf("splitting past the limit was refused with %v, want %s", err, ErrCannotSplit)
	}
}

func TestSplit_RefusedWhenTheTableDoesNotSplitAtAll(t *testing.T) {
	raw := withTable(t, func(s *GameState) {
		s.MaxSplits = splitsNone
		s.Seats[0].Hands[0].Cards = []string{"8H", "8D"}
	})
	if _, err := apply(t, raw, "p1", VerbSplit); module.CodeOf(err) != ErrCannotSplit {
		t.Errorf("splitting at a no-split table was refused with %v, want %s", err, ErrCannotSplit)
	}
}

// --- surrender -----------------------------------------------------------

func TestSurrender_HandsBackHalfTheStakeAndTheOddChip(t *testing.T) {
	raw := withTable(t, func(s *GameState) {
		s.AllowSurrender = true
		s.Seats[0].Bet, s.Seats[0].Staked = 5, 5
		s.Seats[0].Stack = 495
		s.Seats[0].Hands[0] = Hand{ID: handID("p1", 0), Cards: []string{"TS", "6H"}, Bet: 5}
		s.Seats[1].Hands[0].Done = true
		s.Dealer = []string{"9S", "8D"}
	})
	next, err := apply(t, raw, "p1", VerbSurrender)
	if err != nil {
		t.Fatalf("surrender: %v", err)
	}
	s := stateOf(t, next)
	if got := s.Seats[0].Hands[0].Outcome; got != outcomeSurrender {
		t.Fatalf("the hand settled as %q", got)
	}
	// Half of five is two and a half; the player keeps the odd chip.
	if got := s.Rounds[0].Deltas["p1"]; got != -2 {
		t.Errorf("surrendering a stake of 5 cost %d, want 2", -got)
	}
}

func TestSurrender_RefusedWhenTheTableDoesNotOfferItOrTheHandHasMoved(t *testing.T) {
	off := withTable(t, func(s *GameState) { s.AllowSurrender = false })
	if _, err := apply(t, off, "p1", VerbSurrender); module.CodeOf(err) != ErrCannotSurrender {
		t.Errorf("surrender at a table without it was refused with %v, want %s", err, ErrCannotSurrender)
	}

	drawn := withTable(t, func(s *GameState) {
		s.AllowSurrender = true
		s.Seats[0].Hands[0].Cards = []string{"5H", "4D", "3C"}
	})
	if _, err := apply(t, drawn, "p1", VerbSurrender); module.CodeOf(err) != ErrCannotSurrender {
		t.Errorf("surrender after a hit was refused with %v, want %s", err, ErrCannotSurrender)
	}
}

// --- insurance -----------------------------------------------------------

func TestInsurance_PaysTwoToOneWhenTheDealerHasBlackjack(t *testing.T) {
	raw := withTable(t, func(s *GameState) {
		s.Phase = phaseInsurance
		s.Peeked = false
		s.Current, s.CurrentHand = -1, -1
		s.Dealer = []string{"AS", "KD"}
		s.Seats[0].Hands[0].Done = false
		s.Seats[1].InsuranceAnswered = true
	})
	next, _, err := New().Apply(raw, "p1", module.Action{Verb: VerbInsure})
	if err != nil {
		t.Fatalf("insure: %v", err)
	}
	s := stateOf(t, next)
	if len(s.Rounds) != 1 {
		t.Fatalf("a dealer blackjack should have settled the round: %+v", s.Phase)
	}
	// Stake 10 lost, premium 5 paid back at 2:1 for 15: level on the round.
	if got := s.Rounds[0].Deltas["p1"]; got != 0 {
		t.Errorf("an insured hand against a dealer blackjack moved %d chips, want 0", got)
	}
	if got := s.Rounds[0].Deltas["p2"]; got != -10 {
		t.Errorf("the uninsured seat lost %d, want 10", -got)
	}
}

func TestInsurance_IsNotOfferedWithoutADealerAce(t *testing.T) {
	raw := newMatch(t, 11, module.MatchConfig{})
	raw = betAll(t, raw, 10)
	s := stateOf(t, raw)
	if rankOf(s.upCard()) != "A" && s.Phase == phaseInsurance {
		t.Errorf("insurance was opened against a %s", s.upCard())
	}
}

func TestInsurance_ASeatThatCannotAffordItIsNotAsked(t *testing.T) {
	raw := withTable(t, func(s *GameState) {
		s.Phase = phaseInsurance
		s.Peeked = false
		s.Current, s.CurrentHand = -1, -1
		s.Dealer = []string{"AS", "9D"}
		// The premium is half a stake of ten; neither seat can cover it.
		s.Seats[0].Stack, s.Seats[1].Stack = 1, 1
	})
	s := stateOf(t, raw)
	openInsuranceOrPeek(s)
	if s.Phase == phaseInsurance {
		t.Error("a table where nobody can pay a premium should not stop to ask")
	}
}

// --- the match -----------------------------------------------------------

func TestMatch_EndsAfterTheRoundLimitWithTheBiggestStack(t *testing.T) {
	raw := withTable(t, func(s *GameState) {
		s.RoundLimit = 1
		s.RoundNumber = 1
		s.Seats[0].Hands[0] = Hand{ID: handID("p1", 0), Cards: []string{"TS", "TH"}, Bet: 10}
		s.Seats[1].Hands[0] = Hand{ID: handID("p2", 0), Cards: []string{"9H", "8C"}, Bet: 10, Done: true}
		s.Dealer = []string{"TD", "9C"} // nineteen: p1's twenty wins, p2's seventeen does not
	})
	next, err := apply(t, raw, "p1", VerbStand)
	if err != nil {
		t.Fatalf("stand: %v", err)
	}
	done, winners, err := New().Finished(next)
	if err != nil || !done {
		t.Fatalf("the round limit did not end the match: done=%v err=%v", done, err)
	}
	if len(winners) != 1 || winners[0] != "p1" {
		t.Errorf("the match was won by %v, want p1 alone", winners)
	}
}

func TestMatch_ASeatBelowTheMinimumIsOutAndOfferedNothing(t *testing.T) {
	raw := withTable(t, func(s *GameState) {
		s.RoundLimit = 5
		s.Seats[1].Stack = 0
		s.Seats[1].Hands[0] = Hand{ID: handID("p2", 0), Cards: []string{"TS", "2C"}, Bet: 10, Done: true}
		s.Dealer = []string{"9S", "8D"} // seventeen: p1's sixteen and p2's twelve both lose
	})
	next, err := apply(t, raw, "p1", VerbStand)
	if err != nil {
		t.Fatalf("stand: %v", err)
	}
	s := stateOf(t, next)
	if !seatOf(t, s, "p2").Out {
		t.Fatal("a seat that cannot meet the minimum should be out")
	}

	offers, err := New().LegalActions(next, "p2")
	if err != nil {
		t.Fatalf("LegalActions: %v", err)
	}
	for _, o := range offers {
		if o.Enabled && o.Verb != module.VerbContinue {
			t.Errorf("a busted-out seat is still offered %q", o.ID)
		}
	}
}

// betAll puts the same stake up for every seat, so a test can get to a deal
// without caring which seat bets first.
func betAll(t *testing.T, raw module.State, amount int) module.State {
	t.Helper()
	for _, p := range []string{"p1", "p2"} {
		next, err := bet(t, raw, p, amount)
		if err != nil {
			continue
		}
		raw = next
	}
	return raw
}

func remarshal(t *testing.T, s *GameState) module.State {
	t.Helper()
	raw, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return raw
}

// --- the shoe ------------------------------------------------------------

// TestShoe_NeverDealsACardThatIsAlreadyOnTheTable.
//
// The bug this is written from: the cut card was a quarter of the shoe, which
// is thirteen cards of a single deck — fewer than the sixteen a full table
// needs to deal. The round then ran the shoe dry mid-hand, a fresh deck was
// built with the live hands still on the table, and players sat looking at two
// of the same card. Forty of forty single-deck matches showed one by the
// fourth round.
func TestShoe_NeverDealsACardThatIsAlreadyOnTheTable(t *testing.T) {
	m := New()
	players := seats("p1", "p2", "p3", "p4", "p5", "p6", "p7")

	for seed := int64(1); seed <= 12; seed++ {
		cfg := module.MatchConfig{Variation: "single", Options: module.Options{
			OptRounds: 20, OptStartingStack: 2000,
		}}
		state, err := m.NewMatch(cfg, players, seed)
		if err != nil {
			t.Fatalf("NewMatch: %v", err)
		}
		for step := 0; step < 20000; step++ {
			if done, _, _ := m.Finished(state); done {
				break
			}
			awaited := module.AwaitedSeats(m, state, "p1", players)
			if len(awaited) == 0 {
				break
			}
			offers, err := m.LegalActions(state, awaited[0])
			if err != nil {
				t.Fatalf("LegalActions: %v", err)
			}
			// Hitting hardest is what drains a shoe fastest, which is the
			// position this is looking for.
			a, ok := module.ChooseAction(offers, []string{"bet", "decline_insurance", "hit", "stand"})
			if !ok {
				break
			}
			next, _, err := m.Apply(state, awaited[0], a)
			if err != nil {
				t.Fatalf("seed %d step %d: offered %+v but refused: %v", seed, step, a, err)
			}
			state = next

			s := stateOf(t, next)
			seen := map[string]int{}
			for i := range s.Seats {
				for _, h := range s.Seats[i].Hands {
					for _, c := range h.Cards {
						seen[c]++
					}
				}
			}
			for _, c := range s.Dealer {
				seen[c]++
			}
			for card, n := range seen {
				if n > s.Decks {
					t.Fatalf("seed %d round %d: %s is on the table %d times, and the shoe holds %d",
						seed, s.RoundNumber, card, n, s.Decks)
				}
			}
		}
	}
}

// TestShoe_IsReplacedBeforeARoundCanRunOutOfCards is the same rule stated
// where it is decided, rather than only observed through play: whatever the
// table size, the cut card leaves enough for every box and the dealer to be
// dealt to.
func TestShoe_IsReplacedBeforeARoundCanRunOutOfCards(t *testing.T) {
	for _, count := range []int{2, 4, 7} {
		s := tableOf(module.MatchConfig{Variation: "single"})
		for i := 0; i < count; i++ {
			s.Seats = append(s.Seats, Seat{PlayerID: "p" + itoa(i)})
		}
		if need := (count + 1) * 2; cutCard(s) < need {
			t.Errorf("%d seats: the shoe may fall to %d cards, and the deal alone needs %d",
				count, cutCard(s), need)
		}
	}
}

// --- the end of a match --------------------------------------------------

// TestMatch_ATableThatAllWentBrokeNamesNobody.
//
// The bug this is written from: the winner was whoever held the most chips,
// and with every seat on zero that was everybody. The match ended by naming
// three busted players joint winners, which the shell rendered as "You won."
// and the recorder wrote down as a win each.
func TestMatch_ATableThatAllWentBrokeNamesNobody(t *testing.T) {
	raw := withTable(t, func(s *GameState) {
		s.RoundLimit = 10
		s.Seats[0].Stack, s.Seats[1].Stack = 0, 0
		s.Seats[0].Hands[0] = Hand{ID: handID("p1", 0), Cards: []string{"9H", "7D"}, Bet: 10}
		s.Seats[1].Hands[0] = Hand{ID: handID("p2", 0), Cards: []string{"TS", "8C"}, Bet: 10, Done: true}
		s.Dealer = []string{"TD", "9C"} // nineteen: both hands lose
	})
	next, err := apply(t, raw, "p1", VerbStand)
	if err != nil {
		t.Fatalf("stand: %v", err)
	}

	done, winners, err := New().Finished(next)
	if err != nil || !done {
		t.Fatalf("a table with nobody able to bet should be over: done=%v err=%v", done, err)
	}
	if len(winners) != 0 {
		t.Errorf("every seat is on zero chips, and the match named %v as winners", winners)
	}
	// And the scoreboard still reads, so a client has something to show.
	if got := module.StandingsFor(New(), next); len(got) != 2 {
		t.Errorf("the scoreboard lost its rows: %+v", got)
	}
}

// --- refusals say the right thing ---------------------------------------

// TestRefusals_NameTheHandBeforeTheStack.
//
// The bug this is written from: both handlers checked the stack first, so a
// short seat that had already drawn was told "you don't have that many chips"
// about a hand that could not have been doubled at any stack size — and, since
// that code maps to no written rule, the "why" behind the control opened
// nothing.
func TestRefusals_NameTheHandBeforeTheStack(t *testing.T) {
	for _, tc := range []struct {
		name string
		verb string
		want string
		mut  func(*GameState)
	}{
		{
			name: "a hand that has already drawn cannot be doubled at any price",
			verb: VerbDouble, want: ErrCannotDouble,
			mut: func(s *GameState) {
				s.Seats[0].Stack = 2
				s.Seats[0].Hands[0].Cards = []string{"5H", "4D", "3C"}
			},
		},
		{
			name: "a hand that is not a pair cannot be split at any price",
			verb: VerbSplit, want: ErrCannotSplit,
			mut: func(s *GameState) {
				s.Seats[0].Stack = 2
				s.Seats[0].Hands[0].Cards = []string{"9H", "7D"}
			},
		},
		{
			name: "a legal double a short seat cannot cover says so",
			verb: VerbDouble, want: ErrNotEnoughChips,
			mut: func(s *GameState) {
				s.Seats[0].Stack = 2
				s.Seats[0].Hands[0].Cards = []string{"6H", "5D"}
			},
		},
		{
			name: "a legal split a short seat cannot cover says so",
			verb: VerbSplit, want: ErrNotEnoughChips,
			mut: func(s *GameState) {
				s.Seats[0].Stack = 2
				s.Seats[0].Hands[0].Cards = []string{"8H", "8D"}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := withTable(t, tc.mut)
			_, err := apply(t, raw, "p1", tc.verb)
			if got := module.CodeOf(err); got != tc.want {
				t.Errorf("refused with %s, want %s", got, tc.want)
			}
			// And the offer says the same thing, since a player reads the
			// reason there rather than by pressing a dead control.
			offers, err := New().LegalActions(raw, "p1")
			if err != nil {
				t.Fatalf("LegalActions: %v", err)
			}
			id := OfferDouble
			if tc.verb == VerbSplit {
				id = OfferSplit
			}
			o := module.FindOffer(offers, id)
			if o == nil || o.WhyNot != tc.want {
				t.Errorf("the offer says %+v, want whyNot=%s", o, tc.want)
			}
		})
	}
}
