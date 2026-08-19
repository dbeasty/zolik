package rules

// GamesWonByPlayer counts deals where the player had the uniquely lowest penalty score.
func GamesWonByPlayer(state GameState) map[string]int {
	wins := map[string]int{}
	for _, pid := range state.TurnOrder {
		wins[pid] = 0
	}
	maxGames := 0
	for _, pid := range state.TurnOrder {
		if n := len(state.GameScores[pid]); n > maxGames {
			maxGames = n
		}
	}
	for g := 0; g < maxGames; g++ {
		minScore := int(^uint(0) >> 1)
		for _, pid := range state.TurnOrder {
			scores := state.GameScores[pid]
			if g >= len(scores) {
				continue
			}
			if scores[g] < minScore {
				minScore = scores[g]
			}
		}
		winners := []string{}
		for _, pid := range state.TurnOrder {
			scores := state.GameScores[pid]
			if g >= len(scores) {
				continue
			}
			if scores[g] == minScore {
				winners = append(winners, pid)
			}
		}
		if len(winners) == 1 {
			wins[winners[0]]++
		}
	}
	return wins
}

// DetermineGameWinner picks the match winner after 7 deals: lowest total score;
// tiebreak: fewest deals won; still tied => draw.
func DetermineGameWinner(state GameState) (winnerID string, isDraw bool) {
	if len(state.TurnOrder) == 0 {
		return "", true
	}
	gamesWon := GamesWonByPlayer(state)

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

	// Tiebreak: fewest deals won (lower is better in continental rummy scoring).
	best := candidates[0]
	for _, pid := range candidates[1:] {
		if gamesWon[pid] < gamesWon[best] {
			best = pid
		}
	}
	stillTied := false
	for _, pid := range candidates {
		if pid != best && gamesWon[pid] == gamesWon[best] {
			stillTied = true
			break
		}
	}
	if stillTied {
		return "", true
	}
	return best, false
}

func lastGameScores(state GameState) map[string]int {
	out := map[string]int{}
	for _, pid := range state.TurnOrder {
		scores := state.GameScores[pid]
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
