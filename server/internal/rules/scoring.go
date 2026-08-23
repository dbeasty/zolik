package rules

import "strings"

// PenaltyPoints returns the penalty value for a single card.
// If aceAsNatural is true, an Ace counts as 1 (used as natural in a run).
// Otherwise, an Ace counts as 25 (used as wild / in-hand penalty).
func PenaltyPoints(card string, aceAsNatural bool) int {
	if strings.HasPrefix(card, "JOKER") {
		return 50
	}
	if len(card) < 2 {
		return 0
	}
	r := card[0]
	switch r {
	case 'A':
		if aceAsNatural {
			return 1
		}
		return 25
	case 'K', 'Q', 'J', 'T':
		return 10
	default:
		if r >= '2' && r <= '9' {
			return int(r - '0')
		}
	}
	return 0
}

// AceMeldValueInSet is what an ace melded in a set contributes toward the
// initial-meld floor.
//
// An ace in a *run* is a positional card — it occupies rank 1 or rank 14 of
// its suit and is worth 1, the low-ace convention (see NaturalCardValue). An
// ace in a *set* is nothing of the sort: it is a real ace, the highest card
// in the game, and scoring it 1 made three aces worth 3 points — less than
// three 2s, and short of every meld floor the lobby offers. validateSet's own
// comment already says an ace there "is a real rank, not a stand-in for
// another rank"; this is that statement applied to its value.
const AceMeldValueInSet = 15

// NaturalSetCardValue is NaturalCardValue for a card melded in a set, where
// an ace scores as the real card it is rather than as a run endpoint.
func NaturalSetCardValue(card string) int {
	if IsJoker(card) {
		return 0
	}
	if IsAce(card) {
		return AceMeldValueInSet
	}
	return PenaltyPoints(card, true)
}

func NaturalCardValue(card string, aceAsNatural bool) int {
	// Natural-value is used only for initial meld minimum checks.
	// Wild cards contribute 0. For an ace filling a run endpoint it
	// contributes 1; for an ace melded in a set see NaturalSetCardValue.
	if IsJoker(card) {
		return 0
	}
	if IsAce(card) {
		if aceAsNatural {
			return 1
		}
		return 0
	}
	return PenaltyPoints(card, true)
}
