package rules

// RoundsWonByPlayer counts rounds where the player had the uniquely lowest penalty score.
func RoundsWonByPlayer(state GameState) map[string]int {
	return DealsWonByPlayer(state.TurnOrder, state.GameScores)
}

// DealsWonByPlayer is the raw form of RoundsWonByPlayer: it takes only the two
// fields the count actually depends on, so callers outside the engine — match
// recording, live scoreboards — can compute the same number without
// assembling a whole GameState. A deal is won only by the player whose penalty
// was *uniquely* lowest; a tie for the lowest score awards it to nobody.
func DealsWonByPlayer(turnOrder []string, gameScores map[string][]int) map[string]int {
	wins := map[string]int{}
	for _, pid := range turnOrder {
		wins[pid] = 0
	}
	maxRounds := 0
	for _, pid := range turnOrder {
		if n := len(gameScores[pid]); n > maxRounds {
			maxRounds = n
		}
	}
	for r := 0; r < maxRounds; r++ {
		minScore := int(^uint(0) >> 1)
		for _, pid := range turnOrder {
			scores := gameScores[pid]
			if r >= len(scores) {
				continue
			}
			if scores[r] < minScore {
				minScore = scores[r]
			}
		}
		winners := []string{}
		for _, pid := range turnOrder {
			scores := gameScores[pid]
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
	return DetermineMatchWinner(state.TurnOrder, state.TotalScores, state.GameScores)
}

// DetermineMatchWinner is the raw form of DetermineGameWinner, taking the three
// fields the decision depends on. Kept separate so a scoreboard can show who is
// *currently* ahead in an unfinished match using the very rule that will decide
// it, rather than a lookalike reimplementation that drifts.
func DetermineMatchWinner(turnOrder []string, totalScores map[string]int, gameScores map[string][]int) (winnerID string, isDraw bool) {
	if len(turnOrder) == 0 {
		return "", true
	}
	roundsWon := DealsWonByPlayer(turnOrder, gameScores)

	minTotal := int(^uint(0) >> 1)
	for _, pid := range turnOrder {
		if totalScores[pid] < minTotal {
			minTotal = totalScores[pid]
		}
	}
	candidates := []string{}
	for _, pid := range turnOrder {
		if totalScores[pid] == minTotal {
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
