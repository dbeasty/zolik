package rules

import (
	"fmt"
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

// ValidateMeld validates cards against the given RulesConfig's minimum set
// and run sizes. Callers without a live GameState (e.g. AI search) may pass
// any RulesConfig — ProfileContinental for the historical 3/4 minimums.
func ValidateMeld(cards []string, cfg RulesConfig) (MeldValidation, error) {
	minSet := cfg.MinSetSize
	if minSet == 0 {
		minSet = 3
	}
	minRun := cfg.MinRunSize
	if minRun == 0 {
		minRun = 4
	}
	setV, setErr := validateSet(cards, minSet)
	if setErr == nil {
		return setV, nil
	}
	runV, runErr := validateRun(cards, minRun)
	if runErr == nil {
		return runV, nil
	}
	// Neither a set nor a run: surface whichever failure is more specific
	// (a distinct error code, or at least a message) over the generic
	// catch-all below, so the player sees the real reason instead of a
	// bare "invalid meld".
	if re, ok := setErr.(RulesError); ok && (re.Code != ErrInvalidMeld || re.Message != "") {
		return MeldValidation{}, setErr
	}
	if re, ok := runErr.(RulesError); ok && (re.Code != ErrInvalidMeld || re.Message != "") {
		return MeldValidation{}, runErr
	}
	return MeldValidation{}, RulesError{
		Code:    ErrInvalidMeld,
		Message: "these cards don't form a valid set (same rank, different suits) or run (consecutive ranks, same suit)",
	}
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

func validateSet(cards []string, minSetSize int) (MeldValidation, error) {
	if len(cards) < minSetSize {
		return MeldValidation{}, RulesError{
			Code:    ErrInvalidMeld,
			Message: fmt.Sprintf("a set needs at least %d cards", minSetSize),
		}
	}

	var naturals []string
	wildCount := 0
	for _, c := range cards {
		// Only jokers are wild in a set. An ace is a real rank ("A"), not a
		// stand-in for another rank — otherwise two natural queens plus an
		// ace would pass as "three queens" without a genuine third queen.
		if IsJoker(c) {
			wildCount++
			continue
		}
		naturals = append(naturals, c)
	}
	if len(naturals) == 0 {
		return MeldValidation{}, RulesError{
			Code:    ErrInvalidMeld,
			Message: "a set needs at least one natural (non-joker) card to establish its rank",
		}
	}
	if wildCount > len(naturals) {
		return MeldValidation{}, RulesError{Code: ErrTooManyWilds, Message: "a set can't have more jokers than natural cards"}
	}

	targetRank := naturals[0][0]
	seenSuits := map[string]bool{naturals[0][1:]: true}
	for _, c := range naturals[1:] {
		if len(c) < 1 || c[0] != targetRank {
			return MeldValidation{}, RulesError{
				Code:    ErrInvalidMeld,
				Message: "all cards in a set must share the same rank",
			}
		}
		suit := c[1:]
		if seenSuits[suit] {
			// A set may only use one card of each suit (per rank), even when
			// two physical decks are in play — a repeated suit means the
			// second copy belongs to a different set, not this one.
			return MeldValidation{}, RulesError{
				Code:    ErrInvalidMeld,
				Message: "a set can't have two cards of the same suit",
			}
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

func validateRun(cards []string, minRunSize int) (MeldValidation, error) {
	if len(cards) < minRunSize {
		return MeldValidation{}, RulesError{
			Code:    ErrInvalidMeld,
			Message: fmt.Sprintf("a run needs at least %d cards", minRunSize),
		}
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
		return MeldValidation{}, RulesError{
			Code:    ErrInvalidMeld,
			Message: "a run can't be made entirely of jokers",
		}
	}

	// Determine suit from the first fixed natural; if no fixed, from first ace (only if treated natural).
	runSuit := ""
	if len(fixed) > 0 {
		runSuit = CardSuit(fixed[0])
		for _, c := range fixed[1:] {
			if CardSuit(c) != runSuit {
				return MeldValidation{}, RulesError{
					Code:    ErrInvalidMeld,
					Message: "all cards in a run must share the same suit",
				}
			}
		}
	}

	// Convert fixed ranks to 2..13 (A handled separately).
	fixedRanks := make([]int, 0, len(fixed))
	for _, c := range fixed {
		r := cardToRunRank(c)
		if r < 2 || r > 13 {
			return MeldValidation{}, RulesError{
				Code:    ErrInvalidMeld,
				Message: fmt.Sprintf("%q is not a recognized card rank for a run", c),
			}
		}
		fixedRanks = append(fixedRanks, r)
	}
	sort.Ints(fixedRanks)
	for i := 1; i < len(fixedRanks); i++ {
		if fixedRanks[i] == fixedRanks[i-1] {
			// Duplicate natural rank (same suit) cannot exist in a strict run.
			return MeldValidation{}, RulesError{
				Code:    ErrInvalidMeld,
				Message: "a run can't repeat the same rank twice",
			}
		}
	}
	if hasAceBridge(fixedRanks) {
		return MeldValidation{}, RulesError{
			Code:    ErrAceBridge,
			Message: "a run can't bridge from a low card (A-3) to a high card (J-K) around the corner",
		}
	}

	L := len(cards)

	// Candidate start ranks in 1..(14-L+1). 14 represents Ace-high endpoint.
	var starts []int
	for s := 1; s <= 14-L+1; s++ {
		starts = append(starts, s)
	}

	aceAsNatural := map[string]int{}
	var best *MeldValidation

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

		// Unlike jokers, an ace is a specific physical card with a real,
		// fixed suit — it can only stand in for rank 1 or rank 14 of its
		// own suit (handled above as aceAsNatural). An ace that isn't used
		// as a natural endpoint can't fill some other gap in the run
		// (that would let e.g. a wrong-suited ace pass as a wild filler),
		// so this window doesn't fit and we try the next one.
		if len(aces) > len(aceAsNatural) {
			continue tryStart
		}

		naturalCount := len(fixedRanks) + len(aceAsNatural)
		wildCount := len(jokers)

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

		candidate := MeldValidation{
			Type:         MeldRun,
			AceAsNatural: aceAsNatural,
			NaturalValue: naturalValue,
			NaturalCount: naturalCount,
			WildCount:    wildCount,
			ResolvedRun:  append([]int(nil), positions...),
			ResolvedSuit: runSuit,
		}
		// Several windows can fit the same fixed cards (e.g. J-Q-K plus one
		// flex ace could sit at 10-J-Q-K with the ace wild, or J-Q-K-A with
		// the ace natural) — prefer the one that spends the fewest wilds,
		// since a flex ace should always resolve to its natural endpoint
		// over standing in for an unrelated rank when both are possible.
		if best == nil || candidate.WildCount < best.WildCount {
			best = &candidate
		}
	}

	if best != nil {
		return *best, nil
	}

	return MeldValidation{}, RulesError{
		Code:    ErrInvalidMeld,
		Message: "these cards don't fit into one consecutive run of the same suit (check for too many wild cards or two wilds in a row)",
	}
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

// OrderMeldForDisplay returns cards rearranged into a stable, readable
// order for storage/display — callers should persist this order rather
// than whatever order the cards were selected/played in, so a meld a
// player laid as e.g. 6-8-7 always shows as the sorted run 6-7-8 on the
// table. Sets sort naturals by suit (jokers last); runs are rebuilt in
// ascending rank order using mv's already-validated slot assignment.
func OrderMeldForDisplay(cards []string, mv MeldValidation) []string {
	switch mv.Type {
	case MeldSet:
		return orderSetForDisplay(cards)
	case MeldRun:
		return orderRunForDisplay(cards, mv)
	default:
		return cards
	}
}

func orderSetForDisplay(cards []string) []string {
	out := append([]string(nil), cards...)
	sort.SliceStable(out, func(i, j int) bool {
		ji, jj := IsJoker(out[i]), IsJoker(out[j])
		if ji != jj {
			return jj // naturals before jokers
		}
		if ji {
			return false
		}
		return CardSuit(out[i]) < CardSuit(out[j])
	})
	return out
}

// orderRunForDisplay walks mv.ResolvedRun (already ascending) and, for each
// rank slot, picks the specific card that fills it: a fixed natural at that
// rank, an ace assigned as a natural endpoint (rank 1 or 14), or otherwise a
// wildcard (joker/flex ace) filling a gap.
func orderRunForDisplay(cards []string, mv MeldValidation) []string {
	if len(mv.ResolvedRun) == 0 {
		return cards
	}
	remaining := append([]string(nil), cards...)
	take := func(pred func(string) bool) string {
		for i, c := range remaining {
			if pred(c) {
				remaining = append(remaining[:i], remaining[i+1:]...)
				return c
			}
		}
		return ""
	}

	out := make([]string, 0, len(cards))
	for _, pos := range mv.ResolvedRun {
		var picked string
		if pos == 1 || pos == 14 {
			picked = take(func(c string) bool {
				return IsAce(c) && mv.AceAsNatural[c] > 0
			})
		}
		if picked == "" {
			picked = take(func(c string) bool {
				return !IsJoker(c) && !IsAce(c) && cardToRunRank(c) == pos
			})
		}
		if picked == "" {
			picked = take(func(c string) bool { return IsJoker(c) })
		}
		if picked == "" {
			picked = take(func(c string) bool { return IsAce(c) })
		}
		if picked != "" {
			out = append(out, picked)
		}
	}
	// Any leftovers (shouldn't normally happen once validated) go at the end
	// rather than being silently dropped.
	out = append(out, remaining...)
	return out
}
