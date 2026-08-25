// Package holdem implements No-Limit Texas Hold'em as a game module.
//
// It exists to falsify the module interface against a game that is not about
// matching cards at all — the fourth module, and the first whose central
// resource is chips rather than a hand. `docs/one-architecture-plan.md` §5
// records what it bent:
//
//	What poker needed              Why nothing before asked
//	--------------------------------------------------------------------
//	a number as an action input    every other input is a card or a choice
//	  ("raise to how much")        from a short list; a no-limit range
//	                               cannot be enumerated
//	more than one winner           a split pot has no single winner in any
//	                               sense; Canasta had already shown the seam
//	numbers attached to a seat     a stack, a bet in front of you, folded,
//	                               all-in — none of these are cards, so none
//	                               of them fit a Zone
//
// Everything else held. Zones described a board and two hole cards without
// changing; hidden-information filtering went into View exactly as it did for
// the card games; the descriptor expressed blinds and stack sizes with the
// option vocabulary that was already there.
//
// Rules implemented: one 52-card deck, two to nine seats, small and big blinds
// with a rotating button, four betting streets, no-limit raising with a real
// minimum-raise rule, all-ins and side pots, showdown by best five of seven,
// split pots, and elimination.
package holdem

import (
	"encoding/json"
	"fmt"

	"zolik/server/internal/module"
)

// Verbs this module accepts.
const (
	VerbFold  = "fold"
	VerbCheck = "check"
	VerbCall  = "call"
	VerbRaise = "raise"
)

// Streets, in order.
const (
	streetPreflop  = "preflop"
	streetFlop     = "flop"
	streetTurn     = "turn"
	streetRiver    = "river"
	streetShowdown = "showdown"
)

// ParamAmount is the raise size: the total this player's bet becomes, not the
// increment. Total rather than increment because that is what a table says out
// loud ("raise to sixty"), and because it leaves no ambiguity about whether the
// chips already in front of you count.
const ParamAmount = "amount"

// Error codes. Stable keys a client renders from its locale bundle.
const (
	ErrNotYourTurn     = "NOT_YOUR_TURN"
	ErrGameNotActive   = "GAME_NOT_ACTIVE"
	ErrUnknownAction   = "UNKNOWN_ACTION"
	ErrNothingToCall   = "NOTHING_TO_CALL"
	ErrCannotCheck     = "CANNOT_CHECK"
	ErrAmountRequired  = "AMOUNT_REQUIRED"
	ErrAmountNotNumber = "AMOUNT_NOT_A_NUMBER"
	ErrRaiseTooSmall   = "RAISE_TOO_SMALL"
	ErrNotEnoughChips  = "NOT_ENOUGH_CHIPS"
	ErrCannotRaise     = "CANNOT_RAISE"
	ErrSeatNotInHand   = "SEAT_NOT_IN_HAND"
)

// Seat is one player's position at the table, for the whole match.
type Seat struct {
	PlayerID string `json:"playerId"`
	// Stack is chips behind — what has not been pushed forward.
	Stack int `json:"stack"`
	// Bet is what this seat has in front of it on the current street, and
	// Committed is the same figure for the whole hand. Both are needed: the
	// street figure decides what it costs to call, and the hand figure builds
	// side pots.
	Bet       int `json:"bet"`
	Committed int `json:"committed"`

	Folded bool `json:"folded,omitempty"`
	AllIn  bool `json:"allIn,omitempty"`
	// Acted is whether this seat has had its turn on the current street. It is
	// what gives the big blind its option: posting a blind is not acting.
	Acted bool `json:"acted,omitempty"`
	// Out is eliminated from the match — no chips, no way back.
	Out  bool     `json:"out,omitempty"`
	Hole []string `json:"hole,omitempty"`
}

// inHand reports a seat still contesting the pot.
func (s *Seat) inHand() bool { return !s.Out && !s.Folded }

// canAct reports a seat that still has a decision to make: in the hand, with
// chips left to make it with.
func (s *Seat) canAct() bool { return s.inHand() && !s.AllIn }

// GameState is the whole match. Opaque to the runtime.
type GameState struct {
	Status    string `json:"status"` // "active" | "completed"
	Variation string `json:"variation,omitempty"`

	Seats  []Seat `json:"seats"`
	Button int    `json:"button"`
	// Current is the seat index to act, or -1 when nobody is being waited on.
	Current int    `json:"current"`
	Street  string `json:"street"`

	Deck  []string `json:"deck"`
	Board []string `json:"board,omitempty"`

	// Pot is chips collected from streets already closed. Chips on the current
	// street live on the seats until it closes, because that is where the
	// "what does it cost me to call" arithmetic needs them.
	Pot int `json:"pot"`
	// CurrentBet is the highest bet on this street, and MinRaise is the size a
	// raise must increase it by — the last raise's own size, which is the rule
	// most implementations get wrong by using the big blind forever.
	CurrentBet int `json:"currentBet"`
	MinRaise   int `json:"minRaise"`

	SmallBlind    int `json:"smallBlind"`
	BigBlind      int `json:"bigBlind"`
	StartingStack int `json:"startingStack"`
	// HandLimit is how many hands the match runs for; zero means play until
	// one player holds every chip.
	HandLimit  int `json:"handLimit"`
	HandNumber int `json:"handNumber"`

	LastHand *HandResult `json:"lastHand,omitempty"`
	Winners  []string    `json:"winners,omitempty"`
	Seed     int64       `json:"seed"`
}

// HandResult is how one hand finished, kept so a client can show a showdown
// without recomputing anything.
type HandResult struct {
	HandNumber int `json:"handNumber"`
	// Uncontested is a hand everyone else folded out of: there was no
	// showdown, and no hand was ever shown.
	Uncontested bool           `json:"uncontested,omitempty"`
	Board       []string       `json:"board,omitempty"`
	Pots        []PotResult    `json:"pots,omitempty"`
	Shown       []ShownHand    `json:"shown,omitempty"`
	Deltas      map[string]int `json:"deltas,omitempty"`
}

// PotResult is one pot — the main one, or a side pot — and who took it.
type PotResult struct {
	Amount  int      `json:"amount"`
	Winners []string `json:"winners"`
	// LabelKey names the winning hand ("holdem.hand.flush"), or is empty when
	// the pot was won without a showdown.
	LabelKey string `json:"labelKey,omitempty"`
}

// ShownHand is one player's cards at showdown, with what they made.
type ShownHand struct {
	PlayerID string   `json:"playerId"`
	Hole     []string `json:"hole"`
	Best     []string `json:"best"`
	LabelKey string   `json:"labelKey"`
}

func (s *GameState) seat(playerID string) *Seat {
	for i := range s.Seats {
		if s.Seats[i].PlayerID == playerID {
			return &s.Seats[i]
		}
	}
	return nil
}

func (s *GameState) seatIndex(playerID string) int {
	for i := range s.Seats {
		if s.Seats[i].PlayerID == playerID {
			return i
		}
	}
	return -1
}

// liveSeats are the indices still in the match — they have chips, or are in a
// hand holding some.
func (s *GameState) liveSeats() []int {
	var out []int
	for i := range s.Seats {
		if !s.Seats[i].Out {
			out = append(out, i)
		}
	}
	return out
}

// contenders are the indices still contesting the current pot.
func (s *GameState) contenders() []int {
	var out []int
	for i := range s.Seats {
		if s.Seats[i].inHand() {
			out = append(out, i)
		}
	}
	return out
}

// nextSeat walks clockwise from an index, returning the first that satisfies
// want. Returns -1 if nobody does.
func (s *GameState) nextSeat(from int, want func(*Seat) bool) int {
	n := len(s.Seats)
	for i := 1; i <= n; i++ {
		idx := (from + i) % n
		if want(&s.Seats[idx]) {
			return idx
		}
	}
	return -1
}

// toCall is what this seat must put in to match the current bet — capped at
// its stack, because you can always call all-in for less.
func (s *GameState) toCall(seat *Seat) int {
	owed := s.CurrentBet - seat.Bet
	if owed < 0 {
		owed = 0
	}
	if owed > seat.Stack {
		owed = seat.Stack
	}
	return owed
}

func decode(raw module.State) (*GameState, error) {
	var s GameState
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("holdem: decode state: %w", err)
	}
	return &s, nil
}

func encode(s *GameState) (module.State, error) {
	raw, err := json.Marshal(s)
	if err != nil {
		return nil, fmt.Errorf("holdem: encode state: %w", err)
	}
	return raw, nil
}

func errCode(code string) error { return module.Error{Code: code} }

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
