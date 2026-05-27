package rules

// RoundsWonByPlayer counts rounds where the player had the uniquely lowest penalty score.
func RoundsWonByPlayer(state GameState) map[string]int {
	wins := map[string]int{}
	for _, pid := range state.TurnOrder {
		wins[pid] = 0
	}
	maxRounds := 0
	for _, pid := range state.TurnOrder {
		if n := len(state.RoundScores[pid]); n > maxRounds {
			maxRounds = n
		}
	}
	for r := 0; r < maxRounds; r++ {
		minScore := int(^uint(0) >> 1)
		for _, pid := range state.TurnOrder {
			scores := state.RoundScores[pid]
			if r >= len(scores) {
				continue
			}
			if scores[r] < minScore {
				minScore = scores[r]
			}
		}
		winners := []string{}
		for _, pid := range state.TurnOrder {
			scores := state.RoundScores[pid]
			if r >= len(scores) {
				continue
			}
			if scores[r] == minScore {
				winners = append(winners, pid)
			}
		}
		if len(winners) == 1 {
			wins[winners[0]]++
		}
	}
	return wins
}

// DetermineGameWinner picks the winner after 7 rounds: lowest total score;
// tiebreak: fewest rounds won; still tied => draw.
func DetermineGameWinner(state GameState) (winnerID string, isDraw bool) {
	if len(state.TurnOrder) == 0 {
		return "", true
	}
	roundsWon := RoundsWonByPlayer(state)

	minTotal := int(^uint(0) >> 1)
	for _, pid := range state.TurnOrder {
		if state.TotalScores[pid] < minTotal {
			minTotal = state.TotalScores[pid]
		}
	}
	candidates := []string{}
	for _, pid := range state.TurnOrder {
		if state.TotalScores[pid] == minTotal {
			candidates = append(candidates, pid)
		}
	}
	if len(candidates) == 1 {
		return candidates[0], false
	}

	// Tiebreak: fewest rounds won (lower is better in continental rummy scoring).
	best := candidates[0]
	for _, pid := range candidates[1:] {
		if roundsWon[pid] < roundsWon[best] {
			best = pid
		}
	}
	stillTied := false
	for _, pid := range candidates {
		if pid != best && roundsWon[pid] == roundsWon[best] {
			stillTied = true
			break
		}
	}
	if stillTied {
		return "", true
	}
	return best, false
}

func lastRoundScores(state GameState) map[string]int {
	out := map[string]int{}
	for _, pid := range state.TurnOrder {
		scores := state.RoundScores[pid]
		if len(scores) == 0 {
			out[pid] = 0
			continue
		}
		out[pid] = scores[len(scores)-1]
	}
	return out
}

func allHandsForLog(state GameState) map[string][]string {
	out := map[string][]string{}
	for pid, hand := range state.Hands {
		out[pid] = append([]string(nil), hand...)
	}
	return out
}
