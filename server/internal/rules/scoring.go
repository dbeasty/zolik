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

func NaturalCardValue(card string, aceAsNatural bool) int {
	// Natural-value is used only for initial meld minimum checks.
	// Wild cards contribute 0. For Ace, if used as natural it contributes 1.
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

