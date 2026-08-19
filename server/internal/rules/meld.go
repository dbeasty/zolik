package rules

import (
	"sort"
	"strings"
)

const RankOrder = "23456789TJQKA"

func CardRank(card string) int {
	if IsJoker(card) {
		return -1
	}
	if len(card) < 2 {
		return -1
	}
	for i := 0; i < len(RankOrder); i++ {
		if RankOrder[i] == card[0] {
			return i
		}
	}
	return -1
}

func CardSuit(card string) string {
	if IsJoker(card) {
		return ""
	}
	if len(card) < 2 {
		return ""
	}
	return card[1:2]
}

func IsJoker(card string) bool {
	return strings.HasPrefix(card, "JOKER")
}

func IsAce(card string) bool {
	return len(card) >= 1 && card[0] == 'A' && !IsJoker(card)
}

// IsWild returns true for cards that are always wild (jokers) or potentially wild (aces).
// For meld validation, aces may be treated as natural in a run if they occupy the end position.
func IsWild(card string) bool {
	return IsJoker(card) || IsAce(card)
}

type MeldValidation struct {
	Type         MeldType
	AceAsNatural map[string]int // count of aces treated as natural (card string -> count)
	NaturalValue int            // sum of natural card values (wild=0, ace natural=1)
	NaturalCount int
	WildCount    int
	ResolvedRun  []int  // ranks, using 1 for A-low, 14 for A-high, 2..13 otherwise
	ResolvedSuit string // suit for run
}

func ValidateMeld(cards []string) (MeldValidation, error) {
	if v, err := validateSet(cards); err == nil {
		return v, nil
	}
	if v, err := validateRun(cards); err == nil {
		return v, nil
	}
	return MeldValidation{}, RulesError{Code: ErrInvalidMeld}
}

func ValidateMeldValue(cards []string, aceAsNatural map[string]int) int {
	sum := 0
	aceNaturalUsed := func(card string) bool {
		if aceAsNatural == nil {
			return false
		}
		return aceAsNatural[card] > 0
	}
	for _, c := range cards {
		if IsJoker(c) {
			continue
		}
		if IsAce(c) {
			if aceNaturalUsed(c) {
				sum += 1
			}
			continue
		}
		sum += NaturalCardValue(c, true)
	}
	return sum
}

func validateSet(cards []string) (MeldValidation, error) {
	if len(cards) < 3 {
		return MeldValidation{}, RulesError{Code: ErrInvalidMeld}
	}

	var naturals []string
	wildCount := 0
	for _, c := range cards {
		if IsJoker(c) || IsAce(c) {
			wildCount++
			continue
		}
		naturals = append(naturals, c)
	}
	if len(naturals) == 0 {
		return MeldValidation{}, RulesError{Code: ErrInvalidMeld}
	}
	if wildCount > len(naturals) {
		return MeldValidation{}, RulesError{Code: ErrTooManyWilds}
	}

	targetRank := naturals[0][0]
	seenSuits := map[string]bool{naturals[0][1:]: true}
	for _, c := range naturals[1:] {
		if len(c) < 1 || c[0] != targetRank {
			return MeldValidation{}, RulesError{Code: ErrInvalidMeld}
		}
		suit := c[1:]
		if seenSuits[suit] {
			// A set may only use one card of each suit (per rank), even when
			// two physical decks are in play — a repeated suit means the
			// second copy belongs to a different set, not this one.
			return MeldValidation{}, RulesError{Code: ErrInvalidMeld}
		}
		seenSuits[suit] = true
	}

	naturalValue := 0
	for _, c := range naturals {
		naturalValue += NaturalCardValue(c, true)
	}

	return MeldValidation{
		Type:         MeldSet,
		AceAsNatural: map[string]int{},
		NaturalValue: naturalValue,
		NaturalCount: len(naturals),
		WildCount:    wildCount,
	}, nil
}

func validateRun(cards []string) (MeldValidation, error) {
	if len(cards) < 4 {
		return MeldValidation{}, RulesError{Code: ErrInvalidMeld}
	}

	// Separate jokers, aces (flex), and fixed naturals.
	var jokers []string
	var aces []string
	var fixed []string // non-ace, non-joker

	for _, c := range cards {
		switch {
		case IsJoker(c):
			jokers = append(jokers, c)
		case IsAce(c):
			aces = append(aces, c)
		default:
			fixed = append(fixed, c)
		}
	}

	if len(fixed) == 0 && len(aces) == 0 {
		return MeldValidation{}, RulesError{Code: ErrInvalidMeld}
	}

	// Determine suit from the first fixed natural; if no fixed, from first ace (only if treated natural).
	runSuit := ""
	if len(fixed) > 0 {
		runSuit = CardSuit(fixed[0])
		for _, c := range fixed[1:] {
			if CardSuit(c) != runSuit {
				return MeldValidation{}, RulesError{Code: ErrInvalidMeld}
			}
		}
	}

	// Convert fixed ranks to 2..13 (A handled separately).
	fixedRanks := make([]int, 0, len(fixed))
	for _, c := range fixed {
		r := cardToRunRank(c)
		if r < 2 || r > 13 {
			return MeldValidation{}, RulesError{Code: ErrInvalidMeld}
		}
		fixedRanks = append(fixedRanks, r)
	}
	sort.Ints(fixedRanks)
	for i := 1; i < len(fixedRanks); i++ {
		if fixedRanks[i] == fixedRanks[i-1] {
			// Duplicate natural rank (same suit) cannot exist in a strict run.
			return MeldValidation{}, RulesError{Code: ErrInvalidMeld}
		}
	}
	if hasAceBridge(fixedRanks) {
		return MeldValidation{}, RulesError{Code: ErrAceBridge}
	}

	L := len(cards)

	// Candidate start ranks in 1..(14-L+1). 14 represents Ace-high endpoint.
	var starts []int
	for s := 1; s <= 14-L+1; s++ {
		starts = append(starts, s)
	}

	aceAsNatural := map[string]int{}

tryStart:
	for _, start := range starts {
		end := start + L - 1
		if end > 14 {
			continue
		}

		// No wrap-around allowed: if sequence crosses from 13 to 14 and continues, would imply K-A-? which is invalid.
		// Here, 14 can only be the end position.
		if end == 14 && start <= 12 {
			// Allowed (e.g. 11-12-13-14 => J-Q-K-A)
		}
		positions := make([]int, L)
		for i := 0; i < L; i++ {
			positions[i] = start + i
		}

		// Ensure fixed naturals all fit.
		for _, r := range fixedRanks {
			if !containsInt(positions, r) {
				continue tryStart
			}
		}

		// Count how many natural slots are filled by fixed naturals.
		naturalSlots := map[int]bool{}
		for _, r := range fixedRanks {
			naturalSlots[r] = true
		}

		// Decide if we can treat some aces as natural endpoints.
		// Ace natural is only permitted at rank 1 or 14 positions, and only if suit matches runSuit (when suit is known).
		aceNaturalNeeded := 0
		aceLow := containsInt(positions, 1)
		aceHigh := containsInt(positions, 14)

		usableLowAces := 0
		usableHighAces := 0
		for _, a := range aces {
			if runSuit != "" && CardSuit(a) != runSuit {
				continue
			}
			if aceLow {
				usableLowAces++
			}
			if aceHigh {
				usableHighAces++
			}
		}

		if aceLow {
			aceNaturalNeeded++
		}
		if aceHigh {
			aceNaturalNeeded++
		}

		// If suit unknown (no fixed naturals), we must select a suit by requiring at least one natural ace to establish suit.
		// If the run has no fixed naturals, we require the aces used as natural endpoints to share a suit.
		if runSuit == "" {
			if !(aceLow || aceHigh) {
				continue tryStart
			}
			// Choose suit from the first ace; ensure we can supply needed endpoints with that suit.
			suit := CardSuit(aces[0])
			runSuit = suit
			usableLowAces = 0
			usableHighAces = 0
			for _, a := range aces {
				if CardSuit(a) != suit {
					continue
				}
				if aceLow {
					usableLowAces++
				}
				if aceHigh {
					usableHighAces++
				}
			}
		}

		needLow := 0
		needHigh := 0
		if aceLow {
			needLow = 1
		}
		if aceHigh {
			needHigh = 1
		}

		if needLow > 0 && usableLowAces == 0 {
			continue tryStart
		}
		if needHigh > 0 && usableHighAces == 0 {
			continue tryStart
		}

		// Assign specific ace cards as natural if needed.
		aceAsNatural = map[string]int{}
		if needLow > 0 {
			for _, a := range aces {
				if CardSuit(a) == runSuit {
					aceAsNatural[a]++
					break
				}
			}
			naturalSlots[1] = true
		}
		if needHigh > 0 {
			for _, a := range aces {
				if CardSuit(a) == runSuit {
					// Prefer a different physical ace if possible (but representation duplicates, so this is best-effort).
					aceAsNatural[a]++
					break
				}
			}
			naturalSlots[14] = true
		}

		naturalCount := len(fixedRanks) + len(aceAsNatural)
		// Treat remaining aces as wild.
		wildCount := len(jokers) + (len(aces) - len(aceAsNatural))

		if wildCount > naturalCount {
			continue tryStart
		}

		// Adjacent wild rule: two consecutive wild-filled slots are invalid.
		adjacentWild := false
		for i := 0; i < L-1; i++ {
			if !naturalSlots[positions[i]] && !naturalSlots[positions[i+1]] {
				adjacentWild = true
				break
			}
		}
		if adjacentWild {
			continue tryStart
		}

		naturalValue := ValidateMeldValue(cards, aceAsNatural)

		return MeldValidation{
			Type:         MeldRun,
			AceAsNatural: aceAsNatural,
			NaturalValue: naturalValue,
			NaturalCount: naturalCount,
			WildCount:    wildCount,
			ResolvedRun:  positions,
			ResolvedSuit: runSuit,
		}, nil
	}

	return MeldValidation{}, RulesError{Code: ErrInvalidMeld}
}

func containsInt(hay []int, needle int) bool {
	for _, v := range hay {
		if v == needle {
			return true
		}
	}
	return false
}

// hasAceBridge detects naturals that cannot belong to one consecutive run (e.g. K and 2 same suit).
func hasAceBridge(fixedRanks []int) bool {
	hasLow := false
	hasHigh := false
	for _, r := range fixedRanks {
		if r <= 3 {
			hasLow = true
		}
		if r >= 11 {
			hasHigh = true
		}
	}
	return hasLow && hasHigh
}

func cardToRunRank(card string) int {
	// 2..13 for 2..K; A is handled separately.
	if IsJoker(card) || IsAce(card) || len(card) < 1 {
		return -1
	}
	switch card[0] {
	case 'T':
		return 10
	case 'J':
		return 11
	case 'Q':
		return 12
	case 'K':
		return 13
	default:
		if card[0] >= '2' && card[0] <= '9' {
			return int(card[0] - '0')
		}
	}
	return -1
}
