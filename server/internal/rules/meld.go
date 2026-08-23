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
	NaturalValue int            // what the meld is worth toward the initial-meld floor: every card priced by the rank it plays, a wild included (ace: AceRunLowValue at a run's bottom, AceMeldValue in a set or at a run's top)
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
	//
	// Which of the two failures to prefer is decided by the shape the cards
	// were reaching for, not by a fixed order. validateSet almost always
	// comes back with a message, so preferring it unconditionally meant a
	// same-suit sequence that failed as a run was reported as "all cards in
	// a set must share the same rank" — a complaint about a meld the player
	// was never building, which hides the actual reason their run was
	// refused.
	first, second := setErr, runErr
	if looksLikeRunAttempt(cards) {
		first, second = runErr, setErr
	}
	if re, ok := first.(RulesError); ok && (re.Code != ErrInvalidMeld || re.Message != "") {
		return MeldValidation{}, first
	}
	if re, ok := second.(RulesError); ok && (re.Code != ErrInvalidMeld || re.Message != "") {
		return MeldValidation{}, second
	}
	return MeldValidation{}, RulesError{
		Code:    ErrInvalidMeld,
		Message: "these cards don't form a valid set (same rank, different suits) or run (consecutive ranks, same suit)",
	}
}

// looksLikeRunAttempt reports whether these cards read as a run in progress
// rather than a set in progress: every natural card shares one suit and they
// are not all the same rank. It only picks which validator's complaint to
// show the player — never whether the meld is legal.
func looksLikeRunAttempt(cards []string) bool {
	suit := ""
	rank := ""
	naturals := 0
	mixedRanks := false
	for _, c := range cards {
		if IsJoker(c) {
			continue
		}
		s := CardSuit(c)
		if naturals == 0 {
			suit, rank = s, c[:1]
		} else {
			if s != suit {
				return false
			}
			if c[:1] != rank {
				mixedRanks = true
			}
		}
		naturals++
	}
	return naturals >= 2 && mixedRanks
}

// runRankValue is what one slot of a run is worth, named by the resolved rank
// that sits in it (1 for A-low, 14 for A-high, 2..13 otherwise).
//
// The rank is the whole answer, because a wild is worth exactly the card it
// stands in for. So the slot is priced the same whether the real card or a
// joker occupies it, and the two ace slots keep their different values — the
// low ace at rank 1 is worth AceRunLowValue, the high ace above the king is
// worth AceMeldValue — whichever card is filling them.
func runRankValue(rank int) int {
	switch {
	case rank == 1:
		return AceRunLowValue
	case rank == 14:
		return AceMeldValue
	case rank >= 10 && rank <= 13:
		return 10
	case rank >= 2 && rank <= 9:
		return rank
	}
	return 0
}

// runValue totals what a run contributes toward the initial-meld floor, from
// its resolved rank window alone.
//
// The cards themselves need not be consulted: the window already says which
// rank sits in every slot, and a joker filling one is worth that rank. Which
// end an ace occupies — the one thing the card string "AC" cannot tell you,
// since it looks identical at rank 1 and rank 14 — is likewise a property of
// the window, so reading the value off it keeps the two facts from drifting.
func runValue(positions []int) int {
	sum := 0
	for _, p := range positions {
		sum += runRankValue(p)
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

	// Every card in a set is the same rank, so every card is worth the same —
	// including the jokers, which stand in for that rank and are priced as it.
	// naturals[0] names the rank for all of them.
	cardValue := NaturalSetCardValue(naturals[0])
	naturalValue := cardValue * (len(naturals) + wildCount)

	return MeldValidation{
		Type:         MeldSet,
		AceAsNatural: map[string]int{},
		NaturalValue: naturalValue,
		NaturalCount: len(naturals),
		WildCount:    wildCount,
	}, nil
}

// preferHighStart, when set true (pass a single true), breaks ties between
// equally-valid windows (same WildCount) in favor of the highest start rank
// instead of the lowest. Lay-offs use this to resolve a run the same
// direction the player actually dropped their card, rather than always
// defaulting to the window that extends the front.
//
// Passing it at all — true or false — says a drop position was expressed, and
// that outranks the value preference below. The player pointed at an end of
// the run; resolving the other one because it happens to score higher would
// be the engine overruling them, which is exactly what Action.Position exists
// to prevent.
func validateRun(cards []string, minRunSize int, preferHighStart ...bool) (MeldValidation, error) {
	positionGiven := len(preferHighStart) > 0
	preferHigh := positionGiven && preferHighStart[0]
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
	L := len(cards)

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
	if hasAceBridge(fixedRanks, L) {
		return MeldValidation{}, RulesError{
			Code:    ErrAceBridge,
			Message: "a run can't bridge from a low card (A-3) to a high card (J-K) around the corner",
		}
	}

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

		// Decide which cards fill the ace slots. An ace is natural only at
		// rank 1 or 14, and only in the run's own suit.
		aceLow := containsInt(positions, 1)
		aceHigh := containsInt(positions, 14)

		// If suit unknown (no fixed naturals), we must select a suit by requiring at least one natural ace to establish suit.
		// If the run has no fixed naturals, we require the aces used as natural endpoints to share a suit.
		if runSuit == "" {
			if !(aceLow || aceHigh) {
				continue tryStart
			}
			// Choose suit from the first ace; a joker can't name a suit.
			runSuit = CardSuit(aces[0])
		}

		// A joker is wild in the ace slots too. A suited ace claims one first,
		// since it is the real card for the slot, but when the cards hold no
		// such ace the slot is not special: leave it to the wild accounting
		// below, exactly like any other gap. Refusing the whole window
		// instead is what made Q♠-K♠-JOKER resolve as J♠-Q♠-K♠, the joker
		// standing in for the jack because the ace behind the king was the
		// one rank it was not allowed to be; and what made Q♠-K♠ plus two
		// jokers unlayable outright rather than read as J-Q-K-A.
		//
		// Either way the slot is worth the same, since a joker carries the
		// value of the card it replaces — see runRankValue, which prices a
		// slot by its rank and never asks what is sitting in it.
		spentAce := make([]bool, len(aces))
		takeSuitedAce := func() string {
			for i, a := range aces {
				if spentAce[i] || CardSuit(a) != runSuit {
					continue
				}
				spentAce[i] = true
				return a
			}
			return ""
		}

		// Assign specific ace cards as natural where one is available.
		aceAsNatural = map[string]int{}
		acesPlaced := 0
		if aceLow {
			if a := takeSuitedAce(); a != "" {
				aceAsNatural[a]++
				acesPlaced++
				naturalSlots[1] = true
			}
		}
		if aceHigh {
			if a := takeSuitedAce(); a != "" {
				aceAsNatural[a]++
				acesPlaced++
				naturalSlots[14] = true
			}
		}

		// Unlike jokers, an ace is a specific physical card with a real,
		// fixed suit — it can only stand in for rank 1 or rank 14 of its
		// own suit (handled above as aceAsNatural). An ace that isn't used
		// as a natural endpoint can't fill some other gap in the run
		// (that would let e.g. a wrong-suited ace pass as a wild filler),
		// so this window doesn't fit and we try the next one.
		//
		// Counted by physical card, not by distinct card string: two decks
		// can put two A♠ in one hand, and a run holding both ace ends spends
		// both, which a map keyed by "AS" cannot tell from spending one twice.
		if len(aces) > acesPlaced {
			continue tryStart
		}

		naturalCount := len(fixedRanks) + acesPlaced
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

		candidate := MeldValidation{
			Type:         MeldRun,
			AceAsNatural: aceAsNatural,
			NaturalValue: runValue(positions),
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
		//
		// Among windows that spend the same wilds, prefer the one worth more.
		// This is what puts a lone joker behind the king rather than in front
		// of the queen: Q♠-K♠ plus a joker fits at J-Q-K and at Q-K-A alike,
		// but the joker is worth the card it replaces, so the ace slot makes
		// it 35 against the jack slot's 30 — the difference between clearing
		// a 35-point floor and sitting just under it.
		//
		// A caller that named a drop position is asked first, though: the
		// player pointed at an end, and value is a guess at what they wanted
		// where position is a statement of it. With no position given the
		// lowest window still wins an exact tie, as it always has.
		switch {
		case best == nil:
			best = &candidate
		case candidate.WildCount != best.WildCount:
			if candidate.WildCount < best.WildCount {
				best = &candidate
			}
		case positionGiven:
			if preferHigh {
				best = &candidate
			}
		case candidate.NaturalValue > best.NaturalValue:
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

// hasAceBridge detects naturals that cannot belong to one consecutive run
// because reaching from the low one to the high one would have to wrap around
// the corner through the ace (e.g. K and 2 of the same suit).
//
// Holding both a low natural (A-3) and a high natural (J-K) is not by itself a
// bridge: a long enough run reaches from one to the other the honest way, up
// through the middle of the suit. 2-3-4-5-6-7-8-9-10-J is ten cards spanning
// ten ranks and perfectly legal, and a run that has grown that far keeps
// growing into Q and K. So the question is whether the naturals fit inside one
// window of runLen consecutive ranks; only when they demonstrably do not is
// wrapping the only way to explain them, and only then is this the player's
// real problem. fixedRanks must be sorted ascending.
func hasAceBridge(fixedRanks []int, runLen int) bool {
	if len(fixedRanks) == 0 {
		return false
	}
	span := fixedRanks[len(fixedRanks)-1] - fixedRanks[0] + 1
	if span <= runLen {
		return false
	}
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

// runGrowthSides reports which end(s) of an existing run a submission would
// extend: "front" (a lower rank than the run's current low), "end" (a higher
// rank than its current high), or both.
//
// known is false when the question cannot be answered — prevCards does not
// resolve as a run at all, or either side has no resolved rank window. The
// caller must then impose no end constraint, since it has nothing to
// constrain against.
//
// Shared by ValidateLayOff (which rejects a drop on the wrong end) and by
// LegalActions (which tells the client which ends are droppable) so the two
// can never give a player different answers.
func runGrowthSides(prevCards []string, mv MeldValidation) (sides []string, known bool) {
	// minRunSize is forced to 1: prevCards is an existing on-table run,
	// already validated at its real size when it was laid. This call wants
	// only its resolved rank range, not another length check that could
	// reject a run shorter than the current table minimum.
	oldMV, err := validateRun(prevCards, 1)
	if err != nil || len(oldMV.ResolvedRun) == 0 || len(mv.ResolvedRun) == 0 {
		return nil, false
	}
	oldMin, oldMax := oldMV.ResolvedRun[0], oldMV.ResolvedRun[len(oldMV.ResolvedRun)-1]
	newMin, newMax := mv.ResolvedRun[0], mv.ResolvedRun[len(mv.ResolvedRun)-1]
	growsFront := newMin < oldMin
	growsEnd := newMax > oldMax

	// A submission that would grow both ends at once matches neither
	// "front" nor "end", so naming either one is wrong — the caller gets an
	// empty (but known) list, meaning "legal, but do not send a position".
	switch {
	case growsFront && !growsEnd:
		return []string{"front"}, true
	case growsEnd && !growsFront:
		return []string{"end"}, true
	default:
		return nil, true
	}
}

func containsString(hay []string, needle string) bool {
	for _, s := range hay {
		if s == needle {
			return true
		}
	}
	return false
}
