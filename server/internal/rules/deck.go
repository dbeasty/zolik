package rules

import (
	"fmt"
	"math/rand"
	"time"
)

func DeckCountForPlayers(players int) int {
	switch players {
	case 2, 3, 4:
		return 2
	case 5, 6:
		return 3
	case 7, 8:
		return 4
	default:
		// default to 2 decks for non-standard sizes; callers should validate separately.
		return 2
	}
}

func BuildDeck(players int) []string {
	decks := DeckCountForPlayers(players)
	ranks := []string{"2", "3", "4", "5", "6", "7", "8", "9", "T", "J", "Q", "K", "A"}
	suits := []string{"H", "C", "D", "S"}

	var out []string
	out = make([]string, 0, decks*54)
	for d := 0; d < decks; d++ {
		for _, r := range ranks {
			for _, s := range suits {
				out = append(out, r+s)
			}
		}
		// Jokers are not uniquely identifiable in the spec beyond JOKER1/JOKER2.
		// With multiple decks, duplicates are expected and handled as multi-set values.
		out = append(out, "JOKER1", "JOKER2")
	}
	return out
}

func Shuffle(cards []string, seed int64) []string {
	out := append([]string(nil), cards...)
	r := rand.New(rand.NewSource(seed))
	r.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
	return out
}

func NewShuffleSeed() int64 {
	return time.Now().UnixNano()
}

// DealHand deals cfg.DealSize cards to each player (0/unset falls back to 12,
// Continental's historical deal size).
func DealHand(state GameState, cfg RulesConfig) (GameState, error) {
	if len(state.TurnOrder) == 0 {
		return state, fmt.Errorf("no turn order")
	}
	dealSize := cfg.DealSize
	if dealSize == 0 {
		dealSize = 12
	}
	if state.Hands == nil {
		state.Hands = map[string][]string{}
	}
	for _, pid := range state.TurnOrder {
		state.Hands[pid] = nil
	}
	for i := 0; i < dealSize; i++ {
		for _, pid := range state.TurnOrder {
			if len(state.DrawPile) == 0 {
				return state, RulesError{Code: ErrNoCardsLeft, Message: "draw pile exhausted during deal"}
			}
			card := state.DrawPile[len(state.DrawPile)-1]
			state.DrawPile = state.DrawPile[:len(state.DrawPile)-1]
			state.Hands[pid] = append(state.Hands[pid], card)
		}
	}
	return state, nil
}
