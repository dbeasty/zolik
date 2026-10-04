package holdem

import (
	"math"
	"math/rand"
)

// The river, priced by the size of the bet.
//
// On the last street there are no cards to come, so a bet is a polarised
// statement: the hands that want a call and the hands that cannot win without
// a fold. The equilibrium of that spot is elementary (von Neumann &
// Morgenstern's model; Chen & Ankenman, The Mathematics of Poker, 2006): with
// a pot P and a bet B,
//
//	the bettor's bluffs are B/(P+2B) of its betting range — the share at which
//	a bluff-catcher, risking B to win P+B, gains nothing by calling or folding;
//	the caller continues with P/(P+B) of its range — the share at which a
//	bluff, risking B to win P, gains nothing either.
//
// A flat bluff chance gets both wrong at every size but one: it bluffs a
// two-pot overbet as often as a third-pot bet, which tells anyone paying
// attention what the big bet holds. Hard (profile.sizedBluffs, defendMDF)
// prices both off the bet instead.

const (
	// riverBetSize is the polarised river bet, value and bluff alike, as a
	// fraction of the pot: one size, so the size says nothing about which.
	riverBetSize = 0.70
	// riverValue and riverAir are where the polar ends start, in equity
	// against any hand — on the river that is the hand's rank among all the
	// hands that could be held, a percentile.
	riverValue = 0.72
	riverAir   = 0.30
	// riverValueShare and riverAirShare are how much of a range lands at each
	// end. Measured over random river deals by the same 400-trial estimate the
	// bot uses (TestRiverShares): the percentile spreads hands nearly evenly,
	// so they sit close to 0.28 and 0.30.
	riverValueShare = 0.281
	riverAirShare   = 0.310
)

// riverBluffChance is how often to bet a hand from the air end with a bet of
// B into P, so that bluffs are B/(P+2B) of everything bet at that size.
//
// Value is riverValueShare of the range; bluffs must be B/(P+B) times as many
// for the share to come out at B/(P+2B), and they are drawn from
// riverAirShare of it.
func riverBluffChance(bet, pot int) float64 {
	if bet <= 0 || pot <= 0 {
		return 0
	}
	b, p := float64(bet), float64(pot)
	return math.Min(1, riverValueShare*b/(p+b)/riverAirShare)
}

// polarRiver is the checked-to heads-up river: value bets and their bluffs at
// one size. A hand between the ends is not decided here (false), and plays as
// it always has: a thin bet or a check.
func polarRiver(s *GameState, seat *Seat, mn menu, eq float64, bluff bool, rnd *rand.Rand) (choice, bool) {
	to := raiseTarget(s, mn, betOf(s, seat, riverBetSize))
	switch {
	case eq >= riverValue:
		return choice{verb: VerbRaise, to: to}, true
	case eq < riverAir:
		if bluff && rnd.Float64() < riverBluffChance(to-seat.Bet, potNow(s)) {
			return choice{verb: VerbRaise, to: to}, true
		}
		return choice{verb: VerbCheck}, true
	}
	return choice{}, false
}

// defends reports whether a hand with this equity against any hand is in the
// top P/(P+B) of hands, facing `owed` more with `pot` on the table (the bet
// included, as everywhere in this file).
func defends(anyHand float64, owed, pot int) bool {
	if owed <= 0 || pot <= owed {
		return false
	}
	before := float64(pot - owed)
	mdf := before / (before + float64(owed))
	return anyHand >= 1-mdf
}

// opponentReads is the one other player still in the hand's public counters,
// or nil.
func opponentReads(s *GameState, seat *Seat) *SeatReads {
	for i := range s.Seats {
		if o := &s.Seats[i]; o != seat && o.inHand() {
			return o.Reads
		}
	}
	return nil
}

// foldReadsAfter is how many bets an opponent must have answered before its
// fold rate is evidence (profile.nonFolder).
var foldReadsAfter = 3

// bluffsFold reports whether the opponent folds often enough to bluff: it has
// folded to a bet after the flop at least once, and, once it has answered
// foldReadsAfter or more of them, folded at least `share` of those. Until the
// first fold there is no evidence that a bluff can work at all, and the
// equilibrium share is only worth bluffing at a player who folds.
func bluffsFold(s *GameState, seat *Seat, share float64) bool {
	if share <= 0 {
		return true
	}
	r := opponentReads(s, seat)
	if r == nil || r.FoldedToBet == 0 {
		return false
	}
	return folds(r.FoldedToBet, r.FacedBet, share)
}

// folds reports whether `folded` of `faced` is enough folding: true until
// there are foldReadsAfter answers to judge by.
func folds(folded, faced int, share float64) bool {
	return faced < foldReadsAfter || float64(folded) >= share*float64(faced)
}

// shownBluffing reports whether the opponent has turned over a big bet made
// with a weak hand.
func shownBluffing(s *GameState, seat *Seat) bool {
	r := opponentReads(s, seat)
	return r != nil && r.BluffsShown > 0
}
