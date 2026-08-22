// Package prsi implements Prší (Czech Mau-Mau) as a game module.
//
// It exists to falsify the abstraction, not to be a second rummy. Prší shares
// almost no vocabulary with Žolíky:
//
//	Žolíky                        Prší
//	------------------------------------------------------------
//	draw → meld → discard phases  one move per turn, no phases
//	melds, lay-offs, contracts    no melds at all
//	108 cards (2 decks + jokers)  32 cards, no jokers
//	go out by melding + discard   go out by shedding your last card
//	scoring by penalty points     first to empty hand wins the deal
//	                              cards with *effects*: draw-two, skip, wild
//
// If the module interface fits this too, it is a game-agnostic contract. If it
// had to bend, the bend is the finding — and it did bend once, in a way worth
// recording: an offer needed a parameter that is not a card, because playing a
// Queen changes the suit in play and the player must say which. See
// module.ParamSpec.
//
// Rules implemented (the common Czech pub ruleset):
//   - 32 cards: 7,8,9,10,J,Q,K,A in four suits.
//   - Deal 4 each, turn one card face up to start the pile.
//   - On your turn, play a card matching the pile's suit or rank, or draw one.
//   - 7 makes the next player draw two, and stacks: two 7s make it four.
//   - A skips the next player.
//   - Q is wild: play it on anything and name the suit that follows.
//   - Empty your hand to win.
package prsi

import (
	"encoding/json"
	"fmt"

	"zolik/server/internal/module"
)

// Ranks and suits of the 32-card Czech deck, in the notation the rest of the
// server already uses for cards ("7H", "QS", "AD").
var (
	ranks = []string{"7", "8", "9", "T", "J", "Q", "K", "A"}
	suits = []string{"H", "D", "C", "S"}
)

// Card effects. Named rather than checked inline so the rules read as rules.
const (
	rankDrawTwo = "7" // next player draws two, and it stacks
	rankSkip    = "A" // next player loses their turn
	rankWild    = "Q" // playable on anything; names the suit that follows
)

// GameState is the whole match. Opaque to the runtime — only this package
// reads it.
type GameState struct {
	Status    string   `json:"status"` // "active" | "completed"
	Players   []string `json:"players"`
	TurnOrder []string `json:"turnOrder"`
	Current   string   `json:"current"`

	DrawPile    []string            `json:"drawPile"`
	DiscardPile []string            `json:"discardPile"` // top = last
	Hands       map[string][]string `json:"hands"`

	// DeclaredSuit is set while a wild (Q) is the top card: the suit its
	// player named, which is what the next card must match. Empty otherwise.
	DeclaredSuit string `json:"declaredSuit,omitempty"`

	// PendingDraw is how many cards the player to move must take because of
	// unanswered 7s. They may answer with a 7 of their own instead, which
	// adds two more and passes the obligation on.
	PendingDraw int `json:"pendingDraw,omitempty"`

	// SkipPending is set when an Ace was played: the player to move loses
	// their turn unless they answer with an Ace of their own.
	SkipPending bool `json:"skipPending,omitempty"`

	WinnerID string `json:"winnerId,omitempty"`
	Seed     int64  `json:"seed"`
	// Reshuffles counts how many times the pile has been recycled, which also
	// varies the reshuffle seed so the same cards do not come back in the
	// same order.
	Reshuffles int `json:"reshuffles"`
}

// Error codes. Stable keys, rendered by the client's locale bundle — the same
// contract Žolíky's RulesErrorCode has.
const (
	ErrNotYourTurn    = "NOT_YOUR_TURN"
	ErrGameNotActive  = "GAME_NOT_ACTIVE"
	ErrCardNotInHand  = "CARD_NOT_IN_HAND"
	ErrCardDoesNotFit = "CARD_DOES_NOT_FIT"
	ErrMustAnswerDraw = "MUST_ANSWER_DRAW_OR_TAKE"
	ErrSuitRequired   = "SUIT_REQUIRED"
	ErrUnknownSuit    = "UNKNOWN_SUIT"
	ErrNothingToDraw  = "NOTHING_TO_DRAW"
	ErrUnknownAction  = "UNKNOWN_ACTION"
)

// Verbs this module accepts.
const (
	VerbPlay = "play_card"
	VerbDraw = "draw"
	VerbPass = "pass" // take the skip an Ace imposed
)

func rankOf(card string) string {
	if card == "" {
		return ""
	}
	return card[:len(card)-1]
}

func suitOf(card string) string {
	if card == "" {
		return ""
	}
	return card[len(card)-1:]
}

func (s *GameState) top() string {
	if len(s.DiscardPile) == 0 {
		return ""
	}
	return s.DiscardPile[len(s.DiscardPile)-1]
}

// suitInPlay is the suit a card must match: the suit named by a wild if one
// is in force, otherwise the top card's own suit.
func (s *GameState) suitInPlay() string {
	if s.DeclaredSuit != "" {
		return s.DeclaredSuit
	}
	return suitOf(s.top())
}

// playable reports whether card may be played on the current pile.
//
// The obligations come first and override ordinary matching, which is the
// whole character of the game: while a 7 is unanswered only another 7 will
// do, and while an Ace is unanswered only another Ace will.
func (s *GameState) playable(card string) bool {
	if s.PendingDraw > 0 {
		return rankOf(card) == rankDrawTwo
	}
	if s.SkipPending {
		return rankOf(card) == rankSkip
	}
	if rankOf(card) == rankWild {
		return true // a wild goes on anything
	}
	top := s.top()
	// A wild on top is answered by its declared suit, never by matching the
	// Queen's own rank — otherwise a Queen could always be answered by
	// another Queen regardless of the suit just named, and the naming would
	// mean nothing.
	if rankOf(top) == rankWild {
		return suitOf(card) == s.suitInPlay()
	}
	return suitOf(card) == s.suitInPlay() || rankOf(card) == rankOf(top)
}

func (s *GameState) nextPlayer(from string) string {
	for i, p := range s.TurnOrder {
		if p == from {
			return s.TurnOrder[(i+1)%len(s.TurnOrder)]
		}
	}
	if len(s.TurnOrder) > 0 {
		return s.TurnOrder[0]
	}
	return ""
}

func decode(raw module.State) (*GameState, error) {
	var s GameState
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("prsi: decode state: %w", err)
	}
	return &s, nil
}

func encode(s *GameState) (module.State, error) {
	raw, err := json.Marshal(s)
	if err != nil {
		return nil, fmt.Errorf("prsi: encode state: %w", err)
	}
	return raw, nil
}

func removeCard(hand []string, card string) []string {
	out := make([]string, 0, len(hand))
	dropped := false
	for _, c := range hand {
		if !dropped && c == card {
			dropped = true
			continue
		}
		out = append(out, c)
	}
	return out
}

func hasCard(hand []string, card string) bool {
	for _, c := range hand {
		if c == card {
			return true
		}
	}
	return false
}
