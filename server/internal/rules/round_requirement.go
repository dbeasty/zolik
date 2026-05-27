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

// HandPenaltyTotal scores all cards remaining in a player's hand at round end.
// Aces in hand count as wild (25) unless the caller supplies ace-as-natural hints later.
func HandPenaltyTotal(hand []string) int {
	sum := 0
	for _, c := range hand {
		sum += PenaltyPoints(c, false)
	}
	return sum
}
