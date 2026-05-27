package rules

// RoundRequirement describes how many qualifying sets (3+) and runs (4+) a player
// must have on the table before they can go out in a given round.
type RoundRequirement struct {
	Sets int
	Runs int
}

func RoundRequirementFor(round int) RoundRequirement {
	switch round {
	case 1:
		return RoundRequirement{Sets: 2, Runs: 0}
	case 2:
		return RoundRequirement{Sets: 1, Runs: 1}
	case 3:
		return RoundRequirement{Sets: 0, Runs: 2}
	case 4:
		return RoundRequirement{Sets: 3, Runs: 0}
	case 5:
		return RoundRequirement{Sets: 2, Runs: 1}
	case 6:
		return RoundRequirement{Sets: 1, Runs: 2}
	case 7:
		return RoundRequirement{Sets: 0, Runs: 3}
	default:
		return RoundRequirement{}
	}
}

// PlayerMeldCounts returns qualifying sets and runs the player has laid this round.
func PlayerMeldCounts(state GameState, playerID string) (sets, runs int) {
	metas := state.MeldMeta[playerID]
	melds := state.Melds[playerID]
	for i, mi := range metas {
		if i >= len(melds) {
			break
		}
		switch mi.Type {
		case MeldSet:
			if len(melds[i]) >= 3 {
				sets++
			}
		case MeldRun:
			if len(melds[i]) >= 4 {
				runs++
			}
		}
	}
	return sets, runs
}

// PlayerMeetsRoundRequirement reports whether the player has the required melds on table.
func PlayerMeetsRoundRequirement(state GameState, playerID string) bool {
	req := RoundRequirementFor(state.Round)
	sets, runs := PlayerMeldCounts(state, playerID)
	return sets >= req.Sets && runs >= req.Runs
}

// PlayerInitialMeldNaturalValue sums natural card values across all melds the player laid this round.
func PlayerInitialMeldNaturalValue(state GameState, playerID string) int {
	total := 0
	melds := state.Melds[playerID]
	for _, cards := range melds {
		mv, err := ValidateMeld(cards)
		if err != nil {
			continue
		}
		total += mv.NaturalValue
	}
	return total
}

// MeldContributesTowardRequirement reports whether a new qualifying meld moves the
// player toward the current round's pattern before roundReqMet is set.
func MeldContributesTowardRequirement(state GameState, playerID string, meldType MeldType, cardCount int) bool {
	if state.RoundReqMet[playerID] {
		return true
	}
	req := RoundRequirementFor(state.Round)
	setsBefore, runsBefore := PlayerMeldCounts(state, playerID)

	addsSet := meldType == MeldSet && cardCount >= 3
	addsRun := meldType == MeldRun && cardCount >= 4
	if !addsSet && !addsRun {
		return false
	}
	if addsSet && setsBefore < req.Sets {
		return true
	}
	if addsRun && runsBefore < req.Runs {
		return true
	}
	return false
}

// AllTableMelds returns every meld currently on the table (all players).
func AllTableMelds(state GameState) [][]string {
	var out [][]string
	for _, melds := range state.Melds {
		for _, m := range melds {
			out = append(out, append([]string(nil), m...))
		}
	}
	return out
}

// HandPenaltyTotal scores leftover cards at round end.
// Aces count as 1 when they sit in a natural run fragment in hand or can extend a table run.
func HandPenaltyTotal(hand []string) int {
	return HandPenaltyTotalWithMelds(hand, nil)
}

func HandPenaltyTotalWithMelds(hand []string, tableMelds [][]string) int {
	sum := 0
	for _, c := range hand {
		sum += handCardPenalty(c, hand, tableMelds)
	}
	return sum
}

func handCardPenalty(card string, hand []string, tableMelds [][]string) int {
	if IsAce(card) {
		if aceCountsAsNaturalInHand(card, hand) {
			return 1
		}
		for _, meld := range tableMelds {
			if aceExtendsRunAsNatural(card, meld) {
				return 1
			}
		}
		return 25
	}
	return PenaltyPoints(card, false)
}

func aceCountsAsNaturalInHand(ace string, hand []string) bool {
	suit := CardSuit(ace)
	has2, hasQ, hasK := false, false, false
	for _, c := range hand {
		if c == ace || IsJoker(c) {
			continue
		}
		if IsAce(c) {
			continue
		}
		if CardSuit(c) != suit {
			continue
		}
		switch CardRank(c) {
		case 0:
			has2 = true
		case 10:
			hasQ = true
		case 11:
			hasK = true
		}
	}
	if has2 {
		return true
	}
	return hasQ && hasK
}

func minMaxRunRank(cards []string) (min, max int) {
	min, max = 99, 0
	for _, c := range cards {
		if IsJoker(c) || IsAce(c) {
			continue
		}
		r := cardToRunRank(c)
		if r < 2 {
			continue
		}
		if r < min {
			min = r
		}
		if r > max {
			max = r
		}
	}
	return min, max
}

func aceExtendsRunAsNatural(ace string, meld []string) bool {
	if len(meld) < 4 || !IsAce(ace) {
		return false
	}
	if _, err := ValidateMeld(meld); err != nil {
		return false
	}
	suit := CardSuit(ace)
	minR, maxR := minMaxRunRank(meld)

	try := func(extended []string, atEnd bool) bool {
		if len(extended) != len(meld)+1 {
			return false
		}
		mv, err := ValidateMeld(extended)
		if err != nil || mv.Type != MeldRun {
			return false
		}
		if mv.ResolvedSuit != "" && mv.ResolvedSuit != suit {
			return false
		}
		if atEnd {
			return maxR == 13 && extended[len(extended)-1] == ace
		}
		return minR == 2 && extended[0] == ace
	}

	appended := append(append([]string(nil), meld...), ace)
	if try(appended, true) {
		return true
	}
	prepended := append([]string{ace}, meld...)
	return try(prepended, false)
}
