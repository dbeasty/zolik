// Package ginrummy implements Gin Rummy as a game module.
//
// It is the fifth game and the second measurement of one-architecture-plan.md's
// closing claim — one way to add a game — asked of a ruleset nobody had in mind
// when the module interface was built. Unlike Žolíky and Canasta, there is no
// melding during play: melds exist only in the arithmetic of a knock, computed
// by the engine rather than declared by a player, which is what lets
// `knock:<card>` and `gin:<card>` ship as concrete, enumerated offers instead
// of the composite shape Žolíky's lay-down needs. See meld.go's package doc for
// why that computation is cheap enough to not matter.
//
// Rules implemented (Hoyle / the pagat.com ruleset):
//   - 52 cards, two players, ten dealt each, one card turned as the first
//     discard.
//   - Non-dealer may take the upcard; if they decline, the dealer may; if both
//     decline, non-dealer draws from the stock and normal play begins.
//   - Draw one, then discard one — exactly two actions, always in that order.
//   - Sets of 3–4 of a rank, runs of 3+ in a suit, ace low, no wrap.
//   - After drawing, a deadwood of at most the knock limit may be knocked;
//     zero deadwood is gin. After a knock that is not gin, the defender may
//     lay their own deadwood onto the knocker's melds before the hands are
//     compared.
//   - A stock of two cards faced without a knock is a dead hand: nobody
//     scores, and the same dealer redeals.
//   - First to the target score after a hand ends wins the match.
package ginrummy

import (
	"encoding/json"
	"fmt"

	"zolik/server/internal/module"
)

// Phases a hand walks through, in order, with layoff a bounded branch off
// discard rather than a fifth step every hand takes.
const (
	phaseUpcardNonDealer = "upcard_nondealer"
	phaseUpcardDealer    = "upcard_dealer"
	phaseDraw            = "draw"
	phaseDiscard         = "discard"
	phaseLayoff          = "layoff"
)

// Verbs this module accepts. VerbContinue belongs to module.Intermission.
const (
	VerbDraw         = "draw"
	VerbPass         = "pass"
	VerbDiscard      = "discard"
	VerbKnock        = "knock"
	VerbLayOff       = "lay_off"
	VerbFinishLayoff = "finish_layoff"
)

// Error codes. Stable keys, rendered by the client's locale bundle.
const (
	ErrWrongPlayerCount      = "WRONG_PLAYER_COUNT"
	ErrGameNotActive         = "GAME_NOT_ACTIVE"
	ErrNotYourTurn           = "NOT_YOUR_TURN"
	ErrUnknownAction         = "UNKNOWN_ACTION"
	ErrWrongPhase            = "WRONG_PHASE"
	ErrCardNotInHand         = "CARD_NOT_IN_HAND"
	ErrCardDoesNotFit        = "CARD_DOES_NOT_FIT"
	ErrNothingToDraw         = "NOTHING_TO_DRAW"
	ErrUpcardDeclined        = "UPCARD_DECLINED"
	ErrDeadwoodTooHigh       = "DEADWOOD_TOO_HIGH"
	ErrNoSuchMeld            = "NO_SUCH_MELD"
	ErrCardDoesNotExtendMeld = "CARD_DOES_NOT_EXTEND_MELD"
)

// Meld is one of the knocker's melds, revealed on the board the moment a
// knock happens. Extended in place as the defender lays cards onto it.
type Meld struct {
	ID    string   `json:"id"`
	Kind  string   `json:"kind"` // "set" | "run"
	Cards []string `json:"cards"`
}

// HandResult is one hand's settlement, kept for Rounds().
type HandResult struct {
	Number           int            `json:"number"`
	Kind             string         `json:"kind"` // "gin" | "knock" | "undercut" | "dead"
	Winner           string         `json:"winner,omitempty"`
	HandDelta        int            `json:"handDelta,omitempty"`
	KnockerDeadwood  int            `json:"knockerDeadwood,omitempty"`
	DefenderDeadwood int            `json:"defenderDeadwood,omitempty"`
	Deltas           map[string]int `json:"deltas,omitempty"`
	Totals           map[string]int `json:"totals,omitempty"`
}

// GameState is the whole match. Opaque to the runtime — only this package
// reads it.
type GameState struct {
	Status    string `json:"status"` // "active" | "completed"
	Variation string `json:"variation"`

	Players []string `json:"players"` // exactly two, stable seat order
	Dealer  string   `json:"dealer"`
	Current string   `json:"current"`
	Phase   string   `json:"phase"`

	// ForcedStockDraw is true for exactly one draw: the mandatory stock draw
	// after both players decline the upcard, where drawing the discard again
	// would just be taking the very card both of them just refused.
	ForcedStockDraw bool `json:"forcedStockDraw,omitempty"`

	Hands       map[string][]string `json:"hands"`
	Stock       []string            `json:"stock"`
	DiscardPile []string            `json:"discardPile"` // last = top

	// KnockDiscard is the card a knock or gin was declared with. Kept out of
	// DiscardPile — laid face down, per the rules — since the hand ends before
	// another draw would ever need to see it.
	KnockDiscard string `json:"knockDiscard,omitempty"`

	HandNumber int `json:"handNumber"` // 0-based

	Knocker         string `json:"knocker,omitempty"`
	KnockGin        bool   `json:"knockGin,omitempty"`
	KnockerDeadwood int    `json:"knockerDeadwood,omitempty"`
	KnockerMelds    []Meld `json:"knockerMelds,omitempty"`

	// Interest is, per player, every card they have taken from the discard
	// pile this hand — the module's own discard log, per §2.6 of the plan
	// this package implements. A bot reads the opponent's list to discard
	// away from a rank or suit run they have shown interest in, and it is
	// reset every new hand since a stale interest from a hand already scored
	// tells a bot nothing about the hand in front of it.
	Interest map[string][]string `json:"interest"`

	Scores   map[string]int `json:"scores"`
	HandsWon map[string]int `json:"handsWon"`

	Intermission module.Intermission `json:"intermission,omitempty"`
	Pause        bool                `json:"pause,omitempty"`

	TargetScore   int  `json:"targetScore"`
	KnockLimitOpt int  `json:"knockLimitOpt"` // the raw option: a fixed limit, or oklahomaSentinel
	KnockLimit    int  `json:"knockLimit"`    // this hand's effective limit
	BigGin        bool `json:"bigGin,omitempty"`
	LineBonuses   bool `json:"lineBonuses,omitempty"`

	Seed int64 `json:"seed"`

	WinnerID string       `json:"winnerId,omitempty"`
	Rounds   []HandResult `json:"rounds,omitempty"`
}

func decode(raw module.State) (*GameState, error) {
	var s GameState
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("ginrummy: decode state: %w", err)
	}
	return &s, nil
}

func encode(s *GameState) (module.State, error) {
	raw, err := json.Marshal(s)
	if err != nil {
		return nil, fmt.Errorf("ginrummy: encode state: %w", err)
	}
	return raw, nil
}

func errCode(code string) error { return module.Error{Code: code} }

func other(players []string, id string) string {
	for _, p := range players {
		if p != id {
			return p
		}
	}
	return ""
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
