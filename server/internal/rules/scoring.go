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

// An ace's contribution toward the initial-meld floor depends on how it is
// being used, and only one of the three ways is worth a single point.
//
//   - AceMeldValue: the ace played as the real, highest card in the game —
//     in a set (A-A-A), or at the top of a run (Q-K-A).
//   - AceRunLowValue: the ace played as rank 1, the bottom of a run
//     (A-2-3). The low-ace convention, and the only place 1 is right.
//
// Scoring every natural ace 1 made a set of three aces worth 3 points — less
// than three 2s — and Q-K-A worth 21 instead of 35, which put the meld floors
// the lobby offers (35/50/70) out of reach for any hand built on aces.
// validateSet's own comment already says an ace there "is a real rank, not a
// stand-in for another rank"; these are that statement applied to its value.
const (
	AceMeldValue   = 15
	AceRunLowValue = 1
)

// A wild is worth the card it stands in for.
//
// This is why neither of the helpers below can price a joker on its own: the
// card string "JOKER1" says nothing about which rank it is playing. Only the
// meld knows — a set's rank is shared by every card in it, and a run's slot is
// named by the resolved rank window — so both callers price a joker from the
// meld's own shape (validateSet's cardValue, runRankValue for a run) and never
// ask these helpers about one.
//
// The alternative, scoring every wild 0, made the floors the lobby offers
// unreachable for the hands most able to fill them: J♠-Q♠ plus a joker behind
// the king is the ace-high run Q-K-A, plainly worth 35, and counted 20.

// NaturalSetCardValue is NaturalCardValue for a card melded in a set, where
// an ace scores as the real card it is rather than as a run's low endpoint.
func NaturalSetCardValue(card string) int {
	if IsJoker(card) {
		return 0
	}
	if IsAce(card) {
		return AceMeldValue
	}
	return PenaltyPoints(card, true)
}

func NaturalCardValue(card string, aceAsNatural bool) int {
	// Used only for initial meld minimum checks. The ace cases have their own
	// helpers — NaturalSetCardValue for a set, runRankValue for a run's slots
	// — so the bare 1 here is only ever the low endpoint.
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
