package ferbl

import (
	"encoding/json"
	"fmt"

	"zolik/server/internal/module"
)

// Seat is one player.
type Seat struct {
	PlayerID string   `json:"playerId"`
	Stack    int      `json:"stack"`
	Hand     []string `json:"hand,omitempty"`
	// In is whether the seat was dealt into this hand; Folded is whether it
	// has since given it up.
	In     bool `json:"in,omitempty"`
	Folded bool `json:"folded,omitempty"`
	// Bet is what the seat has put in this betting round; Total is what it
	// has put in this hand, ante included.
	Bet   int  `json:"bet,omitempty"`
	Total int  `json:"total,omitempty"`
	Acted bool `json:"acted,omitempty"`
	// Raised counts the bets and raises this seat has made this hand: public,
	// and what a reading bot weighs its hand by.
	Raised int `json:"raised,omitempty"`
	// Shown is whether the hand was turned up at the showdown.
	Shown bool `json:"shown,omitempty"`
	Delta int  `json:"delta,omitempty"`
}

// GameState is the whole match. Opaque to the runtime.
type GameState struct {
	Status string `json:"status"`
	Seats  []Seat `json:"seats"`
	Seed   int64  `json:"seed"`

	StartingStack int  `json:"startingStack"`
	Vizi          int  `json:"vizi"`
	Hands         int  `json:"hands"` // the match is this many hands
	Pause         bool `json:"pause,omitempty"`

	Hand        int `json:"hand"` // counted from zero
	FirstDealer int `json:"firstDealer"`

	Phase      string   `json:"phase"` // "first" (two cards) or "second" (four)
	Current    string   `json:"current"`
	Deck       []string `json:"deck,omitempty"`
	CurrentBet int      `json:"currentBet,omitempty"`
	Raises     int      `json:"raises,omitempty"`

	Rounds       []HandRecord        `json:"rounds,omitempty"`
	Intermission module.Intermission `json:"intermission,omitempty"`
}

// HandRecord is one settled hand. Public: what was shown, never a folded
// hand.
type HandRecord struct {
	Number  int                 `json:"number"`
	Winners []string            `json:"winners"`
	Pot     int                 `json:"pot"`
	Shown   map[string][]string `json:"shown,omitempty"`
	Scores  map[string]int      `json:"scores,omitempty"`
	Deltas  map[string]int      `json:"deltas"`
	Stacks  map[string]int      `json:"stacks"`
}

// Phases: a betting round on two cards, then one on four.
const (
	phaseFirst  = "first"
	phaseSecond = "second"
)

// Verbs this module accepts. VerbContinue belongs to module.Intermission.
const (
	VerbCheck = "check"
	VerbBet   = "bet"
	VerbCall  = "call"
	VerbRaise = "raise"
	VerbFold  = "fold"
)

// Error codes. Stable keys, worded by the client's locale bundles.
const (
	ErrNotYourTurn   = "NOT_YOUR_TURN"
	ErrGameNotActive = "GAME_NOT_ACTIVE"
	ErrUnknownAction = "UNKNOWN_ACTION"

	ErrCannotCheck   = "FERBL_CANNOT_CHECK"
	ErrCannotBet     = "FERBL_CANNOT_BET"
	ErrNothingToCall = "FERBL_NOTHING_TO_CALL"
	ErrRaiseCap      = "FERBL_RAISES_CLOSED"
	ErrCannotRaise   = "FERBL_CANNOT_RAISE"
	ErrNoNeedToFold  = "FERBL_NO_NEED_TO_FOLD"
)

func decode(raw module.State) (*GameState, error) {
	var s GameState
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("ferbl: decode state: %w", err)
	}
	return &s, nil
}

func encode(s *GameState) (module.State, error) {
	raw, err := json.Marshal(s)
	if err != nil {
		return nil, fmt.Errorf("ferbl: encode state: %w", err)
	}
	return raw, nil
}

func errCode(code string) error { return module.Error{Code: code} }

func (s *GameState) seat(id string) *Seat {
	for i := range s.Seats {
		if s.Seats[i].PlayerID == id {
			return &s.Seats[i]
		}
	}
	return nil
}

func (s *GameState) index(id string) int {
	for i := range s.Seats {
		if s.Seats[i].PlayerID == id {
			return i
		}
	}
	return -1
}

func (s *GameState) players() []string {
	out := make([]string, len(s.Seats))
	for i, st := range s.Seats {
		out[i] = st.PlayerID
	}
	return out
}

func (s *GameState) dealerIndex() int { return (s.FirstDealer + s.Hand) % len(s.Seats) }

// order is the seats in turn order: from the dealer's left round to the
// dealer.
func (s *GameState) order() []int {
	n := len(s.Seats)
	out := make([]int, n)
	for k := 0; k < n; k++ {
		out[k] = (s.dealerIndex() + 1 + k) % n
	}
	return out
}

// live is a seat still contesting the hand.
func (st *Seat) live() bool { return st.In && !st.Folded }

// canAct is a live seat with chips left to bet.
func (st *Seat) canAct() bool { return st.live() && st.Stack > 0 }

// unit is the bet in this round: one vizi on two cards, two on four.
func (s *GameState) unit() int {
	if s.Phase == phaseSecond {
		return 2 * s.Vizi
	}
	return s.Vizi
}

// pot is every chip put in this hand.
func (s *GameState) pot() int {
	n := 0
	for _, st := range s.Seats {
		n += st.Total
	}
	return n
}

// toCall is what this seat must add to stay in.
func (s *GameState) toCall(st *Seat) int { return max(0, s.CurrentBet-st.Bet) }
